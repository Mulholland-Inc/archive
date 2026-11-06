import Foundation

/// What the app, its widget and its Share extension share: the App Group's defaults, where the app keeps how much it sent and when.
enum Group {
    static let id = "group.dev.archive.app"
    static let defaults = UserDefaults(suiteName: id) ?? .standard
    static var sent: [String: Int] { defaults.dictionary(forKey: "sent") as? [String: Int] ?? [:] }
    static var at: [String: Date] { defaults.dictionary(forKey: "at") as? [String: Date] ?? [:] }
}
