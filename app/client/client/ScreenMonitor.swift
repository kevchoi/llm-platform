//
//  ScreenMonitor.swift
//  client
//
//  Created by kev on 2025-11-25.
//

import Foundation
import ScreenCaptureKit
import Combine

// Simple struct for JSON encoding
struct WindowInfo: Codable {
    let windowID: UInt32
    let title: String
    let app: String
    let bundleID: String
    let x: CGFloat
    let y: CGFloat
    let width: CGFloat
    let height: CGFloat
    let isActive: Bool
}

@MainActor
class ScreenMonitor: ObservableObject {
    @Published var windows: [SCWindow] = []
    
    init () {}

    func getWindows() async {
        do {
            let content = try await SCShareableContent.excludingDesktopWindows(true, onScreenWindowsOnly: true)
            
            // Filter out menu bar items and system UI elements
            // Normal app windows have windowLayer == 0
            // Menu bar, Dock, Spotlight etc. have higher layer values
            let filteredWindows = content.windows.filter { window in
                window.windowLayer == 0 &&
                window.frame.width > 0 &&
                window.frame.height > 0
            }
            
            print("Fetched \(filteredWindows.count) windows (filtered from \(content.windows.count))")
            
            self.windows = filteredWindows
            for window in filteredWindows {
                print("""
                    Window ID: \(window.windowID)
                    Title: \(window.title ?? "No Title")
                    App: \(window.owningApplication?.applicationName ?? "Unknown")
                    Bundle ID: \(window.owningApplication?.bundleIdentifier ?? "Unknown")
                    Frame: \(window.frame)
                    Is Active: \(window.isActive)
                    Is On Screen: \(window.isOnScreen)
                    Window Layer: \(window.windowLayer)
                    ---
                    """)
            }
        } catch {
            print("Error fetching windows: \(error.localizedDescription)")
        }
    }
    
    func sendToServer() async {
        let windowInfos = windows.map { window in
            WindowInfo(
                windowID: window.windowID,
                title: window.title ?? "",
                app: window.owningApplication?.applicationName ?? "",
                bundleID: window.owningApplication?.bundleIdentifier ?? "",
                x: window.frame.origin.x,
                y: window.frame.origin.y,
                width: window.frame.width,
                height: window.frame.height,
                isActive: window.isActive
            )
        }
        
        guard let url = URL(string: "http://localhost:8080/windows") else { return }
        
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        
        do {
            request.httpBody = try JSONEncoder().encode(windowInfos)
            let (_, response) = try await URLSession.shared.data(for: request)
            print("Sent to server: \((response as? HTTPURLResponse)?.statusCode ?? 0)")
        } catch {
            print("Error sending to server: \(error)")
        }
    }

    func captureScreen() async throws -> CGImage? {
        let content = try await SCShareableContent.excludingDesktopWindows(false, onScreenWindowsOnly: true)
        
        guard let display = content.displays.first else { return nil }
        
        let filter = SCContentFilter(display: display, excludingWindows: [])
        let config = SCStreamConfiguration()
        config.width = Int(display.width)
        config.height = Int(display.height)
    
        return try await SCScreenshotManager.captureImage(
            contentFilter: filter,
            configuration: config
        )
    }

    func sendScreenshot() async {
        do {
            guard let cgImage = try await captureScreen() else {
                print("Failed to capture screen")
                return
            }
            
            // Convert CGImage to PNG data
            let bitmapRep = NSBitmapImageRep(cgImage: cgImage)
            guard let pngData = bitmapRep.representation(using: .png, properties: [:]) else {
                print("Failed to convert to PNG")
                return
            }
            
            // Create multipart request
            let boundary = UUID().uuidString
            guard let url = URL(string: "http://localhost:8080/screenshot") else { return }
            
            var request = URLRequest(url: url)
            request.httpMethod = "POST"
            request.setValue("multipart/form-data; boundary=\(boundary)", forHTTPHeaderField: "Content-Type")
            
            // Build multipart body
            var body = Data()
            body.append("--\(boundary)\r\n".data(using: .utf8)!)
            body.append("Content-Disposition: form-data; name=\"screenshot\"; filename=\"screenshot.png\"\r\n".data(using: .utf8)!)
            body.append("Content-Type: image/png\r\n\r\n".data(using: .utf8)!)
            body.append(pngData)
            body.append("\r\n--\(boundary)--\r\n".data(using: .utf8)!)
            
            request.httpBody = body
            
            let (_, response) = try await URLSession.shared.data(for: request)
            print("Screenshot sent: \((response as? HTTPURLResponse)?.statusCode ?? 0)")
        } catch {
            print("Error sending screenshot: \(error)")
        }
    }
}
