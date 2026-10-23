import SwiftUI

struct Glyph: View {
    enum Shape { case circle, square }
    enum Scale { case small, regular, large }

    let symbol: String
    var tone: Tone = .muted
    var shape: Shape = .circle
    var scale: Scale = .regular

    var body: some View {
        let size = scale == .small ? Size.tileSmall : scale == .large ? Size.tileLarge : Size.tile
        Image(systemName: symbol)
            .font(.uiIcon)
            .foregroundStyle(tone.glyphInk)
            .frame(width: size, height: size)
            .background {
                if shape == .circle {
                    Circle().fill(tone.fill)
                } else {
                    RoundedRectangle(cornerRadius: Radius.tile, style: .continuous).fill(tone.fill)
                }
            }
    }
}

struct Card<Actions: View, Content: View>: View {
    var title: String? = nil
    var subtitle: String? = nil
    @ViewBuilder var actions: Actions
    @ViewBuilder var content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: Space.s4) {
            if let title {
                HStack(spacing: Space.s3) {
                    VStack(alignment: .leading, spacing: Space.half) {
                        Text(title).font(.uiTitle).tracking(Tracking.tight)
                        if let subtitle {
                            Text(subtitle).font(.uiSmall).foregroundStyle(Palette.muted)
                        }
                    }
                    Spacer(minLength: 0)
                    HStack(spacing: Space.s2) { actions }
                }
                .frame(minHeight: Size.control)
            }
            content
        }
        .foregroundStyle(Palette.foreground)
        .padding(Space.s5)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: Radius.card, style: .continuous).fill(Palette.card))
    }
}

extension Card where Actions == EmptyView {
    init(title: String? = nil, subtitle: String? = nil, @ViewBuilder content: () -> Content) {
        self.init(title: title, subtitle: subtitle, actions: { EmptyView() }, content: content)
    }
}

struct Panel<Content: View>: View {
    @ViewBuilder var content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: Space.s3) { content }
            .padding(Space.s4)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(RoundedRectangle(cornerRadius: Radius.panel, style: .continuous).fill(Palette.panel))
    }
}

struct SegmentedProgress: View {
    let total: Int
    let value: Int
    @State private var shown = 0

    var body: some View {
        HStack(spacing: Space.s1h) {
            ForEach(0..<total, id: \.self) { index in
                Capsule()
                    .fill(Palette.panel)
                    .overlay(alignment: .leading) {
                        GeometryReader { proxy in
                            Capsule()
                                .fill(Palette.success)
                                .frame(width: index < shown ? proxy.size.width : 0)
                        }
                    }
                    .clipShape(Capsule())
                    .frame(height: Size.progress)
                    .animation(Motion.fill.delay(Double(index) * Motion.stagger), value: shown)
            }
        }
        .onAppear { shown = value }
        .onChange(of: value) { _, new in shown = new }
        .accessibilityElement()
        .accessibilityValue("\(value) of \(total)")
    }
}

struct Disclosure<Footer: View, Content: View>: View {
    let symbol: String
    var tone: Tone = .success
    let title: String
    let subtitle: String
    @Binding var isOpen: Bool
    @ViewBuilder var footer: Footer
    @ViewBuilder var content: Content

    var body: some View {
        VStack(alignment: .leading, spacing: Space.s3) {
            Button {
                withAnimation(Motion.spring) { isOpen.toggle() }
            } label: {
                HStack(spacing: Space.s3) {
                    Glyph(symbol: symbol, tone: tone)
                    VStack(alignment: .leading, spacing: Space.half) {
                        Text(title).font(.uiMedium)
                        Text(subtitle).font(.uiSmall).foregroundStyle(Palette.muted).contentTransition(.numericText())
                    }
                    Spacer(minLength: 0)
                    Image(systemName: "chevron.down")
                        .font(.uiIconSmall)
                        .foregroundStyle(Palette.muted)
                        .rotationEffect(.degrees(isOpen ? 180 : 0))
                }
                .contentShape(Rectangle())
            }
            .buttonStyle(PressStyle(scale: Motion.pressScaleSubtle))
            footer
            if isOpen {
                content.transition(.opacity.combined(with: .offset(y: -Space.s2)))
            }
        }
        .foregroundStyle(Palette.foreground)
        .padding(Space.s4)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(RoundedRectangle(cornerRadius: Radius.card, style: .continuous).fill(Palette.card))
        .sensoryFeedback(.selection, trigger: isOpen)
    }
}

struct Checklist: View {
    struct Item: Identifiable {
        let label: String
        let done: Bool
        var id: String { label }
    }

    let items: [Item]

    var body: some View {
        VStack(alignment: .leading, spacing: Space.s2) {
            ForEach(items) { item in
                HStack(spacing: Space.s2h) {
                    Image(systemName: item.done ? "checkmark" : "xmark")
                        .font(.uiIconSmall)
                        .foregroundStyle(item.done ? Palette.success : Palette.warning)
                        .contentTransition(.symbolEffect(.replace))
                    Text(item.label).font(.uiSmall)
                }
            }
        }
        .padding(.top, Space.s1)
    }
}

struct Citation: View {
    let label: String
    var symbol = "doc.text"

    var body: some View {
        HStack(spacing: Space.s1h) {
            Image(systemName: symbol).font(.uiIconSmall)
            Text(label).font(.uiSmall)
        }
        .foregroundStyle(Palette.muted)
        .padding(.leading, Space.s1)
    }
}

struct ListItem<Trailing: View>: View {
    let symbol: String
    var tone: Tone? = nil
    let title: String
    var subtitle: String? = nil
    @ViewBuilder var trailing: Trailing

    var body: some View {
        HStack(spacing: Space.s3) {
            if let tone {
                Glyph(symbol: symbol, tone: tone, shape: .square, scale: .small)
                    .transition(.scale.combined(with: .opacity))
            } else {
                Image(systemName: symbol)
                    .font(.uiIcon)
                    .frame(width: Size.tileSmall, height: Size.tileSmall)
            }
            VStack(alignment: .leading, spacing: Space.half) {
                Text(title).font(.uiMedium)
                if let subtitle {
                    Text(subtitle).font(.uiSmall).foregroundStyle(Palette.muted).lineLimit(1)
                }
            }
            Spacer(minLength: Space.s2)
            trailing
        }
        .foregroundStyle(Palette.foreground)
        .padding(Space.s3)
        .background(RoundedRectangle(cornerRadius: Radius.item, style: .continuous).fill(Palette.panel))
    }
}

struct StatGrid: View {
    struct Item: Identifiable {
        let label: String
        let value: String
        var id: String { label }
    }

    let items: [Item]
    var border: Color = Palette.line
    var labelColor: Color = Palette.muted

    var body: some View {
        HStack(spacing: 0) {
            ForEach(Array(items.enumerated()), id: \.element.id) { index, item in
                if index > 0 {
                    Rectangle().fill(border).frame(width: Size.line)
                }
                VStack(alignment: .leading, spacing: Space.half) {
                    Text(item.label).font(.uiSmall).foregroundStyle(labelColor)
                    Text(item.value).font(.uiSmallMedium).contentTransition(.numericText())
                }
                .padding(.horizontal, Space.s3)
                .padding(.vertical, Space.s2h)
                .frame(maxWidth: .infinity, alignment: .leading)
            }
        }
        .fixedSize(horizontal: false, vertical: true)
        .overlay(RoundedRectangle(cornerRadius: Radius.tile, style: .continuous).strokeBorder(border, lineWidth: Size.line))
    }
}

struct ActionRow: View {
    let symbol: String
    var tone: Tone = .ai
    let title: String
    let subtitle: String
    var action: () -> Void = {}

    var body: some View {
        Button(action: action) {
            HStack(spacing: Space.s3) {
                Glyph(symbol: symbol, tone: tone, scale: .large)
                VStack(alignment: .leading, spacing: Space.half) {
                    Text(title).font(.uiMedium)
                    Text(subtitle).font(.uiSmall).foregroundStyle(Palette.muted)
                }
                .multilineTextAlignment(.leading)
                Spacer(minLength: 0)
                Image(systemName: "chevron.right").font(.uiIconSmall).foregroundStyle(Palette.muted)
            }
            .foregroundStyle(Palette.foreground)
            .padding(Space.s3)
            .background(RoundedRectangle(cornerRadius: Radius.card, style: .continuous).fill(Palette.card))
        }
        .buttonStyle(PressStyle(scale: Motion.pressScaleSubtle))
    }
}

struct ActionBar<Content: View>: View {
    @ViewBuilder var content: Content

    var body: some View {
        HStack(spacing: Space.s2) {
            content
            Spacer(minLength: 0)
        }
        .padding(Space.s2)
        .frame(height: Size.actionBar)
        .background(Capsule().fill(Palette.card).shadow(color: Palette.shadow, radius: Space.s4, y: Space.s1h))
    }
}

struct StatusCard: View {
    let label: String
    var symbol = "hourglass"
    let title: String
    let subtitle: String
    let facts: [StatGrid.Item]
    var complete = false

    var body: some View {
        VStack(spacing: Space.s2) {
            HStack {
                HStack(spacing: Space.s1h) {
                    Image(systemName: symbol).font(.uiIconSmall).contentTransition(.symbolEffect(.replace))
                    Text(label).font(.uiSmallMedium)
                }
                .foregroundStyle(Palette.foreground)
                .padding(.horizontal, Space.s3h)
                .frame(height: Size.control - Space.s1)
                .background(Capsule().fill(Palette.card))
                Spacer()
                CircleButton(symbol: "bell", label: "Notify me")
            }
            VStack(alignment: .leading, spacing: Space.s1) {
                Text(title).font(.uiHero).tracking(Tracking.tight).contentTransition(.interpolate)
                Text(subtitle).font(.uiSmall).foregroundStyle(Palette.inkMuted)
                StatGrid(items: facts, border: complete ? Palette.successBorder : Palette.pendingBorder, labelColor: Palette.inkSubtle)
                    .padding(.top, Space.s3)
            }
            .foregroundStyle(Palette.ink)
            .padding(.horizontal, Space.s4)
            .padding(.top, Space.s5)
            .padding(.bottom, Space.s4)
            .frame(maxWidth: .infinity, alignment: .leading)
            .background(RoundedRectangle(cornerRadius: Radius.panel, style: .continuous).fill(complete ? Palette.successPale : Palette.pendingSoft))
        }
        .padding(Space.s2)
        .background(RoundedRectangle(cornerRadius: Radius.card, style: .continuous).fill(complete ? Palette.mint : Palette.pending))
    }
}

struct Timeline: View {
    struct Event: Identifiable, Equatable {
        let date: String
        let symbol: String
        let tone: Tone
        let title: String
        let subtitle: String
        var id: String { title }

        static func == (a: Event, b: Event) -> Bool { a.id == b.id && a.subtitle == b.subtitle }
    }

    let events: [Event]

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            ForEach(Array(events.enumerated()), id: \.element.id) { index, event in
                ZStack(alignment: .topLeading) {
                    if index < events.count - 1 {
                        Rectangle()
                            .fill(Palette.foreground)
                            .frame(width: Size.rail)
                            .frame(maxHeight: .infinity)
                            .padding(.leading, Size.dateColumn + Space.s3 + Size.tile / 2 - Size.rail / 2)
                            .padding(.top, Size.tile)
                    }
                    HStack(alignment: .top, spacing: Space.s3) {
                        Text(event.date)
                            .font(.uiMono)
                            .foregroundStyle(Palette.muted)
                            .frame(width: Size.dateColumn, alignment: .leading)
                            .padding(.top, Space.s1)
                        Glyph(symbol: event.symbol, tone: event.tone)
                        VStack(alignment: .leading, spacing: Space.s1) {
                            Text(event.title).font(.uiMedium)
                            Text(event.subtitle).font(.uiSmall).foregroundStyle(Palette.muted)
                        }
                        .padding(.top, Space.s1h)
                    }
                    .padding(.bottom, Space.s5)
                }
                .transition(.asymmetric(insertion: .offset(y: -Space.s5).combined(with: .opacity), removal: .opacity))
            }
        }
        .foregroundStyle(Palette.foreground)
    }
}
