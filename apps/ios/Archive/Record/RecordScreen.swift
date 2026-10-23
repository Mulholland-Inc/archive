import SwiftUI

struct RecordScreen: View {
    @State private var model = RecordModel()
    @State private var showKit = false

    var body: some View {
        VStack(spacing: 0) {
            header
            TabView(selection: $model.section) {
                ForEach(RecordSection.allCases, id: \.self) { section in
                    ScrollView {
                        VStack(spacing: Space.s3) { content(for: section) }
                            .padding(.horizontal, Space.gutter)
                            .padding(.top, Space.s1)
                            .padding(.bottom, Size.actionBar + Space.s12)
                    }
                    .scrollIndicators(.hidden)
                    .tag(section)
                }
            }
            .tabViewStyle(.page(indexDisplayMode: .never))
        }
        .background(Palette.background.ignoresSafeArea())
        .overlay(alignment: .bottom) {
            actionBar
                .padding(.horizontal, Space.gutter)
                .padding(.bottom, Space.s1)
        }
        .animation(Motion.spring, value: model.section)
        .sensoryFeedback(.selection, trigger: model.section)
        .sheet(isPresented: $showKit) { KitScreen() }
    }

    private var header: some View {
        VStack(spacing: Space.s3) {
            NavBar(title: "Invoice · Hetzner Online", subtitle: "REC-48291 · Mila Ahn") {
                Menu {
                    Button("Show components", systemImage: "square.grid.2x2") { showKit = true }
                    Button("Reset demo", systemImage: "arrow.counterclockwise") { model.reset() }
                } label: {
                    CircleLabel(symbol: "ellipsis")
                }
                .accessibilityLabel("More")
            }
            ChipRail {
                Chip(tone: model.stage.tone, label: model.stage.label)
                    .animation(Motion.spring, value: model.stage)
                Chip(person: "Mila Ahn", label: "Mila Ahn")
                Chip(symbol: "envelope", label: "Gmail")
                Chip(symbol: "calendar", label: "Oct 21")
            }
            SegmentedTabs(
                selection: $model.section,
                items: RecordSection.allCases.map { .init(value: $0, label: $0.title) }
            )
        }
        .padding(.horizontal, Space.gutter)
        .padding(.top, Space.s1)
        .padding(.bottom, Space.s3)
    }

    @ViewBuilder
    private func content(for section: RecordSection) -> some View {
        switch section {
        case .review: review
        case .evidence: evidence
        case .record: record
        case .history: history
        }
    }

    private var review: some View {
        Group {
            Disclosure(
                symbol: "sparkles",
                tone: .ai,
                title: "Extraction confidence",
                subtitle: "\(model.confidentFields) of \(model.fields.count) fields above threshold",
                isOpen: $model.checklistOpen
            ) {
                SegmentedProgress(total: 12, value: 8 + model.confidentFields)
            } content: {
                Checklist(items: model.fields.map { .init(label: "\($0.name) at \($0.confidence)%", done: $0.confident) })
            }
            Card(title: "Fields") {
                CircleButton(symbol: "arrow.triangle.2.circlepath", style: .muted, label: "Run again")
            } content: {
                VStack(spacing: Space.s2) {
                    ForEach(model.fields) { field in
                        ListItem(symbol: field.symbol, tone: field.confident ? nil : .warning, title: field.name, subtitle: field.value) {
                            Tag(label: "\(field.confidence)%", tone: field.confident ? .success : .warning)
                        }
                    }
                }
            }
        }
    }

    private var evidence: some View {
        Group {
            Card(title: "Source evidence", subtitle: "Documents behind this record") {
                CircleButton(symbol: "plus", style: .muted, label: "Add document")
            } content: {
                VStack(spacing: Space.s2) {
                    ForEach(model.sources) { source in
                        ListItem(symbol: source.symbol, tone: source.verified ? nil : .warning, title: source.title, subtitle: source.subtitle) {
                            Tag(label: source.verified ? "Verified" : "Missing", tone: source.verified ? .success : .warning)
                        }
                    }
                }
                StatGrid(items: [
                    .init(label: "Documents", value: "\(model.verifiedSources) of \(model.sources.count)"),
                    .init(label: "Verified", value: "\(model.verifiedSources)"),
                    .init(label: "Fields", value: "\(model.fields.count * 3)")
                ])
            }
            ActionRow(symbol: "camera", title: "Scan a paper document", subtitle: "Archive reads it and links it to the right record")
        }
    }

    private var record: some View {
        Group {
            Disclosure(
                symbol: "checkmark",
                tone: model.passedChecks == 8 ? .success : .warning,
                title: "Validation checklist",
                subtitle: "\(model.passedChecks) of 8 checks passed",
                isOpen: $model.checklistOpen
            ) {
                SegmentedProgress(total: 8, value: model.passedChecks)
            } content: {
                Checklist(items: model.checks)
            }
            Card(title: "Summary") {
                CircleButton(symbol: "square.and.pencil", style: .muted, label: "Edit summary")
                CircleButton(symbol: "arrow.triangle.2.circlepath", style: .muted, label: "Regenerate")
            } content: {
                Panel {
                    HStack {
                        Tag(label: "AI-generated", tone: .ai, symbol: "sparkles")
                        Spacer()
                        Text("Draft 2").font(.uiSmall).foregroundStyle(Palette.muted)
                    }
                    Text("Hetzner billed Mulholland Inc €48.90 for October, covering one dedicated server and a storage box, with 19% German VAT reverse-charged.")
                        .font(.ui)
                        .lineSpacing(Space.s1)
                    Citation(label: "Invoice PDF · Oct 1")
                    Text("The amount left the ING business account on October 4 and matches the invoice number exactly.")
                        .font(.ui)
                        .lineSpacing(Space.s1)
                    Citation(label: "Bank transaction")
                }
            }
            Card(title: "Final check") {
                Tag(label: model.passedChecks == 8 ? "Ready" : "Blocked", tone: model.passedChecks == 8 ? .success : .warning, dot: true)
            } content: {
                StatGrid(items: [
                    .init(label: "Space", value: "Finance"),
                    .init(label: "Documents", value: "\(model.verifiedSources) attached")
                ])
            }
        }
    }

    private var history: some View {
        Group {
            if model.stage == .applied {
                StatusCard(
                    label: "Approval status",
                    symbol: "checkmark.seal",
                    title: "Applied to Finance",
                    subtitle: "Approved · Oct 23, 9:14 AM",
                    facts: [.init(label: "Record", value: "REC-48291"), .init(label: "Next action", value: "Nothing left to do")],
                    complete: true
                )
                .transition(.scale(scale: Motion.pressScale).combined(with: .opacity))
            } else {
                StatusCard(
                    label: "Approval status",
                    title: model.stage == .pending ? "Pending approval" : "Not proposed yet",
                    subtitle: model.stage == .pending ? "Proposed · Oct 21, 10:42 AM" : "Apply the record to send it for approval",
                    facts: [.init(label: "Expected response", value: "Within 2 days"), .init(label: "Next action", value: "No action required")]
                )
                .transition(.scale(scale: Motion.pressScale).combined(with: .opacity))
            }
            Card(title: "Timeline") {
                Text("Day 4").font(.uiMedium)
            } content: {
                Timeline(events: model.events)
            }
        }
    }

    private var actionBar: some View {
        ActionBar {
            switch model.section {
            case .review:
                CircleButton(symbol: "bubble.left", style: .muted, scale: .large, label: "Ask about this record")
                PrimaryButton(symbol: "checkmark", title: "Accept all fields", state: model.action, action: model.perform)
            case .evidence:
                CircleButton(symbol: "camera", style: .muted, scale: .large, label: "Scan")
                PrimaryButton(symbol: "envelope", title: "Request document", state: model.action, action: model.perform)
            case .record:
                CircleButton(symbol: "eye", style: .muted, scale: .large, label: "Preview")
                PrimaryButton(symbol: "paperplane", title: "Apply to Finance", state: model.action, action: model.perform)
            case .history:
                CircleButton(symbol: "bubble.left", style: .muted, scale: .large, label: "Message the owner")
                PrimaryButton(symbol: "arrow.triangle.2.circlepath", title: "Check status now", state: model.action, action: model.perform)
            }
        }
    }
}
