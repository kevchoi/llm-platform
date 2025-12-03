//
//  ContentView.swift
//  client
//
//  Created by kev on 2025-11-24.
//

import SwiftUI

struct ContentView: View {
    @State private var captureManager: CaptureManagerProtocol = CaptureManager()
    
    var body: some View {
        VStack {
            Image(systemName: "globe")
                .imageScale(.large)
                .foregroundStyle(.tint)
            Text("Hello, world!")
        }
        .padding()
        Button {
            Task {
                let captureInfos = try? await captureManager.capture()
                if let captureInfos = captureInfos {
                    for captureInfo in captureInfos {
                        print("Display ID: \(captureInfo.displayInfo.displayId)")
                        print("Frame: \(captureInfo.displayInfo.frame)")
                        print("Width: \(captureInfo.displayInfo.width)")
                        print("Height: \(captureInfo.displayInfo.height)")
                        print("Scale Factor: \(captureInfo.displayInfo.scaleFactor)")
                        print("Screenshot: \(captureInfo.displayInfo.screenshot != nil)")
                        print("Event: \(captureInfo.event)")
                        print("Timestamp: \(captureInfo.timestamp)")
                        print("--------------------------------")
                        for windowInfo in captureInfo.windowInfos {
                            print("Window ID: \(windowInfo.windowID)")
                            print("Process ID: \(windowInfo.processID)")
                            print("Display ID: \(windowInfo.displayID)")
                            print("Title: \(windowInfo.title)")
                            print("App Name: \(windowInfo.appName)")
                            print("Bundle ID: \(windowInfo.bundleID)")
                            print("Frame: \(windowInfo.frame)")
                            print("Is Active: \(windowInfo.isActive)")
                            print("Is On Screen: \(windowInfo.isOnScreen)")
                            print("Window Layer: \(windowInfo.windowLayer)")
                        }
                        print("--------------------------------")
                    }
                }
            }
        } label: {
            Text("Capture")
                .font(.title3)
                .fontWeight(.semibold)
                .frame(maxWidth: .infinity)
                .padding()
        }
    }
}

#Preview {
    ContentView()
}
