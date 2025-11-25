//
//  ContentView.swift
//  client
//
//  Created by kev on 2025-11-24.
//

import SwiftUI

struct ContentView: View {
    @StateObject private var screenMonitor = ScreenMonitor()
    
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
                await screenMonitor.getWindows()
            }
        } label: {
            Text("Get Windows")
                .font(.title3)
                .fontWeight(.semibold)
                .frame(maxWidth: .infinity)
                .padding()
                .foregroundColor(.white)
        }
        .buttonStyle(.plain)
        .cornerRadius(10)
    }
}

#Preview {
    ContentView()
}
