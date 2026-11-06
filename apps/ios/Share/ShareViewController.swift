import UIKit
import UniformTypeIdentifiers

/// Archive in the Share sheet: what the user shares from another app, like a PDF, a photo or a link, goes to their Archive files.
final class ShareViewController: UIViewController {
    private let label = UILabel()

    override func viewDidLoad() {
        super.viewDidLoad()
        view.backgroundColor = .systemBackground
        label.font = .preferredFont(forTextStyle: .headline)
        label.textAlignment = .center
        label.numberOfLines = 0
        label.text = "Saving to Archive…"
        label.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(label)
        NSLayoutConstraint.activate([label.centerXAnchor.constraint(equalTo: view.centerXAnchor), label.centerYAnchor.constraint(equalTo: view.centerYAnchor),
                                     label.widthAnchor.constraint(lessThanOrEqualTo: view.widthAnchor, constant: -48)])
        Task { await save() }
    }

    private func save() async {
        let providers = (extensionContext?.inputItems as? [NSExtensionItem] ?? []).flatMap { $0.attachments ?? [] }
        var saved = 0
        var failed: Error?
        for p in providers {
            do {
                if try await save(p) { saved += 1 }
            } catch {
                failed = error
            }
        }
        if !Client.shared.signedIn {
            label.text = "Open Archive and sign in first."
        } else if let failed, saved == 0 {
            label.text = failed.localizedDescription
        } else {
            label.text = saved == 1 ? "Saved to Archive" : "Saved \(saved) to Archive"
        }
        try? await Task.sleep(for: .seconds(failed == nil ? 0.8 : 2.5))
        extensionContext?.completeRequest(returningItems: nil)
    }

    /// A file as it is; a link alone as an Internet shortcut, which opens the page again.
    private func save(_ p: NSItemProvider) async throws -> Bool {
        let tmp = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: tmp) }
        if let type = p.registeredContentTypes.first(where: { !$0.conforms(to: .url) || $0.conforms(to: .fileURL) }) {
            let name: String = try await withCheckedThrowingContinuation { done in
                _ = p.loadFileRepresentation(for: type, openInPlace: false) { url, _, err in
                    guard let url else { return done.resume(throwing: err ?? CocoaError(.fileReadUnknown)) }
                    do {
                        try FileManager.default.copyItem(at: url, to: tmp)
                        done.resume(returning: url.lastPathComponent)
                    } catch {
                        done.resume(throwing: error)
                    }
                }
            }
            try await Client.shared.save(p.suggestedName.map { $0 + "." + (URL(filePath: name).pathExtension) } ?? name, file: tmp)
            return true
        }
        if p.canLoadObject(ofClass: URL.self) {
            let url: URL = try await withCheckedThrowingContinuation { done in
                _ = p.loadObject(ofClass: URL.self) { u, err in u.map { done.resume(returning: $0) } ?? done.resume(throwing: err ?? CocoaError(.fileReadUnknown)) }
            }
            try Data("[InternetShortcut]\nURL=\(url.absoluteString)\n".utf8).write(to: tmp)
            try await Client.shared.save((url.host() ?? "Link") + ".url", file: tmp)
            return true
        }
        return false
    }
}
