import SwiftUI
import UIKit

extension UIColor {
    convenience init(hex: UInt32, alpha: CGFloat = 1) {
        self.init(
            red: CGFloat((hex >> 16) & 0xff) / 255,
            green: CGFloat((hex >> 8) & 0xff) / 255,
            blue: CGFloat(hex & 0xff) / 255,
            alpha: alpha
        )
    }
}

extension Color {
    init(hex: UInt32) {
        self.init(uiColor: UIColor(hex: hex))
    }

    static func dynamic(_ light: UInt32, _ dark: UInt32) -> Color {
        Color(uiColor: UIColor { $0.userInterfaceStyle == .dark ? UIColor(hex: dark) : UIColor(hex: light) })
    }

    static func dynamic(light: UIColor, dark: UIColor) -> Color {
        Color(uiColor: UIColor { $0.userInterfaceStyle == .dark ? dark : light })
    }
}

enum Primitive {
    static let stone0: UInt32 = 0xffffff
    static let stone50: UInt32 = 0xf3f2ee
    static let stone100: UInt32 = 0xeaebe6
    static let stone500: UInt32 = 0x8c8b86
    static let stone950: UInt32 = 0x0a0a0a
    static let night0: UInt32 = 0x2a2a28
    static let night50: UInt32 = 0x343431
    static let night100: UInt32 = 0x1b1b1a
    static let night500: UInt32 = 0x9a9993
    static let night950: UInt32 = 0xf4f4f2
    static let lime400: UInt32 = 0xd6ed3d
    static let emerald400: UInt32 = 0x2fdda0
    static let emerald500: UInt32 = 0x21b45c
    static let emerald100: UInt32 = 0xdff5e7
    static let emerald900: UInt32 = 0x1f3a2a
    static let lavender300: UInt32 = 0xc4b4fb
    static let tangerine500: UInt32 = 0xf9a11b
    static let sun400: UInt32 = 0xfccc0a
    static let sun100: UInt32 = 0xfff4c2
    static let sun200: UInt32 = 0xf3dc84
    static let sun900: UInt32 = 0x4a3f12
    static let sun800: UInt32 = 0x6b5a17
}

enum Palette {
    static let background = Color.dynamic(Primitive.stone100, Primitive.night100)
    static let card = Color.dynamic(Primitive.stone0, Primitive.night0)
    static let panel = Color.dynamic(Primitive.stone50, Primitive.night50)
    static let foreground = Color.dynamic(Primitive.stone950, Primitive.night950)
    static let muted = Color.dynamic(Primitive.stone500, Primitive.night500)
    static let primary = foreground
    static let primaryForeground = Color.dynamic(Primitive.stone0, Primitive.stone950)
    static let ink = Color(hex: Primitive.stone950)
    static let onColor = Color(hex: Primitive.stone0)
    static let highlight = Color(hex: Primitive.lime400)
    static let success = Color(hex: Primitive.emerald500)
    static let successSoft = Color.dynamic(Primitive.emerald100, Primitive.emerald900)
    static let mint = Color(hex: Primitive.emerald400)
    static let ai = Color(hex: Primitive.lavender300)
    static let warning = Color(hex: Primitive.tangerine500)
    static let pending = Color(hex: Primitive.sun400)
    static let pendingSoft = Color(hex: Primitive.sun100)
    static let pendingBorder = Color(hex: Primitive.sun200)
    static let inkMuted = Color(hex: Primitive.stone950).opacity(0.6)
    static let inkSubtle = Color(hex: Primitive.stone950).opacity(0.45)
    static let successBorder = Color(hex: Primitive.emerald500).opacity(0.35)
    static let successPale = Color(hex: Primitive.emerald100)
    static let line = Color.dynamic(light: UIColor(white: 0, alpha: 0.09), dark: UIColor(white: 1, alpha: 0.12))
    static let pressed = Color.dynamic(light: UIColor(white: 0, alpha: 0.04), dark: UIColor(white: 1, alpha: 0.06))
    static let shadow = Color.dynamic(light: UIColor(white: 0, alpha: 0.09), dark: UIColor(white: 0, alpha: 0.4))
}

enum Space {
    static let half: CGFloat = 2
    static let s1: CGFloat = 4
    static let s1h: CGFloat = 6
    static let s2: CGFloat = 8
    static let s2h: CGFloat = 10
    static let s3: CGFloat = 12
    static let s3h: CGFloat = 14
    static let s4: CGFloat = 16
    static let s5: CGFloat = 20
    static let s6: CGFloat = 24
    static let s12: CGFloat = 48
    static let gutter: CGFloat = 16
}

enum Radius {
    static let tile: CGFloat = 12
    static let item: CGFloat = 16
    static let panel: CGFloat = 20
    static let card: CGFloat = 28
}

enum Size {
    static let control: CGFloat = 40
    static let button: CGFloat = 48
    static let actionBar: CGFloat = 64
    static let chip: CGFloat = 32
    static let tag: CGFloat = 26
    static let tile: CGFloat = 40
    static let tileSmall: CGFloat = 32
    static let tileLarge: CGFloat = 48
    static let avatar: CGFloat = 24
    static let progress: CGFloat = 5
    static let dot: CGFloat = 6
    static let line: CGFloat = 1
    static let rail: CGFloat = 2
    static let dateColumn: CGFloat = 52
    static let icon: CGFloat = 15
    static let iconSmall: CGFloat = 12
}

extension Font {
    static let ui = Font.system(size: 15)
    static let uiMedium = Font.system(size: 15, weight: .medium)
    static let uiSemibold = Font.system(size: 15, weight: .semibold)
    static let uiSmall = Font.system(size: 13)
    static let uiSmallMedium = Font.system(size: 13, weight: .medium)
    static let uiMono = Font.system(size: 12, design: .monospaced)
    static let uiTitle = Font.system(size: 22, weight: .medium)
    static let uiHero = Font.system(size: 26, weight: .medium)
    static let uiInitials = Font.system(size: 11, weight: .medium)
    static let uiIcon = Font.system(size: 15, weight: .medium)
    static let uiIconSmall = Font.system(size: 12, weight: .semibold)
}

enum Tracking {
    static let tight: CGFloat = -0.5
    static let snug: CGFloat = -0.2
}

enum Motion {
    static let spring = Animation.spring(response: 0.42, dampingFraction: 0.84)
    static let bouncy = Animation.spring(response: 0.36, dampingFraction: 0.68)
    static let press = Animation.spring(response: 0.22, dampingFraction: 0.62)
    static let fade = Animation.easeOut(duration: 0.22)
    static let fill = Animation.easeOut(duration: 0.34)
    static let stagger: Double = 0.035
    static let pressScale: CGFloat = 0.96
    static let pressScaleSubtle: CGFloat = 0.985
    static let pressScaleStrong: CGFloat = 0.9
    static let enterScale: CGFloat = 0.4
}

enum Tone {
    case success, warning, ai, pending, muted, highlight, mint, card

    var fill: Color {
        switch self {
        case .success: Palette.success
        case .warning: Palette.warning
        case .ai: Palette.ai
        case .pending: Palette.pending
        case .muted: Palette.panel
        case .highlight: Palette.highlight
        case .mint: Palette.mint
        case .card: Palette.card
        }
    }

    var ink: Color {
        switch self {
        case .success: Palette.onColor
        case .muted, .card: Palette.foreground
        default: Palette.ink
        }
    }

    var glyphInk: Color {
        switch self {
        case .muted, .card: Palette.foreground
        default: Palette.ink
        }
    }
}
