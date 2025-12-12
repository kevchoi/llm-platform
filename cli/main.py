import dis
import time
import subprocess
import ScreenCaptureKit as sck
from threading import Event
import json
from AppKit import NSRunLoop, NSDate
from PIL import Image
import Quartz
import datetime
from typing import List
import base64
import io
from anthropic import Anthropic  # or use openai

class Window:
    windowId: int
    title: str
    processId: int
    bundleId: str
    applicationName: str
    windowLayer: int
    x: int
    y: int
    width: int
    height: int
    z: int
    
    def __init__(self, windowId: int, title: str, processId: int, bundleId: str, applicationName: str, windowLayer: int, x: int, y: int, width: int, height: int, z: int):
        self.windowId = windowId
        self.title = title
        self.processId = processId
        self.bundleId = bundleId
        self.applicationName = applicationName
        self.windowLayer = windowLayer
        self.x = x
        self.y = y
        self.width = width
        self.height = height
        self.z = z

class Display:
    displayId: int
    x: int
    y: int
    width: int
    height: int
    screenshot: Image

    def __init__(self, displayId: int, x: int, y: int, width: int, height: int, screenshot: Image):
        self.displayId = displayId
        self.x = x
        self.y = y
        self.width = width
        self.height = height
        self.screenshot = screenshot

class CaptureEvent:
    name: str
    time: datetime.datetime
    displays: List[Display]
    windows: List[Window]

    def __init__(self, name: str, time: datetime.datetime, displays: List[Display], windows: List[Window]):
        self.name = name
        self.time = time
        self.displays = displays
        self.windows = windows

def get_window_order():
    """Get window IDs in z-order (frontmost first)."""
    window_list = Quartz.CGWindowListCopyWindowInfo(
        Quartz.kCGWindowListOptionOnScreenOnly | Quartz.kCGWindowListExcludeDesktopElements,
        Quartz.kCGNullWindowID
    )
    
    # Returns dict: window_id -> z_index (0 = frontmost)
    order = {}
    for i, window in enumerate(window_list):
        if window.get(Quartz.kCGWindowLayer, 0) == 0:  # Normal windows only
            window_id = window.get(Quartz.kCGWindowNumber)
            order[window_id] = i
    return order

class CaptureHelper:
    def __init__(self):
        self.shareable_content = None
    
    def _capture(self):
        event = Event()
        error_holder = [None]
        content_holder = [None]
        def handler(content, error):
            content_holder[0] = content
            error_holder[0] = error
            event.set()
        sck.SCShareableContent.getShareableContentWithCompletionHandler_(handler)
        # Run the run loop until the handler completes
        while not event.is_set():
            NSRunLoop.currentRunLoop().runMode_beforeDate_(
                "NSDefaultRunLoopMode", 
                NSDate.dateWithTimeIntervalSinceNow_(0.1)
            )
        if error_holder[0]:
            raise PermissionError(
                f"Screen capture not authorized. Enable in System Settings → "
                f"Privacy & Security → Screen Recording. Error: {error_holder[0]}"
            )
        self.shareable_content = content_holder[0]

    def get_capture_event(self):
        self._capture()
        displays = []
        windows = []

        window_order = get_window_order()

        for display in self.shareable_content.displays():
            screenshot = self._capture_screenshot(display)
            img = self._cgimage_to_pil(screenshot)
            displays.append(Display(
                displayId=display.displayID(),
                x=display.frame().origin.x,
                y=display.frame().origin.y,
                width=display.frame().size.width,
                height=display.frame().size.height,
                screenshot=img,
            ))
        for window in self.shareable_content.windows():
            if window.windowLayer() != 0:
                continue
            if window.frame().size.width == 0 or window.frame().size.height == 0:
                continue
            if window.title() == "":
                continue
            if not window.isOnScreen():
                continue
            if not window.isActive():
                continue
            z_index = window_order.get(window.windowID(), 999)
            windows.append(Window(
                windowId=window.windowID(),
                title=window.title(),
                processId=window.owningApplication().processID(),
                bundleId=window.owningApplication().bundleIdentifier(),
                applicationName=window.owningApplication().applicationName(),
                windowLayer=window.windowLayer(),
                x=window.frame().origin.x,
                y=window.frame().origin.y,
                width=window.frame().size.width,
                height=window.frame().size.height,
                z=z_index,
            ))

        return CaptureEvent(
            name="capture",
            time=datetime.datetime.now(),
            displays=displays,
            windows=windows,
        )

    def _capture_screenshot(self, display):
        # Create filter for this display (exclude nothing)
        content_filter = sck.SCContentFilter.alloc().initWithDisplay_excludingWindows_(
            display, []
        )
        
        # Configure capture
        config = sck.SCStreamConfiguration.alloc().init()
        
        # Capture screenshot
        event = Event()
        image_holder = [None]
        error_holder = [None]
        
        def capture_handler(image, error):
            image_holder[0] = image
            error_holder[0] = error
            event.set()
        
        sck.SCScreenshotManager.captureImageWithFilter_configuration_completionHandler_(
            content_filter, config, capture_handler
        )
        
        while not event.is_set():
            NSRunLoop.currentRunLoop().runMode_beforeDate_(
                "NSDefaultRunLoopMode",
                NSDate.dateWithTimeIntervalSinceNow_(0.1)
            )
        
        if error_holder[0]:
            raise RuntimeError(f"Capture failed: {error_holder[0]}")
        
        return image_holder[0]

    def _cgimage_to_pil(self, cgimage):
        width = Quartz.CGImageGetWidth(cgimage)
        height = Quartz.CGImageGetHeight(cgimage)
        pixeldata = Quartz.CGDataProviderCopyData(Quartz.CGImageGetDataProvider(cgimage))
        bpr = Quartz.CGImageGetBytesPerRow(cgimage)
        return Image.frombuffer("RGBA", (width, height), pixeldata, "raw", "BGRA", bpr, 1)

def show_notification(title: str, message: str):
    subprocess.run(["osascript", "-e", f'display notification "{message}" with title "{title}"'])

def analyze_productivity(capture_event: CaptureEvent, goal: str) -> dict:
    """Send a CaptureEvent to an LLM to analyze if the user is productive."""
    
    # Convert screenshots to base64
    images_base64 = []
    for display in capture_event.displays:
        buffer = io.BytesIO()
        display.screenshot.save(buffer, format="PNG")
        img_base64 = base64.standard_b64encode(buffer.getvalue()).decode("utf-8")
        images_base64.append(img_base64)
    
    # Build window context
    window_info = "\n".join([
        f"- {w.applicationName}: '{w.title}' (z-order: {w.z})"
        for w in sorted(capture_event.windows, key=lambda w: w.z)
    ])
    
    # Create the prompt
    prompt = f"""The user's goal for this session is: "{goal}"

Current time: {capture_event.time.strftime("%H:%M:%S")}

Open windows (front to back):
{window_info}

Based on the screenshot(s) and window information, analyze:
1. Is the user currently focused on their stated goal?
2. What are they actually doing?
3. Productivity score (0-10)
4. Brief suggestion if they're off-track

Respond as JSON with keys: focused (bool), activity (str), score (int), suggestion (str or null)"""

    # Call the LLM with vision
    client = Anthropic()
    
    content = [{"type": "text", "text": prompt}]
    for img_b64 in images_base64:
        content.append({
            "type": "image",
            "source": {
                "type": "base64",
                "media_type": "image/png",
                "data": img_b64,
            }
        })
    
    response = client.messages.create(
        model="claude-sonnet-4-20250514",
        max_tokens=500,
        messages=[
            {"role": "user", "content": content},
            {"role": "assistant", "content": "{"}
        ]
    )
    
    return json.loads("{" + response.content[0].text)

def main():
    goal = input("What's your goal for this session? ")
    duration_minutes = 25
    interval_seconds = 30

    end_time = time.time() + (duration_minutes * 60)
    capture_helper = CaptureHelper()

    print(f"Running focus checks every {interval_seconds}s for {duration_minutes} minutes. Press Ctrl+C to stop.")

    printed_layout = False
    try:
        while time.time() < end_time:
            capture_event = capture_helper.get_capture_event()

            # Print display/window layout once (useful for debugging without spamming).
            if not printed_layout:
                for display in capture_event.displays:
                    print(
                        f"Display {display.displayId} ({display.x}, {display.y}, {display.width}, {display.height})"
                    )
                for window in capture_event.windows:
                    print(
                        f"Window {window.windowId} {window.applicationName} {window.title} "
                        f"at {window.x}, {window.y}, {window.width}, {window.height} "
                        f"(layer {window.windowLayer}, z {window.z})"
                    )
                printed_layout = True

            result = analyze_productivity(capture_event, goal=goal)
            ts = capture_event.time.strftime("%H:%M:%S")
            suggestion = result.get("suggestion")
            print(f"[{ts}] Focused: {result.get('focused')}, Score: {result.get('score')}/10")
            if suggestion:
                print(f"[{ts}] Suggestion: {suggestion}")
                show_notification("Focus Check", suggestion)

            remaining = end_time - time.time()
            if remaining <= 0:
                break
            time.sleep(min(interval_seconds, remaining))
    except KeyboardInterrupt:
        print("\nStopped.")

if __name__ == "__main__":
    main()