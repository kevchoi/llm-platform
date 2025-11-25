//
//  ScreenMonitor.swift
//  client
//
//  Created by kev on 2025-11-25.
//

import Foundation
import ScreenCaptureKit
import Combine

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
}
