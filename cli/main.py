import time
import subprocess
import ScreenCaptureKit as sck
from threading import Event
from AppKit import NSRunLoop, NSDate
from PIL import Image
import Quartz

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

    def get_window_info(self):
        self._capture()
        for display in self.shareable_content.displays():
            print(display.displayID())
            print(display.frame())
            screenshot = self._capture_screenshot(display)
            img = self._cgimage_to_pil(screenshot)
            img.save(f"screenshot_{display.displayID()}.png")
            img.show()
        # for window in self.shareable_content.windows():
        #     if window.windowLayer() == 0 and window.title() != "":
        #         print(window.owningApplication())
        #         print(window.title())
        #         print(window.frame())

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

def main():
    goal = input("What's your goal for this session? ")
    duration_minutes = 25
    interval_seconds = 60

    end_time = time.time() + (duration_minutes * 60)

if __name__ == "__main__":
    capture_helper = CaptureHelper()
    capture_helper.get_window_info()