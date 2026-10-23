import SwiftUI
import Observation

enum Stage: Int {
    case review, evidence, ready, pending, applied

    var label: String {
        switch self {
        case .review: "In review"
        case .evidence: "Needs evidence"
        case .ready: "Ready to apply"
        case .pending: "Pending approval"
        case .applied: "Applied"
        }
    }

    var tone: Tone {
        switch self {
        case .review: .ai
        case .evidence: .warning
        case .ready: .highlight
        case .pending: .pending
        case .applied: .mint
        }
    }
}

enum RecordSection: String, CaseIterable, Hashable {
    case review, evidence, record, history

    var title: String {
        switch self {
        case .review: "AI Review"
        case .evidence: "Evidence"
        case .record: "Record"
        case .history: "History"
        }
    }
}

struct Field: Identifiable {
    let symbol: String
    let name: String
    let value: String
    var confidence: Int
    var id: String { name }
    var confident: Bool { confidence >= 90 }
}

struct Source: Identifiable {
    let symbol: String
    let title: String
    var subtitle: String
    var verified: Bool
    var id: String { title }
}

@MainActor
@Observable
final class RecordModel {
    var stage: Stage = .review
    var section: RecordSection = .review
    var action: ActionState = .idle
    var checklistOpen = false
    var fields: [Field] = []
    var sources: [Source] = []
    var events: [Timeline.Event] = []

    init() { reset() }

    var confidentFields: Int { fields.filter(\.confident).count }
    var verifiedSources: Int { sources.filter(\.verified).count }

    var checks: [Checklist.Item] {
        [
            .init(label: "Vendor matches a known company", done: true),
            .init(label: "Total equals the sum of lines", done: true),
            .init(label: "VAT number is valid", done: confidentFields == fields.count),
            .init(label: "Currency is EUR", done: true),
            .init(label: "Paid from a linked account", done: true),
            .init(label: "No duplicate in Finance", done: true),
            .init(label: "Date within the fiscal year", done: true),
            .init(label: "Purchase order attached", done: verifiedSources == sources.count)
        ]
    }

    var passedChecks: Int { checks.filter(\.done).count }

    func reset() {
        withAnimation(Motion.spring) {
            stage = .review
            section = .review
            action = .idle
            checklistOpen = false
            fields = [
                Field(symbol: "building.2", name: "Vendor", value: "Hetzner Online GmbH", confidence: 99),
                Field(symbol: "eurosign", name: "Total", value: "€48.90 including VAT", confidence: 97),
                Field(symbol: "calendar", name: "Invoice date", value: "October 1, 2026", confidence: 96),
                Field(symbol: "number", name: "VAT number", value: "DE812871812", confidence: 64)
            ]
            sources = [
                Source(symbol: "doc.text", title: "Invoice PDF", subtitle: "billing@hetzner.com · Oct 1, 2026", verified: true),
                Source(symbol: "envelope", title: "Email thread", subtitle: "Oct 1 to Oct 3 · 3 messages", verified: true),
                Source(symbol: "creditcard", title: "Bank transaction", subtitle: "ING · Oct 4, 2026", verified: true),
                Source(symbol: "doc.viewfinder", title: "Purchase order", subtitle: "Required by Finance policy", verified: false)
            ]
            events = [
                .init(date: "Oct 20", symbol: "sparkles", tone: .ai, title: "AI review completed", subtitle: "97% confidence"),
                .init(date: "Oct 19", symbol: "folder", tone: .muted, title: "Source documents collected", subtitle: "4 documents")
            ]
        }
    }

    func perform() {
        guard action == .idle else { return }
        let current = section
        Task {
            withAnimation(Motion.bouncy) { action = .loading }
            try? await Task.sleep(for: .milliseconds(1100))
            withAnimation(Motion.spring) {
                action = .done
                complete(current)
            }
            try? await Task.sleep(for: .milliseconds(900))
            withAnimation(Motion.spring) {
                action = .idle
                advance(from: current)
            }
        }
    }

    private func complete(_ section: RecordSection) {
        switch section {
        case .review:
            if let i = fields.firstIndex(where: { !$0.confident }) { fields[i].confidence = 100 }
        case .evidence:
            if let i = sources.firstIndex(where: { !$0.verified }) {
                sources[i].verified = true
                sources[i].subtitle = "Uploaded by Mila Ahn · just now"
            }
        case .record:
            events.insert(.init(date: "Oct 21", symbol: "paperplane", tone: .mint, title: "Change proposed", subtitle: "Intake agent · CHG-2291047"), at: 0)
            events.insert(.init(date: "Oct 22", symbol: "hourglass", tone: .pending, title: "Owner approval", subtitle: "In progress"), at: 0)
        case .history:
            if let i = events.firstIndex(where: { $0.title == "Owner approval" }) {
                events[i] = .init(date: "Oct 23", symbol: "checkmark", tone: .success, title: "Owner approval", subtitle: "Approved by Mila Ahn")
            }
        }
    }

    private func advance(from section: RecordSection) {
        switch section {
        case .review:
            stage = .evidence
            self.section = .evidence
        case .evidence:
            stage = .ready
            self.section = .record
        case .record:
            stage = .pending
            self.section = .history
        case .history:
            stage = .applied
        }
    }
}
