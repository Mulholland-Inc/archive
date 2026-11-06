import SwiftUI
import WidgetKit

/// How much of the iPhone is in Archive, and when it last synced, on the Home Screen and the Lock Screen.
@main
struct ArchiveWidgets: WidgetBundle {
    var body: some Widget { ArchiveWidget() }
}

struct ArchiveWidget: Widget {
    var body: some WidgetConfiguration {
        StaticConfiguration(kind: "archive", provider: Provider()) { e in
            Card(entry: e).containerBackground(.fill.tertiary, for: .widget)
        }
        .configurationDisplayName("Archive")
        .description("When your iPhone last synced with Archive, and how much it has sent.")
        .supportedFamilies([.systemSmall, .accessoryRectangular])
    }
}

struct Entry: TimelineEntry {
    let date: Date
    let at: Date?
    let sent: Int
}

struct Provider: TimelineProvider {
    func placeholder(in _: Context) -> Entry { Entry(date: .now, at: .now, sent: 12480) }
    func getSnapshot(in c: Context, completion: @escaping (Entry) -> Void) { completion(c.isPreview ? placeholder(in: c) : now()) }
    func getTimeline(in _: Context, completion: @escaping (Timeline<Entry>) -> Void) {
        completion(Timeline(entries: [now()], policy: .after(.now + 3600)))
    }

    private func now() -> Entry { Entry(date: .now, at: Group.at.values.max(), sent: Group.sent.values.reduce(0, +)) }
}

struct Card: View {
    let entry: Entry

    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            Label("Archive", systemImage: "archivebox").font(.caption.weight(.semibold))
            Spacer(minLength: 0)
            Text(entry.sent.formatted()).font(.title2.weight(.semibold)).monospacedDigit()
            Text("items kept").font(.caption).foregroundStyle(.secondary)
            if let at = entry.at {
                Text(at, style: .relative).font(.caption2).foregroundStyle(.secondary)
            } else {
                Text("Not synced yet").font(.caption2).foregroundStyle(.secondary)
            }
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}
