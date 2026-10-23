import SwiftUI

struct KitScreen: View {
    @Environment(\.dismiss) private var dismiss
    @State private var tab = "record"
    @State private var open = true
    @State private var state: ActionState = .idle

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: Space.s3) {
                HStack {
                    Text("Components").font(.uiHero).tracking(Tracking.tight)
                    Spacer()
                    CircleButton(symbol: "xmark", label: "Close") { dismiss() }
                }
                .padding(.bottom, Space.s2)

                section("Navigation") {
                    NavBar(title: "Invoice · Hetzner Online", subtitle: "REC-48291") {
                        CircleButton(symbol: "ellipsis", label: "More")
                    }
                    SegmentedTabs(selection: $tab, items: [
                        .init(value: "review", label: "AI Review"),
                        .init(value: "evidence", label: "Evidence"),
                        .init(value: "record", label: "Record")
                    ])
                }

                section("Chips and tags") {
                    ChipRail {
                        Chip(tone: .highlight, label: "Ready to apply")
                        Chip(tone: .warning, label: "Needs evidence")
                        Chip(tone: .pending, label: "Pending approval")
                        Chip(tone: .ai, label: "In review")
                        Chip(person: "Mila Ahn", label: "Mila Ahn")
                        Chip(symbol: "calendar")
                    }
                    HStack(spacing: Space.s2) {
                        Tag(label: "Verified")
                        Tag(label: "Missing", tone: .warning)
                        Tag(label: "AI-generated", tone: .ai, symbol: "sparkles")
                        Tag(label: "Ready", dot: true)
                    }
                }

                section("Glyphs and buttons") {
                    HStack(spacing: Space.s2) {
                        Glyph(symbol: "checkmark", tone: .success)
                        Glyph(symbol: "hourglass", tone: .pending)
                        Glyph(symbol: "paperplane", tone: .mint)
                        Glyph(symbol: "sparkles", tone: .ai)
                        Glyph(symbol: "folder")
                        Glyph(symbol: "doc.viewfinder", tone: .warning, shape: .square, scale: .small)
                        Avatar(name: "Mila Ahn")
                    }
                    HStack(spacing: Space.s2) {
                        CircleButton(symbol: "chevron.left", label: "Back")
                        CircleButton(symbol: "square.and.pencil", style: .muted, label: "Edit")
                        CircleButton(symbol: "eye", style: .muted, scale: .large, label: "Preview")
                    }
                    PrimaryButton(symbol: "paperplane", title: "Apply to Finance", state: state) {
                        Task {
                            withAnimation(Motion.bouncy) { state = .loading }
                            try? await Task.sleep(for: .milliseconds(1100))
                            withAnimation(Motion.bouncy) { state = .done }
                            try? await Task.sleep(for: .milliseconds(1400))
                            withAnimation(Motion.bouncy) { state = .idle }
                        }
                    }
                }

                Disclosure(symbol: "checkmark", title: "Validation checklist", subtitle: "8 of 8 checks passed", isOpen: $open) {
                    SegmentedProgress(total: 8, value: 8)
                } content: {
                    Checklist(items: [.init(label: "Vendor matches a known company", done: true), .init(label: "Purchase order attached", done: false)])
                }

                Card(title: "Summary", subtitle: "Card, Panel and Citation") {
                    CircleButton(symbol: "square.and.pencil", style: .muted, label: "Edit")
                } content: {
                    Panel {
                        Text("Hetzner billed Mulholland Inc €48.90 for October.").font(.ui)
                        Citation(label: "Invoice PDF · Oct 1")
                    }
                    ListItem(symbol: "doc.text", title: "Invoice PDF", subtitle: "billing@hetzner.com") { Tag(label: "Verified") }
                    ListItem(symbol: "doc.viewfinder", tone: .warning, title: "Purchase order", subtitle: "Required by Finance policy") { Tag(label: "Missing", tone: .warning) }
                    StatGrid(items: [.init(label: "Documents", value: "3 of 4"), .init(label: "Verified", value: "3"), .init(label: "Fields", value: "12")])
                }

                ActionRow(symbol: "camera", title: "Scan a paper document", subtitle: "Archive reads it and links it to the right record")

                StatusCard(label: "Approval status", title: "Pending approval", subtitle: "Proposed · Oct 21, 10:42 AM", facts: [.init(label: "Expected response", value: "Within 2 days"), .init(label: "Next action", value: "None")])

                Card(title: "Timeline") {
                    Timeline(events: [
                        .init(date: "Oct 22", symbol: "hourglass", tone: .pending, title: "Owner approval", subtitle: "In progress"),
                        .init(date: "Oct 21", symbol: "paperplane", tone: .mint, title: "Change proposed", subtitle: "CHG-2291047"),
                        .init(date: "Oct 19", symbol: "folder", tone: .muted, title: "Documents collected", subtitle: "4 documents")
                    ])
                }

                ActionBar {
                    CircleButton(symbol: "eye", style: .muted, scale: .large, label: "Preview")
                    PrimaryButton(symbol: "paperplane", title: "Apply to Finance")
                }
            }
            .padding(Space.gutter)
        }
        .background(Palette.background.ignoresSafeArea())
    }

    private func section<Content: View>(_ title: String, @ViewBuilder content: () -> Content) -> some View {
        Card(title: title) {
            VStack(alignment: .leading, spacing: Space.s3) { content() }
        }
    }
}
