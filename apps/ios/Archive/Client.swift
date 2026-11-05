import AuthenticationServices
import CryptoKit
import DeviceCheck
import Foundation
import Security
import UIKit

enum ClientError: LocalizedError {
    case signedOut
    case refused(Int, String)

    var errorDescription: String? {
        switch self {
        case .signedOut: "Sign in to Archive again to keep syncing."
        case let .refused(status, body):
            // Archive answers { error }; show its words, not the JSON.
            (try? JSONDecoder().decode([String: String].self, from: Data(body.utf8)))?["error"].map { $0.prefix(1).uppercased() + $0.dropFirst() }
                ?? (status == 0 ? body : "Archive couldn't take this (\(status)). It tries again on the next sync.")
        }
    }
}

/// The phone's way into Archive: Archive's own OAuth, the one MCP clients use, with PKCE. The phone registers itself as a client once,
/// signs in on the web app, and keeps the rotating refresh token in the Keychain, which the Share extension shares; /device/* takes the access token.
/// Where Archive asks for it, the app also proves with App Attest that it is the real app on a real iPhone, and signs each request with the key it attested.
actor Client {
    static let shared = Client()
    static let api = URL(string: Bundle.main.object(forInfoDictionaryKey: "ArchiveAPI") as! String)!
    static let scheme = "dev.archive.app"
    static let redirect = scheme + ":/oauth"

    private var access: String?
    private var until = Date.distantPast
    private var refreshing: Task<String, Error>?
    private var attestOff = false

    /// Only the app signs with its attested key; the Share extension saves files, which Archive takes unsigned.
    static let signs = Bundle.main.bundleURL.pathExtension == "app"

    nonisolated var signedIn: Bool { Keychain.get("refresh") != nil }

    #if !EXTENSION
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
    #endif

    func signOut() async {
        if let refresh = Keychain.get("refresh"), let client = Keychain.get("client") {
            _ = try? await form("oauth/revoke", ["token": refresh, "client_id": client])
        }
        Keychain.set("refresh", nil)
        access = nil
    }

    /// Lands records of a stream in Archive: [{ id, data }] or [{ id, deleted: true }]; full when they are all of it, more when another batch of it follows.
    func push(_ stream: String, _ records: [[String: Any]], full: Bool = false, more: Bool = false) async throws {
        let body = try JSONSerialization.data(withJSONObject: ["records": records, "full": full, "more": more])
        _ = try await send("POST", "device/\(stream)", body: .data(body, "application/json"))
    }

    /// Checks the sign-in still holds, so a phone Archive turned down hears of it even with nothing new to send.
    func check() async throws {
        for attempt in 0 ..< 2 {
            var req = URLRequest(url: Self.api.appending(path: "device/attest"))
            req.setValue("Bearer " + (try await bearer()), forHTTPHeaderField: "Authorization")
            req.setValue(Self.install, forHTTPHeaderField: "Archive-Device")
            req.setValue(await UIDevice.current.name, forHTTPHeaderField: "Archive-Device-Name")
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

    /// Keeps a photo's original, full size, with the photo Archive has of it already.
    func original(_ id: String, name: String, file: URL) async throws {
        _ = try await send("PUT", "device/originals", query: [.init(name: "photo", value: id), .init(name: "name", value: name)],
                           body: .file(file, "application/octet-stream"), cheap: true)
    }

    /// Keeps a file in the member's Archive files, as an upload in the web app would.
    func save(_ name: String, file: URL) async throws {
        _ = try await send("POST", "device/files", query: [.init(name: "name", value: name)], body: .file(file, "application/octet-stream"))
    }

    /// Asks the workspace's agent, as the web app's chat does, and answers with what it said last, after its last run of code.
    func ask(_ text: String) async throws -> String {
        let (lines, _) = try await send("POST", "device/ask", body: .data(try JSONSerialization.data(withJSONObject: ["text": text]), "application/json"))
        var answer = ""
        for line in lines.split(separator: UInt8(ascii: "\n")) {
            guard let e = try? JSONSerialization.jsonObject(with: Data(line)) as? [String: Any] else { continue }
            switch e["type"] as? String {
            case "text": answer += e["text"] as? String ?? ""
            case "run", "code": answer = ""
            case "error": throw ClientError.refused(500, e["error"] as? String ?? "The agent failed")
            default: break
            }
        }
        return answer.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    enum Body {
        case data(Data, String)
        case file(URL, String)
    }

    /// A request of /device/*: the access token, which install of the app it is, and, where Archive wants it, an assertion of the attested key
    /// over the method, the path and the body's SHA-256. Cheap ones wait for Wi-Fi, and keep off Low Data Mode.
    private func send(_ method: String, _ path: String, query: [URLQueryItem] = [], body: Body, cheap: Bool = false) async throws -> (Data, Int) {
        var url = URLComponents(url: Self.api.appending(path: path), resolvingAgainstBaseURL: false)!
        if !query.isEmpty { url.queryItems = query }
        var req = URLRequest(url: url.url!)
        req.httpMethod = method
        req.allowsExpensiveNetworkAccess = !cheap
        req.allowsConstrainedNetworkAccess = !cheap
        let sum: String
        switch body {
        case let .data(d, type):
            req.setValue(type, forHTTPHeaderField: "Content-Type")
            sum = SHA256.hash(data: d).hex
        case let .file(f, type):
            req.setValue(type, forHTTPHeaderField: "Content-Type")
            sum = try Self.sha256(f)
        }
        req.setValue(sum, forHTTPHeaderField: "Archive-Body-SHA256")
        req.setValue(Self.install, forHTTPHeaderField: "Archive-Device")
        req.setValue(await UIDevice.current.name, forHTTPHeaderField: "Archive-Device-Name")
        for attempt in 0 ..< 3 {
            req.setValue("Bearer " + (try await bearer()), forHTTPHeaderField: "Authorization")
            if let key = try await attested() {
                let client = Data(SHA256.hash(data: Data("\(method) \(req.url!.path)\n\(sum)".utf8)))
                req.setValue(try await DCAppAttestService.shared.generateAssertion(key, clientDataHash: client).base64EncodedString(), forHTTPHeaderField: "Archive-Assertion")
            }
            let (got, res): (Data, URLResponse)
            switch body {
            case let .data(d, _): (got, res) = try await URLSession.shared.upload(for: req, from: d)
            case let .file(f, _): (got, res) = try await URLSession.shared.upload(for: req, fromFile: f)
            }
            let status = (res as? HTTPURLResponse)?.statusCode ?? 0
            let said = String(decoding: got, as: UTF8.self)
            if status == 401, attempt < 2 {
                // An assertion overtaken by another of the same key is only retried; any other App Attest refusal attests a new key.
                if said.contains("used before") { continue }
                if said.contains("App Attest") {
                    Keychain.set("attestKey", nil)
                    attestOff = false
                } else {
                    access = nil
                }
                continue
            }
            guard status == 200 else { throw ClientError.refused(status, said) }
            return (got, status)
        }
        throw ClientError.signedOut
    }

    /// The App Attest key this install signs with, attested on first use; nil where Archive does not ask for it, as in the Simulator.
    private func attested() async throws -> String? {
        guard Self.signs else { return nil }
        if let key = Keychain.get("attestKey") { return key }
        if attestOff { return nil }
        var req = URLRequest(url: Self.api.appending(path: "device/attest"))
        req.setValue("Bearer " + (try await bearer()), forHTTPHeaderField: "Authorization")
        req.setValue(Self.install, forHTTPHeaderField: "Archive-Device")
        req.setValue(await UIDevice.current.name, forHTTPHeaderField: "Archive-Device-Name")
        struct Challenge: Decodable { let challenge: String, required: Bool }
        let (body, res) = try await URLSession.shared.data(for: req)
        guard (res as? HTTPURLResponse)?.statusCode == 200, let c = try? JSONDecoder().decode(Challenge.self, from: body) else {
            throw ClientError.refused((res as? HTTPURLResponse)?.statusCode ?? 0, String(decoding: body, as: UTF8.self))
        }
        guard c.required else {
            attestOff = true
            return nil
        }
        let service = DCAppAttestService.shared
        guard service.isSupported else { throw ClientError.refused(0, "This iPhone can't prove to Archive that it runs the real app (App Attest)") }
        let key = try await service.generateKey()
        let attestation = try await service.attestKey(key, clientDataHash: Data(SHA256.hash(data: Data(c.challenge.utf8))))
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        req.httpBody = try JSONSerialization.data(withJSONObject: ["key": key, "attestation": attestation.base64EncodedString(), "challenge": c.challenge])
        let (said, posted) = try await URLSession.shared.data(for: req)
        guard (posted as? HTTPURLResponse)?.statusCode == 200 else {
            throw ClientError.refused((posted as? HTTPURLResponse)?.statusCode ?? 0, String(decoding: said, as: UTF8.self))
        }
        Keychain.set("attestKey", key)
        return key
    }

    /// This install's id, Archive's account for the phone's connection; kept in the Keychain, so the Share extension sends the same.
    static var install: String {
        if let id = Keychain.get("install") { return id }
        let id = UUID().uuidString.lowercased()
        Keychain.set("install", id)
        return id
    }

    private static func sha256(_ file: URL) throws -> String {
        let h = try FileHandle(forReadingFrom: file)
        defer { try? h.close() }
        var sum = SHA256()
        while let chunk = try h.read(upToCount: 1 << 20), !chunk.isEmpty { sum.update(data: chunk) }
        return sum.finalize().hex
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
        // A refresh Archive turns down, or a client it no longer knows, means signing in again.
        if status == 400 || status == 401, params["grant_type"] == "refresh_token" {
            Keychain.set("refresh", nil)
            if status == 401 { Keychain.set("client", nil) }
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
        let name = await UIDevice.current.model
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

#if !EXTENSION
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
#endif

/// Secrets the app keeps after the first unlock, so background syncs can sign in while the phone is locked; in the access group
/// the Share extension has too, where the build has it, as signed builds do.
enum Keychain {
    static let group = Bundle.main.object(forInfoDictionaryKey: "KeychainGroup") as? String

    private static func query(_ key: String, grouped: Bool) -> [CFString: Any] {
        var q: [CFString: Any] = [kSecClass: kSecClassGenericPassword, kSecAttrService: Client.scheme, kSecAttrAccount: key]
        if grouped, let group, !group.hasPrefix("$") { q[kSecAttrAccessGroup] = group }
        return q
    }

    static func get(_ key: String) -> String? {
        for grouped in [true, false] {
            var out: AnyObject?
            var q = query(key, grouped: grouped)
            q[kSecReturnData] = true
            if SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess, let d = out as? Data { return String(decoding: d, as: UTF8.self) }
        }
        return nil
    }

    static func set(_ key: String, _ value: String?) {
        for grouped in [true, false] { SecItemDelete(query(key, grouped: grouped) as CFDictionary) }
        guard let value else { return }
        for grouped in [true, false] {
            var add = query(key, grouped: grouped)
            add[kSecValueData] = Data(value.utf8)
            add[kSecAttrAccessible] = kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly
            if SecItemAdd(add as CFDictionary, nil) != errSecMissingEntitlement { return }
        }
    }
}

extension SHA256.Digest {
    var hex: String { map { String(format: "%02x", $0) }.joined() }
}

extension Data {
    var base64URL: String {
        base64EncodedString().replacingOccurrences(of: "+", with: "-").replacingOccurrences(of: "/", with: "_").replacingOccurrences(of: "=", with: "")
    }
}
