import Foundation
import Combine

// MARK: - Network Manager Protocol

/// Manages communication with the remote server
public protocol NetworkManagerProtocol: Actor {
    func upload(captureInfos: [CaptureInfo]) async throws -> UploadResponse
}

// MARK: - Network Manager Implementation

public actor NetworkManager: NetworkManagerProtocol {
    public init() {}
    
    public func upload(captureInfos: [CaptureInfo]) async throws -> UploadResponse {
        let url = URL(string: "http://localhost:8080/upload")!
        let request = URLRequest(url: url)
        let (data, response) = try await URLSession.shared.data(for: request)
        return try JSONDecoder().decode(UploadResponse.self, from: data)
    }
}