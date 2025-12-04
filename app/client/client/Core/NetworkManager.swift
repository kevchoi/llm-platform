import Foundation

public protocol NetworkManagerProtocol: Actor {
    func upload(captureInfos: [CaptureInfo]) async throws -> UploadResponse
}

public struct UploadResponse: Codable, Sendable {
    public let success: Bool
}

public actor NetworkManager: NetworkManagerProtocol {
    public init() {}
    
    public func upload(captureInfos: [CaptureInfo]) async throws -> UploadResponse {
        let url = URL(string: "http://localhost:8080/upload")!
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.setValue("image/png", forHTTPHeaderField: "Content-Type")
        request.httpBody = captureInfos.first?.displayInfo.screenshot
        
        let (data, _) = try await URLSession.shared.data(for: request)
        return try JSONDecoder().decode(UploadResponse.self, from: data)
    }
}
