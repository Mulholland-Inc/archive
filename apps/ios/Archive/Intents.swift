import AppIntents
import Foundation

/// Archive in Siri, Shortcuts and Spotlight: ask the workspace's agent, save a file, or sync now.
struct AskArchive: AppIntent {
    static let title: LocalizedStringResource = "Ask Archive"
    static let description = IntentDescription("Asks your Archive's agent a question and says its answer.")

    @Parameter(title: "Question", requestValueDialog: "What do you want to ask Archive?")
    var question: String

    func perform() async throws -> some IntentResult & ReturnsValue<String> & ProvidesDialog {
        let answer = try await Client.shared.ask(question)
        return .result(value: answer, dialog: "\(answer)")
    }
}

struct SaveToArchive: AppIntent {
    static let title: LocalizedStringResource = "Save to Archive"
    static let description = IntentDescription("Keeps a file, like a PDF or a photo, in your Archive files.")

    @Parameter(title: "File")
    var file: IntentFile

    func perform() async throws -> some IntentResult & ProvidesDialog {
        let tmp = FileManager.default.temporaryDirectory.appending(path: UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: tmp) }
        if let url = file.fileURL, url.startAccessingSecurityScopedResource() {
            defer { url.stopAccessingSecurityScopedResource() }
            try FileManager.default.copyItem(at: url, to: tmp)
        } else {
            try file.data.write(to: tmp)
        }
        try await Client.shared.save(file.filename, file: tmp)
        return .result(dialog: "Saved \(file.filename) to Archive.")
    }
}

struct SyncArchive: AppIntent {
    static let title: LocalizedStringResource = "Sync Archive"
    static let description = IntentDescription("Sends what changed on this iPhone to Archive now.")

    func perform() async throws -> some IntentResult & ProvidesDialog {
        await Syncer.shared.all()
        return .result(dialog: "Archive is up to date.")
    }
}

struct ArchiveShortcuts: AppShortcutsProvider {
    static var appShortcuts: [AppShortcut] {
        AppShortcut(intent: AskArchive(), phrases: ["Ask \(.applicationName)", "Ask \(.applicationName) a question"],
                    shortTitle: "Ask Archive", systemImageName: "bubble.left.and.text.bubble.right")
        AppShortcut(intent: SaveToArchive(), phrases: ["Save to \(.applicationName)"], shortTitle: "Save to Archive", systemImageName: "tray.and.arrow.down")
        AppShortcut(intent: SyncArchive(), phrases: ["Sync \(.applicationName)"], shortTitle: "Sync", systemImageName: "arrow.triangle.2.circlepath")
    }
}
