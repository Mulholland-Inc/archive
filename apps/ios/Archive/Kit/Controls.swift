import SwiftUI

struct PressStyle: ButtonStyle {
    var scale: CGFloat = Motion.pressScale

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? scale : 1)
            .animation(Motion.press, value: configuration.isPressed)
    }
}

struct CircleButton: View {
    enum Style { case card, muted }
    enum Scale { case regular, large }

    let symbol: String
    var style: Style = .card
    var scale: Scale = .regular
    var label: String = ""
    var action: () -> Void = {}

    var body: some View {
        Button(action: action) {
            CircleLabel(symbol: symbol, style: style, scale: scale)
        }
        .buttonStyle(PressStyle(scale: Motion.pressScaleStrong))
        .accessibilityLabel(label)
    }
}

struct CircleLabel: View {
    let symbol: String
    var style: CircleButton.Style = .card
    var scale: CircleButton.Scale = .regular

    var body: some View {
        let size = scale == .large ? Size.button : Size.control
        Image(systemName: symbol)
            .font(.uiIcon)
            .foregroundStyle(Palette.foreground)
            .frame(width: size, height: size)
            .background(Circle().fill(style == .card ? Palette.card : Palette.panel))
            .contentShape(Circle())
    }
}

struct Avatar: View {
    let name: String
    var tone: Tone = .mint

    var initials: String {
        name.split(separator: " ").prefix(2).compactMap(\.first).map(String.init).joined().uppercased()
    }

    var body: some View {
        Text(initials)
            .font(.uiInitials)
            .foregroundStyle(tone.glyphInk)
            .frame(width: Size.avatar, height: Size.avatar)
            .background(Circle().fill(tone.fill))
    }
}

struct Chip: View {
    var tone: Tone? = nil
    var symbol: String? = nil
    var person: String? = nil
    var label: String? = nil

    var body: some View {
        HStack(spacing: Space.s1h) {
            if tone != nil {
                Circle().fill(Palette.ink).frame(width: Size.dot, height: Size.dot)
            }
            if let person {
                Avatar(name: person)
            }
            if let symbol {
                Image(systemName: symbol).font(.uiIconSmall)
            }
            if let label {
                Text(label).font(.uiSmallMedium).contentTransition(.interpolate)
            }
        }
        .foregroundStyle(tone?.ink ?? Palette.foreground)
        .padding(.leading, leadingInset)
        .padding(.trailing, label == nil ? Space.s2h : Space.s3)
        .frame(height: Size.chip)
        .background(Capsule().fill(tone?.fill ?? Palette.card))
        .fixedSize()
    }

    private var leadingInset: CGFloat {
        if person != nil { return Space.s1 }
        return Space.s2h
    }
}

struct ChipRail<Content: View>: View {
    @ViewBuilder var content: Content

    var body: some View {
        ScrollView(.horizontal) {
            HStack(spacing: Space.s1h) { content }
                .padding(.horizontal, Space.gutter)
        }
        .scrollIndicators(.hidden)
        .padding(.horizontal, -Space.gutter)
    }
}

struct Tag: View {
    let label: String
    var tone: Tone = .success
    var symbol: String? = nil
    var dot = false

    var body: some View {
        HStack(spacing: Space.s1) {
            if dot {
                Circle().fill(tone.ink).frame(width: Size.dot, height: Size.dot)
            }
            if let symbol {
                Image(systemName: symbol).font(.uiIconSmall)
            }
            Text(label).font(.uiSmallMedium).contentTransition(.numericText())
        }
        .foregroundStyle(tone.ink)
        .padding(.horizontal, Space.s2h)
        .frame(height: Size.tag)
        .background(Capsule().fill(tone.fill))
        .fixedSize()
    }
}

struct SegmentedTabs<Value: Hashable>: View {
    struct Item: Identifiable {
        let value: Value
        let label: String
        var id: Value { value }
    }

    @Binding var selection: Value
    let items: [Item]
    @Namespace private var namespace

    var body: some View {
        HStack(spacing: 0) {
            ForEach(items) { item in
                let active = item.value == selection
                Button {
                    selection = item.value
                } label: {
                    Text(item.label)
                        .font(.uiMedium)
                        .foregroundStyle(active ? Palette.primaryForeground : Palette.muted)
                        .padding(.horizontal, Space.s3h)
                        .frame(height: Size.control - Space.s2)
                        .background {
                            if active {
                                Capsule().fill(Palette.primary).matchedGeometryEffect(id: "thumb", in: namespace)
                            }
                        }
                        .contentShape(Capsule())
                }
                .buttonStyle(PressStyle())
            }
        }
        .padding(Space.s1)
        .background(Capsule().fill(Palette.card))
        .animation(Motion.spring, value: selection)
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

struct NavBar<Trailing: View>: View {
    let title: String
    let subtitle: String
    var onBack: () -> Void = {}
    @ViewBuilder var trailing: Trailing

    var body: some View {
        HStack(spacing: Space.s3) {
            CircleButton(symbol: "chevron.left", label: "Back", action: onBack)
            Spacer(minLength: 0)
            VStack(spacing: Space.half) {
                Text(title).font(.uiSemibold).tracking(Tracking.snug)
                Text(subtitle).font(.uiMono).foregroundStyle(Palette.muted)
            }
            .lineLimit(1)
            Spacer(minLength: 0)
            trailing
        }
        .foregroundStyle(Palette.foreground)
    }
}

enum ActionState: Equatable { case idle, loading, done }

struct PrimaryButton: View {
    let symbol: String
    let title: String
    var state: ActionState = .idle
    var action: () -> Void = {}

    var body: some View {
        Button(action: action) {
            HStack(spacing: Space.s2) {
                ZStack {
                    switch state {
                    case .idle:
                        Image(systemName: symbol).transition(.scale(scale: Motion.enterScale).combined(with: .opacity))
                    case .loading:
                        ProgressView().controlSize(.small).tint(Palette.primaryForeground)
                            .transition(.scale(scale: Motion.enterScale).combined(with: .opacity))
                    case .done:
                        Image(systemName: "checkmark").transition(.scale(scale: Motion.enterScale).combined(with: .opacity))
                    }
                }
                .font(.uiIcon)
                .frame(width: Size.icon + Space.s1, height: Size.icon + Space.s1)
                Text(title).font(.uiMedium).contentTransition(.interpolate)
            }
            .foregroundStyle(state == .done ? Palette.onColor : Palette.primaryForeground)
            .padding(.horizontal, Space.s6)
            .frame(height: Size.button)
            .background(Capsule().fill(state == .done ? Palette.success : Palette.primary))
            .animation(Motion.bouncy, value: state)
        }
        .buttonStyle(PressStyle())
        .disabled(state != .idle)
        .sensoryFeedback(.impact(weight: .light), trigger: state) { _, new in new == .loading }
        .sensoryFeedback(.success, trigger: state) { _, new in new == .done }
    }
}
