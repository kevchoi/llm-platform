from __future__ import annotations

import base64
import io
import json
import subprocess
import time
from dataclasses import dataclass
from datetime import datetime
from threading import Event, Thread
from typing import Any, Callable

import Quartz
import ScreenCaptureKit as sck
from AppKit import NSDate, NSRunLoop
from anthropic import Anthropic  # or use openai
from PIL import Image

@dataclass(frozen=True, slots=True)
class Window:
    window_id: int
    title: str
    process_id: int
    bundle_id: str
    application_name: str
    window_layer: int
    x: float
    y: float
    width: float
    height: float
    z: int


@dataclass(frozen=True, slots=True)
class Display:
    display_id: int
    x: float
    y: float
    width: float
    height: float
    screenshot: Image.Image


@dataclass(frozen=True, slots=True)
class CaptureEvent:
    name: str
    time: datetime
    displays: list[Display]
    windows: list[Window]


def _spin_runloop_until(event: Event, poll_seconds: float = 0.1) -> None:
    while not event.is_set():
        NSRunLoop.currentRunLoop().runMode_beforeDate_(
            "NSDefaultRunLoopMode",
            NSDate.dateWithTimeIntervalSinceNow_(poll_seconds),
        )


def _await_objc_completion(
    register: Callable[[Callable[[Any, Any], None]], None],
) -> tuple[Any, Any]:
    """
    Await an ObjC async API that accepts a completion handler: (result, error) -> None.
    Returns (result, error).
    """
    event = Event()
    result_holder: list[Any] = [None]
    error_holder: list[Any] = [None]

    def handler(result: Any, error: Any) -> None:
        result_holder[0] = result
        error_holder[0] = error
        event.set()

    register(handler)
    _spin_runloop_until(event)
    return result_holder[0], error_holder[0]

def get_window_order() -> dict[int, int]:
    """Get window IDs in z-order (frontmost first)."""
    window_list = Quartz.CGWindowListCopyWindowInfo(
        Quartz.kCGWindowListOptionOnScreenOnly | Quartz.kCGWindowListExcludeDesktopElements,
        Quartz.kCGNullWindowID
    )
    if not window_list:
        return {}
    
    # Returns dict: window_id -> z_index (0 = frontmost)
    order = {}
    for i, window in enumerate(window_list):
        if window.get(Quartz.kCGWindowLayer, 0) == 0:  # Normal windows only
            window_id = window.get(Quartz.kCGWindowNumber)
            order[window_id] = i
    return order

class CaptureHelper:
    def __init__(self):
        self._shareable_content = None
    
    def _ensure_shareable_content(self) -> None:
        if self._shareable_content is not None:
            return

        content, error = _await_objc_completion(
            sck.SCShareableContent.getShareableContentWithCompletionHandler_
        )
        if error:
            raise PermissionError(
                f"Screen capture not authorized. Enable in System Settings → "
                f"Privacy & Security → Screen Recording. Error: {error}"
            )
        self._shareable_content = content

    def get_capture_event(self) -> CaptureEvent:
        self._ensure_shareable_content()
        displays = []
        windows = []

        window_order = get_window_order()

        for display in self._shareable_content.displays():
            screenshot = self._capture_screenshot(display)
            img = self._cgimage_to_pil(screenshot)
            displays.append(Display(
                display_id=display.displayID(),
                x=display.frame().origin.x,
                y=display.frame().origin.y,
                width=display.frame().size.width,
                height=display.frame().size.height,
                screenshot=img,
            ))
        for window in self._shareable_content.windows():
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
                window_id=window.windowID(),
                title=window.title(),
                process_id=window.owningApplication().processID(),
                bundle_id=window.owningApplication().bundleIdentifier(),
                application_name=window.owningApplication().applicationName(),
                window_layer=window.windowLayer(),
                x=window.frame().origin.x,
                y=window.frame().origin.y,
                width=window.frame().size.width,
                height=window.frame().size.height,
                z=z_index,
            ))

        return CaptureEvent(
            name="capture",
            time=datetime.now(),
            displays=displays,
            windows=windows,
        )

    def _capture_screenshot(self, display: Any) -> Any:
        # Create filter for this display (exclude nothing)
        content_filter = sck.SCContentFilter.alloc().initWithDisplay_excludingWindows_(
            display, []
        )
        
        # Configure capture
        config = sck.SCStreamConfiguration.alloc().init()
        
        # Capture screenshot
        image, error = _await_objc_completion(
            lambda handler: sck.SCScreenshotManager.captureImageWithFilter_configuration_completionHandler_(
                content_filter, config, handler
            )
        )
        if error:
            raise RuntimeError(f"Capture failed: {error}")
        return image

    def _cgimage_to_pil(self, cgimage: Any) -> Image.Image:
        width = Quartz.CGImageGetWidth(cgimage)
        height = Quartz.CGImageGetHeight(cgimage)
        pixeldata = Quartz.CGDataProviderCopyData(Quartz.CGImageGetDataProvider(cgimage))
        bpr = Quartz.CGImageGetBytesPerRow(cgimage)
        return Image.frombuffer("RGBA", (width, height), pixeldata, "raw", "BGRA", bpr, 1)

def show_notification(title: str, message: str):
    subprocess.run(
        ["osascript", "-e", f'display notification "{message}" with title "{title}"'],
        check=False,
    )

def _safe_show_notification(title: str, message: str) -> None:
    """
    Best-effort notifications: never crash the CLI if osascript fails.
    (E.g. running in CI / missing permissions / no UI session.)
    """
    try:
        show_notification(title, message)
    except Exception:
        # Notifications are optional; terminal output is still shown.
        pass

def _extract_json_object(text: str) -> dict[str, Any]:
    """
    Best-effort JSON extraction.
    The model should return a JSON object, but we defensively handle extra text/code fences.
    """
    text = text.strip()
    try:
        value = json.loads(text)
        if isinstance(value, dict):
            return value
    except json.JSONDecodeError:
        pass

    start = text.find("{")
    end = text.rfind("}")
    if start != -1 and end != -1 and end > start:
        candidate = text[start : end + 1]
        value = json.loads(candidate)
        if isinstance(value, dict):
            return value

    raise ValueError(f"Could not parse model response as JSON object. Response was: {text[:500]!r}")


def analyze_productivity(capture_event: CaptureEvent, goal: str) -> dict[str, Any]:
    """Send a CaptureEvent to an LLM to analyze if the user is productive."""
    
    # Convert screenshots to base64
    images_base64 = []
    for display in capture_event.displays:
        buffer = io.BytesIO()
        display.screenshot.save(buffer, format="PNG")
        img_base64 = base64.standard_b64encode(buffer.getvalue()).decode("utf-8")
        images_base64.append(img_base64)
    
    # Build window context
    window_info = "\n".join(
        f"- {w.application_name}: '{w.title}' (z-order: {w.z})"
        for w in sorted(capture_event.windows, key=lambda w: w.z)
    )
    
    # Create the prompt
    prompt = f"""The user's goal for this session is: "{goal}"

Current time: {capture_event.time.strftime("%H:%M:%S")}

Open windows (front to back):
{window_info}

Based on the screenshot(s) and window information, analyze:
1. Is the user currently focused on their stated goal?
2. What are they actually doing?
3. Productivity score (0-10)
4. Brief one-sentence suggestion if they're off-track

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
        messages=[{"role": "user", "content": content}],
    )
    
    return _extract_json_object(response.content[0].text)

def _goal_reminder_loop(stop: Event, interval_seconds: float, initial_delay_seconds: float) -> None:
    """
    While we're blocked on input(), periodically remind the user to set a goal.
    Runs in a daemon thread and stops when `stop` is set.
    """
    if initial_delay_seconds > 0 and stop.wait(timeout=initial_delay_seconds):
        return

    msg = "Please enter a goal to start focus checks."
    while not stop.is_set():
        _safe_show_notification("Set a goal", msg)
        stop.wait(timeout=interval_seconds)

def prompt_for_goal() -> str:
    """Continuously prompt until the user provides a non-empty goal."""
    while True:
        stop = Event()
        # Keep this frequent enough to be noticeable, but not constant spam.
        reminder_thread = Thread(
            target=_goal_reminder_loop,
            args=(stop, 15.0, 3.0),
            daemon=True,
        )
        reminder_thread.start()

        goal = input("What's your goal for this session? ").strip()
        stop.set()
        reminder_thread.join(timeout=1.0)
        if goal:
            return goal
        print("Please enter a goal (it can't be empty). Press Ctrl+C to exit.")
        _safe_show_notification("Goal required", "Please enter a goal (it can't be empty).")

def main():
    duration_minutes = 25
    interval_seconds = 30
    capture_helper = CaptureHelper()

    print(
        f"Running focus checks every {interval_seconds}s for {duration_minutes} minutes per goal. "
        f"Press Ctrl+C to stop."
    )

    try:
        while True:
            goal = prompt_for_goal()
            end_time = time.time() + (duration_minutes * 60)

            printed_layout = False
            while time.time() < end_time:
                capture_event = capture_helper.get_capture_event()

                # Print display/window layout once (useful for debugging without spamming).
                if not printed_layout:
                    for display in capture_event.displays:
                        print(
                            f"Display {display.display_id} ({display.x}, {display.y}, {display.width}, {display.height})"
                        )
                    for window in capture_event.windows:
                        print(
                            f"Window {window.window_id} {window.application_name} {window.title} "
                            f"at {window.x}, {window.y}, {window.width}, {window.height} "
                            f"(layer {window.window_layer}, z {window.z})"
                        )
                    printed_layout = True

                result = analyze_productivity(capture_event, goal=goal)
                ts = capture_event.time.strftime("%H:%M:%S")
                suggestion = result.get("suggestion")
                print(f"[{ts}] Focused: {result.get('focused')}, Score: {result.get('score')}/10")
                if suggestion:
                    print(f"[{ts}] Suggestion: {suggestion}")
                    _safe_show_notification("Focus Check", str(suggestion))

                remaining = end_time - time.time()
                if remaining <= 0:
                    break
                time.sleep(min(interval_seconds, remaining))

            print("\nSession complete — let's set a new goal.\n")
    except KeyboardInterrupt:
        print("\nStopped.")

if __name__ == "__main__":
    main()