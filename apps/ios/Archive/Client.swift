import AuthenticationServices
import CryptoKit
import Foundation
import Security
import UIKit

enum ClientError: LocalizedError {
    case signedOut
    case refused(Int, String)

    var errorDescription: String? {
        switch self {
        case .signedOut: "Signed out"
        case let .refused(status, body): "Archive said \(status): \(body)"
        }
    }
}

/// The phone's way into Archive: Archive's own OAuth, the one MCP clients use, with PKCE. The phone registers itself as a client once,
/// signs in on the web app, and keeps the rotating refresh token in the Keychain; /device/{stream} takes the access token.
actor Client {
    static let shared = Client()
    static let api = URL(string: Bundle.main.object(forInfoDictionaryKey: "ArchiveAPI") as! String)!
    static let scheme = "dev.archive.app"
    static let redirect = scheme + ":/oauth"

    private var access: String?
    private var until = Date.distantPast
    private var refreshing: Task<String, Error>?

    nonisolated var signedIn: Bool { Keychain.get("refresh") != nil }

    func signIn() async throws {
        let client = try await clientID()
        let verifier = Self.random()
        let state = Self.random()
        var url = URLComponents(url: Self.api.appending(path: "oauth/authorize"), resolvingAgainstBaseURL: false)!
        url.queryItems = [
            .init(name: "response_type", value: "code"), .init(name: "client_id", value: client),
            .init(name: "redirect_uri", value: Self.redirect), .init(name: "state", value: state),
            .init(name: "code_challenge", value: Data(SHA256.hash(data: Data(verifier.utf8))).base64URL),
            .init(name: "code_challenge_method", value: "S256"),
        ]
        let back = try await Web.open(url.url!)
        let q = URLComponents(url: back, resolvingAgainstBaseURL: false)?.queryItems ?? []
        let item = { (name: String) in q.first { $0.name == name }?.value }
        guard item("state") == state, let code = item("code") else {
            throw ClientError.refused(400, item("error_description") ?? "Sign-in did not finish")
        }
        _ = try await token(["grant_type": "authorization_code", "code": code, "redirect_uri": Self.redirect, "code_verifier": verifier, "client_id": client])
    }

    func signOut() async {
        if let refresh = Keychain.get("refresh"), let client = Keychain.get("client") {
            _ = try? await form("oauth/revoke", ["token": refresh, "client_id": client])
        }
        Keychain.set("refresh", nil)
        access = nil
    }

    /// Lands records of a stream in Archive: [{ id, data }] or [{ id, deleted: true }]; full when they are all of it, more when another batch of it follows.
    func push(_ stream: String, _ records: [[String: Any]], full: Bool = false, more: Bool = false) async throws {
        var req = URLRequest(url: Self.api.appending(path: "device/\(stream)"))
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONSerialization.data(withJSONObject: ["device": await UIDevice.current.model, "records": records, "full": full, "more": more])
        for attempt in 0 ..< 2 {
            req.setValue("Bearer " + (try await bearer()), forHTTPHeaderField: "Authorization")
            let (body, res) = try await URLSession.shared.data(for: req)
            let status = (res as? HTTPURLResponse)?.statusCode ?? 0
            if status == 401, attempt == 0 {
                access = nil
                continue
            }
            guard status == 200 else { throw ClientError.refused(status, String(decoding: body, as: UTF8.self)) }
            return
        }
    }

    private func bearer() async throws -> String {
        if let access, until > .now { return access }
        if let refreshing { return try await refreshing.value }
        guard let refresh = Keychain.get("refresh"), let client = Keychain.get("client") else { throw ClientError.signedOut }
        let task = Task { try await token(["grant_type": "refresh_token", "refresh_token": refresh, "client_id": client]) }
        refreshing = task
        defer { refreshing = nil }
        return try await task.value
    }

    private func token(_ params: [String: String]) async throws -> String {
        let (body, status) = try await form("oauth/token", params)
        if status == 400, params["grant_type"] == "refresh_token" {
            Keychain.set("refresh", nil)
            throw ClientError.signedOut
        }
        struct Answer: Decodable { let access_token: String, refresh_token: String, expires_in: Double }
        guard status == 200, let a = try? JSONDecoder().decode(Answer.self, from: body) else {
            throw ClientError.refused(status, String(decoding: body, as: UTF8.self))
        }
        Keychain.set("refresh", a.refresh_token)
        access = a.access_token
        until = Date.now.addingTimeInterval(a.expires_in - 60)
        return a.access_token
    }

    private func clientID() async throws -> String {
        if let id = Keychain.get("client") { return id }
        var req = URLRequest(url: Self.api.appending(path: "oauth/register"))
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let name = "Archive for " + (await UIDevice.current.model)
        req.httpBody = try JSONSerialization.data(withJSONObject: ["client_name": name, "redirect_uris": [Self.redirect], "token_endpoint_auth_method": "none"])
        let (body, res) = try await URLSession.shared.data(for: req)
        struct Answer: Decodable { let client_id: String }
        guard (res as? HTTPURLResponse)?.statusCode == 201, let a = try? JSONDecoder().decode(Answer.self, from: body) else {
            throw ClientError.refused((res as? HTTPURLResponse)?.statusCode ?? 0, String(decoding: body, as: UTF8.self))
        }
        Keychain.set("client", a.client_id)
        return a.client_id
    }

    private func form(_ path: String, _ params: [String: String]) async throws -> (Data, Int) {
        var req = URLRequest(url: Self.api.appending(path: path))
        req.httpMethod = "POST"
        req.setValue("application/x-www-form-urlencoded", forHTTPHeaderField: "Content-Type")
        var c = URLComponents()
        c.queryItems = params.map { URLQueryItem(name: $0.key, value: $0.value) }
        req.httpBody = Data((c.percentEncodedQuery ?? "").replacingOccurrences(of: "+", with: "%2B").utf8)
        let (body, res) = try await URLSession.shared.data(for: req)
        return (body, (res as? HTTPURLResponse)?.statusCode ?? 0)
    }

    private static func random() -> String {
        var b = [UInt8](repeating: 0, count: 32)
        _ = SecRandomCopyBytes(kSecRandomDefault, b.count, &b)
        return Data(b).base64URL
    }
}

/// The sign-in page, in the browser the user is signed in to Archive with.
@MainActor final class Web: NSObject, ASWebAuthenticationPresentationContextProviding {
    static func open(_ url: URL) async throws -> URL {
        let web = Web()
        return try await withCheckedThrowingContinuation { done in
            let s = ASWebAuthenticationSession(url: url, callbackURLScheme: Client.scheme) { back, err in
                if let back { done.resume(returning: back) } else { done.resume(throwing: err ?? ClientError.signedOut) }
            }
            s.presentationContextProvider = web
            s.start()
        }
    }

    func presentationAnchor(for _: ASWebAuthenticationSession) -> ASPresentationAnchor {
        UIApplication.shared.connectedScenes.compactMap { ($0 as? UIWindowScene)?.keyWindow }.first ?? ASPresentationAnchor()
    }
}

/// Secrets the app keeps after the first unlock, so background syncs can sign in while the phone is locked.
enum Keychain {
    static func get(_ key: String) -> String? {
        var out: AnyObject?
        let q: [CFString: Any] = [kSecClass: kSecClassGenericPassword, kSecAttrService: Client.scheme, kSecAttrAccount: key, kSecReturnData: true]
        guard SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess, let d = out as? Data else { return nil }
        return String(decoding: d, as: UTF8.self)
    }

    static func set(_ key: String, _ value: String?) {
        let q: [CFString: Any] = [kSecClass: kSecClassGenericPassword, kSecAttrService: Client.scheme, kSecAttrAccount: key]
        SecItemDelete(q as CFDictionary)
        guard let value else { return }
        var add = q
        add[kSecValueData] = Data(value.utf8)
        add[kSecAttrAccessible] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
        SecItemAdd(add as CFDictionary, nil)
    }
}

extension Data {
    var base64URL: String {
        base64EncodedString().replacingOccurrences(of: "+", with: "-").replacingOccurrences(of: "/", with: "_").replacingOccurrences(of: "=", with: "")
    }
}
