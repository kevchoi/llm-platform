import Foundation
import ScreenCaptureKit

// MARK: - Protocol

public protocol CaptureManagerProtocol: Actor {
    func requestPermission() async -> Bool
    func capture() async throws -> [CaptureInfo]
    var hasPermission: Bool { get async }
}

// MARK: - Events

public enum CaptureEvent: Sendable {
    case initial
    case windowCreated(WindowInfo)
    case windowDestroyed(WindowInfo)
    case windowMoved(WindowInfo, from: CGRect)
    case windowResized(WindowInfo, from: CGSize)
    case windowTitleChanged(WindowInfo, from: String)
    case windowActiveChanged(WindowInfo)
    case windowLayerChanged(WindowInfo, from: Int)
}

// MARK: - Models

public struct WindowInfo: Sendable, Equatable {
    public let windowID: CGWindowID
    public let processID: pid_t
    public let displayID: CGDirectDisplayID
    public let title: String
    public let appName: String
    public let bundleID: String
    public let frame: CGRect
    public let isActive: Bool
    public let isOnScreen: Bool
    public let windowLayer: Int
    
    public nonisolated init(
        windowID: CGWindowID,
        processID: pid_t,
        displayID: CGDirectDisplayID,
        title: String,
        appName: String,
        bundleID: String,
        frame: CGRect,
        isActive: Bool,
        isOnScreen: Bool,
        windowLayer: Int
    ) {
        self.windowID = windowID
        self.processID = processID
        self.displayID = displayID
        self.title = title
        self.appName = appName
        self.bundleID = bundleID
        self.frame = frame
        self.isActive = isActive
        self.isOnScreen = isOnScreen
        self.windowLayer = windowLayer
    }
}

public struct DisplayInfo: Sendable {
    public let displayId: CGDirectDisplayID
    public let frame: CGRect
    public let width: Int
    public let height: Int
    public let scaleFactor: CGFloat
    public let screenshot: Data?
    
    public nonisolated init(
        displayId: CGDirectDisplayID,
        frame: CGRect,
        width: Int,
        height: Int,
        scaleFactor: CGFloat = 1.0,
        screenshot: Data? = nil
    ) {
        self.displayId = displayId
        self.frame = frame
        self.width = width
        self.height = height
        self.scaleFactor = scaleFactor
        self.screenshot = screenshot
    }
}

public struct CaptureInfo: Sendable {
    public let displayInfo: DisplayInfo
    public let windowInfos: [WindowInfo]
    public let event: CaptureEvent
    public let timestamp: Date
    
    public nonisolated init(
        displayInfo: DisplayInfo,
        windowInfos: [WindowInfo],
        event: CaptureEvent,
        timestamp: Date = Date()
    ) {
        self.displayInfo = displayInfo
        self.windowInfos = windowInfos
        self.event = event
        self.timestamp = timestamp
    }
}

// MARK: - Implementation
public actor CaptureManager: CaptureManagerProtocol {
    
    private var _hasPermission: Bool = false
    
    public init() {}
    
    public var hasPermission: Bool {
        get async {
            // Check current permission status by attempting to get shareable content
            do {
                _ = try await SCShareableContent.excludingDesktopWindows(true, onScreenWindowsOnly: true)
                _hasPermission = true
                return true
            } catch {
                _hasPermission = false
                return false
            }
        }
    }
    
    public func requestPermission() async -> Bool {
        // ScreenCaptureKit doesn't have an explicit permission request API.
        // Permission is requested automatically when you first access SCShareableContent.
        // The system will show a prompt if permission hasn't been granted.
        do {
            _ = try await SCShareableContent.excludingDesktopWindows(true, onScreenWindowsOnly: true)
            _hasPermission = true
            return true
        } catch {
            _hasPermission = false
            return false
        }
    }
    
    public func capture() async throws -> [CaptureInfo] {
        let content = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: true)
        
        var captureInfos: [CaptureInfo] = []
        
        // Get the frontmost application to determine active window
        let frontmostApp = NSWorkspace.shared.frontmostApplication
        
        for display in content.displays {
            // Capture screenshot for this display
            let screenshot = try await captureScreenshot(for: display)
            
            let displayInfo = DisplayInfo(
                displayId: display.displayID,
                frame: display.frame,
                width: display.width,
                height: display.height,
                scaleFactor: CGFloat(display.width) / display.frame.width,
                screenshot: screenshot
            )
            
            // Filter windows for this display
            let windowsOnDisplay = content.windows.filter { window in
                window.windowLayer == 0 &&
                window.isOnScreen &&
                window.frame.width > 0 &&
                window.frame.height > 0 &&
                display.frame.contains(window.frame.origin)
            }
            
            // Windows come in z-order (front to back). Track covered regions.
            var visibleWindows: [SCWindow] = []
            var coveredRegion = CGRect.zero
            
            for window in windowsOnDisplay {
                // Check if this window is completely covered
                if coveredRegion.contains(window.frame) {
                    continue  // Skip fully occluded windows
                }
                
                visibleWindows.append(window)
                
                // Expand covered region (simple union - not pixel-perfect but works)
                // Doesn't work that well; use a more sophisticated algorithm.
                coveredRegion = coveredRegion.union(window.frame)
            }
            
            let windowInfos = visibleWindows.map { window -> WindowInfo in
                let isActive = window.owningApplication?.processID == frontmostApp?.processIdentifier && window.isActive
                
                return WindowInfo(
                    windowID: window.windowID,
                    processID: window.owningApplication?.processID ?? 0,
                    displayID: display.displayID,
                    title: window.title ?? "",
                    appName: window.owningApplication?.applicationName ?? "",
                    bundleID: window.owningApplication?.bundleIdentifier ?? "",
                    frame: window.frame,
                    isActive: isActive,
                    isOnScreen: window.isOnScreen,
                    windowLayer: window.windowLayer
                )
            }
            
            let captureInfo = CaptureInfo(
                displayInfo: displayInfo,
                windowInfos: windowInfos,
                event: .initial
            )
            
            captureInfos.append(captureInfo)
        }
        
        return captureInfos
    }
    
    // MARK: - Private Helpers
    
    private func captureScreenshot(for display: SCDisplay) async throws -> Data? {
        let filter = SCContentFilter(display: display, excludingWindows: [])
        let config = SCStreamConfiguration()
        config.width = display.width
        config.height = display.height
        config.pixelFormat = kCVPixelFormatType_32BGRA
        
        let cgImage = try await SCScreenshotManager.captureImage(
            contentFilter: filter,
            configuration: config
        )
        
        let bitmapRep = NSBitmapImageRep(cgImage: cgImage)
        return bitmapRep.representation(using: .png, properties: [:])
    }
}
