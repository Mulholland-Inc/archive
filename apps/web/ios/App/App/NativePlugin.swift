import UIKit
import Capacitor

@objc(NativePlugin)
public class NativePlugin: CAPPlugin, CAPBridgedPlugin {
    public let identifier = "NativePlugin"
    public let jsName = "Native"
    public let pluginMethods: [CAPPluginMethod] = [
        CAPPluginMethod(name: "show", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "hide", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "setEnabled", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "setText", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "clear", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "toolbar", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "setState", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "topBar", returnType: CAPPluginReturnPromise),
        CAPPluginMethod(name: "shift", returnType: CAPPluginReturnPromise)
    ]

    private lazy var composer = ComposerBar { [weak self] text in self?.notifyListeners("send", data: ["text": text]) }
    private lazy var tools = ToolsBar { [weak self] action in self?.notifyListeners("tap", data: ["action": action]) }
    private lazy var top = TopBar { [weak self] action in self?.notifyListeners("tap", data: ["action": action]) }
    private var keyboard = false
    private var toolsWanted = false
    private var cover: CGFloat = -1

    override public func load() {
        let center = NotificationCenter.default
        center.addObserver(self, selector: #selector(keyboardWillShow), name: UIResponder.keyboardWillShowNotification, object: nil)
        center.addObserver(self, selector: #selector(keyboardWillHide), name: UIResponder.keyboardWillHideNotification, object: nil)
        center.addObserver(self, selector: #selector(report), name: UIResponder.keyboardDidShowNotification, object: nil)
        center.addObserver(self, selector: #selector(report), name: UIResponder.keyboardDidHideNotification, object: nil)
        composer.onResize = { [weak self] in self?.report() }
    }

    @objc func show(_ call: CAPPluginCall) {
        DispatchQueue.main.async {
            self.attach(self.composer)
            self.composer.isHidden = false
            self.report()
            call.resolve()
        }
    }

    @objc func hide(_ call: CAPPluginCall) {
        DispatchQueue.main.async {
            self.composer.field.resignFirstResponder()
            self.composer.isHidden = true
            self.report()
            call.resolve()
        }
    }

    @objc func setEnabled(_ call: CAPPluginCall) {
        let on = call.getBool("enabled", true)
        DispatchQueue.main.async {
            self.composer.enabled = on
            call.resolve()
        }
    }

    @objc func setText(_ call: CAPPluginCall) {
        let text = call.getString("text", "")
        DispatchQueue.main.async {
            self.composer.text = text
            call.resolve()
        }
    }

    @objc func clear(_ call: CAPPluginCall) {
        DispatchQueue.main.async {
            self.composer.text = ""
            call.resolve()
        }
    }

    @objc func toolbar(_ call: CAPPluginCall) {
        let on = call.getBool("show", true)
        DispatchQueue.main.async {
            self.attach(self.tools)
            self.toolsWanted = on
            self.updateTools()
            call.resolve()
        }
    }

    @objc func setState(_ call: CAPPluginCall) {
        let type = call.getString("type")
        let marks = Set((call.getArray("marks") as? [String]) ?? [])
        DispatchQueue.main.async {
            self.tools.update(type: type, marks: marks)
            call.resolve()
        }
    }

    @objc func topBar(_ call: CAPPluginCall) {
        let on = call.getBool("show", true)
        DispatchQueue.main.async {
            if self.top.superview == nil, let view = self.bridge?.viewController?.view {
                self.top.translatesAutoresizingMaskIntoConstraints = false
                view.addSubview(self.top)
                NSLayoutConstraint.activate([
                    self.top.leadingAnchor.constraint(equalTo: view.leadingAnchor),
                    self.top.trailingAnchor.constraint(equalTo: view.trailingAnchor),
                    self.top.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
                    self.top.heightAnchor.constraint(equalToConstant: 60)
                ])
            }
            self.top.isHidden = !on
            call.resolve()
        }
    }

    @objc func shift(_ call: CAPPluginCall) {
        let px = CGFloat(call.getDouble("px", 0))
        let progress = CGFloat(call.getDouble("p", 0))
        let animate = call.getBool("animate", false) && !UIAccessibility.isReduceMotionEnabled
        DispatchQueue.main.async {
            let apply = {
                for bar in [self.composer, self.tools, self.top] as [UIView] {
                    bar.transform = CGAffineTransform(translationX: px, y: 0)
                    bar.alpha = 1 - 0.4 * progress
                }
            }
            if animate {
                let timing = UICubicTimingParameters(controlPoint1: CGPoint(x: 0.32, y: 0.72), controlPoint2: CGPoint(x: 0, y: 1))
                let animator = UIViewPropertyAnimator(duration: 0.4, timingParameters: timing)
                animator.isUserInteractionEnabled = true
                animator.addAnimations(apply)
                animator.startAnimation()
            } else {
                apply()
            }
            call.resolve()
        }
    }

    @objc private func keyboardWillShow() {
        DispatchQueue.main.async {
            self.keyboard = true
            self.updateTools()
        }
    }

    @objc private func keyboardWillHide() {
        DispatchQueue.main.async {
            self.keyboard = false
            self.updateTools()
        }
    }

    private func updateTools() {
        tools.isHidden = !(toolsWanted && keyboard && !composer.field.isFirstResponder)
        report()
    }

    private func attach(_ bar: UIView) {
        guard bar.superview == nil, let view = bridge?.viewController?.view else { return }
        bar.isHidden = true
        bar.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(bar)
        NSLayoutConstraint.activate([
            bar.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            bar.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            bar.bottomAnchor.constraint(equalTo: view.keyboardLayoutGuide.topAnchor)
        ])
    }

    @objc private func report() {
        DispatchQueue.main.async {
            guard let view = self.bridge?.viewController?.view, let web = self.bridge?.webView else { return }
            view.layoutIfNeeded()
            let bottom = web.convert(web.bounds, to: view).maxY
            let tops = [self.composer, self.tools].filter { $0.superview != nil && !$0.isHidden }.map { $0.frame.minY }
            let px = max(0, bottom - (tops.min() ?? bottom))
            if px != self.cover {
                self.cover = px
                self.notifyListeners("height", data: ["px": px])
            }
        }
    }
}

final class Glass: UIVisualEffectView {
    init(radius: CGFloat) {
        if #available(iOS 26.0, *) {
            let glass = UIGlassEffect()
            glass.isInteractive = true
            super.init(effect: glass)
        } else {
            super.init(effect: UIBlurEffect(style: .systemChromeMaterial))
        }
        layer.cornerRadius = radius
        layer.cornerCurve = .continuous
        clipsToBounds = true
        translatesAutoresizingMaskIntoConstraints = false
    }

    required init?(coder: NSCoder) { nil }
}

final class TopBar: UIView {
    init(onTap: @escaping (String) -> Void) {
        super.init(frame: .zero)
        isHidden = true
        let left = Self.button("sidebar.left", label: "Open sidebar") { onTap("sidebar") }
        let right = Self.button("square.and.pencil", label: "New chat") { onTap("newChat") }
        addSubview(left)
        addSubview(right)
        NSLayoutConstraint.activate([
            left.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 8),
            left.topAnchor.constraint(equalTo: topAnchor, constant: 8),
            right.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -8),
            right.topAnchor.constraint(equalTo: topAnchor, constant: 8)
        ])
    }

    required init?(coder: NSCoder) { nil }

    override func hitTest(_ point: CGPoint, with event: UIEvent?) -> UIView? {
        let hit = super.hitTest(point, with: event)
        return hit === self ? nil : hit
    }

    private static func button(_ symbol: String, label: String, tap: @escaping () -> Void) -> UIButton {
        var c: UIButton.Configuration
        if #available(iOS 26.0, *) {
            c = .glass()
        } else {
            c = .plain()
        }
        c.image = UIImage(systemName: symbol, withConfiguration: UIImage.SymbolConfiguration(pointSize: 17, weight: .medium))
        c.baseForegroundColor = .label
        c.cornerStyle = .capsule
        let button = UIButton(configuration: c, primaryAction: UIAction { _ in
            UIImpactFeedbackGenerator(style: .light).impactOccurred()
            tap()
        })
        button.accessibilityLabel = label
        button.translatesAutoresizingMaskIntoConstraints = false
        button.widthAnchor.constraint(equalToConstant: 44).isActive = true
        button.heightAnchor.constraint(equalToConstant: 44).isActive = true
        return button
    }
}

final class ComposerBar: UIView, UITextViewDelegate {
    let field = UITextView()
    private let placeholder = UILabel()
    private let send = UIButton(type: .system)
    private let glass = Glass(radius: 22)
    private lazy var height = field.heightAnchor.constraint(equalToConstant: 36)
    private let onSend: (String) -> Void
    var onResize: (() -> Void)?

    var enabled = true { didSet { refresh() } }
    var text: String {
        get { field.text }
        set {
            field.text = newValue
            refresh()
        }
    }

    init(onSend: @escaping (String) -> Void) {
        self.onSend = onSend
        super.init(frame: .zero)
        field.font = .preferredFont(forTextStyle: .body)
        field.adjustsFontForContentSizeCategory = true
        field.backgroundColor = .clear
        field.textContainerInset = UIEdgeInsets(top: 8, left: 4, bottom: 8, right: 4)
        field.autocapitalizationType = .sentences
        field.autocorrectionType = .yes
        field.isScrollEnabled = false
        field.delegate = self
        field.translatesAutoresizingMaskIntoConstraints = false
        placeholder.text = "Message"
        placeholder.font = field.font
        placeholder.textColor = .placeholderText
        placeholder.translatesAutoresizingMaskIntoConstraints = false
        send.setImage(UIImage(systemName: "arrow.up.circle.fill", withConfiguration: UIImage.SymbolConfiguration(pointSize: 30)), for: .normal)
        send.tintColor = .label
        send.accessibilityLabel = "Send"
        send.addTarget(self, action: #selector(tapSend), for: .touchUpInside)
        send.translatesAutoresizingMaskIntoConstraints = false
        addSubview(glass)
        glass.contentView.addSubview(field)
        glass.contentView.addSubview(placeholder)
        glass.contentView.addSubview(send)
        NSLayoutConstraint.activate([
            glass.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 12),
            glass.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -12),
            glass.topAnchor.constraint(equalTo: topAnchor, constant: 8),
            glass.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -8),
            field.leadingAnchor.constraint(equalTo: glass.contentView.leadingAnchor, constant: 10),
            field.topAnchor.constraint(equalTo: glass.contentView.topAnchor, constant: 4),
            field.bottomAnchor.constraint(equalTo: glass.contentView.bottomAnchor, constant: -4),
            height,
            placeholder.leadingAnchor.constraint(equalTo: field.leadingAnchor, constant: 9),
            placeholder.centerYAnchor.constraint(equalTo: field.centerYAnchor),
            send.leadingAnchor.constraint(equalTo: field.trailingAnchor, constant: 4),
            send.trailingAnchor.constraint(equalTo: glass.contentView.trailingAnchor, constant: -6),
            send.bottomAnchor.constraint(equalTo: glass.contentView.bottomAnchor, constant: -6),
            send.widthAnchor.constraint(equalToConstant: 32),
            send.heightAnchor.constraint(equalToConstant: 32)
        ])
        refresh()
    }

    required init?(coder: NSCoder) { nil }

    func textViewDidChange(_ textView: UITextView) { refresh() }

    private func refresh() {
        placeholder.isHidden = !field.text.isEmpty
        send.isEnabled = enabled && !field.text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
        let line = field.font?.lineHeight ?? 20
        let inset = field.textContainerInset.top + field.textContainerInset.bottom
        let fit = field.sizeThatFits(CGSize(width: field.bounds.width > 0 ? field.bounds.width : 240, height: .greatestFiniteMagnitude)).height
        let target = min(max(fit, line + inset), line * 5 + inset)
        field.isScrollEnabled = fit > target
        if height.constant != target {
            height.constant = target
            superview?.layoutIfNeeded()
            onResize?()
        }
    }

    override func layoutSubviews() {
        super.layoutSubviews()
        refresh()
    }

    @objc private func tapSend() {
        let text = field.text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard send.isEnabled, !text.isEmpty else { return }
        UIImpactFeedbackGenerator(style: .light).impactOccurred()
        onSend(text)
    }
}

final class ToolsBar: UIView {
    private static let items: [(String, String?, String?)] = [
        ("turn:paragraph", "paragraphsign", nil), ("turn:heading1", nil, "H1"), ("turn:heading2", nil, "H2"), ("turn:heading3", nil, "H3"),
        ("turn:bullet", "list.bullet", nil), ("turn:numbered", "list.number", nil), ("turn:todo", "checklist", nil),
        ("turn:quote", "text.quote", nil), ("turn:code", "curlybraces", nil), ("", nil, nil),
        ("indent:-1", "decrease.indent", nil), ("indent:1", "increase.indent", nil), ("", nil, nil),
        ("mark:bold", "bold", nil), ("mark:italic", "italic", nil), ("mark:underline", "underline", nil),
        ("mark:strike", "strikethrough", nil), ("mark:code", "chevron.left.forwardslash.chevron.right", nil), ("mark:link", "link", nil)
    ]
    private var buttons: [String: UIButton] = [:]
    private let onTap: (String) -> Void

    init(onTap: @escaping (String) -> Void) {
        self.onTap = onTap
        super.init(frame: .zero)
        let glass = Glass(radius: 24)
        let scroll = UIScrollView()
        scroll.showsHorizontalScrollIndicator = false
        scroll.translatesAutoresizingMaskIntoConstraints = false
        let stack = UIStackView()
        stack.axis = .horizontal
        stack.alignment = .center
        stack.spacing = 2
        stack.translatesAutoresizingMaskIntoConstraints = false
        for (action, symbol, title) in Self.items {
            if action.isEmpty {
                let line = UIView()
                line.backgroundColor = .separator
                line.translatesAutoresizingMaskIntoConstraints = false
                line.widthAnchor.constraint(equalToConstant: 1).isActive = true
                line.heightAnchor.constraint(equalToConstant: 22).isActive = true
                stack.addArrangedSubview(line)
                continue
            }
            let button = UIButton(configuration: Self.style(on: false, symbol: symbol, title: title), primaryAction: UIAction { [weak self] _ in
                UISelectionFeedbackGenerator().selectionChanged()
                self?.onTap(action)
            })
            button.accessibilityLabel = action
            button.widthAnchor.constraint(greaterThanOrEqualToConstant: 40).isActive = true
            button.heightAnchor.constraint(equalToConstant: 40).isActive = true
            buttons[action] = button
            stack.addArrangedSubview(button)
        }
        addSubview(glass)
        glass.contentView.addSubview(scroll)
        scroll.addSubview(stack)
        NSLayoutConstraint.activate([
            glass.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 8),
            glass.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -8),
            glass.topAnchor.constraint(equalTo: topAnchor, constant: 6),
            glass.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -6),
            glass.heightAnchor.constraint(equalToConstant: 48),
            scroll.leadingAnchor.constraint(equalTo: glass.contentView.leadingAnchor),
            scroll.trailingAnchor.constraint(equalTo: glass.contentView.trailingAnchor),
            scroll.topAnchor.constraint(equalTo: glass.contentView.topAnchor),
            scroll.bottomAnchor.constraint(equalTo: glass.contentView.bottomAnchor),
            stack.leadingAnchor.constraint(equalTo: scroll.contentLayoutGuide.leadingAnchor, constant: 6),
            stack.trailingAnchor.constraint(equalTo: scroll.contentLayoutGuide.trailingAnchor, constant: -6),
            stack.topAnchor.constraint(equalTo: scroll.contentLayoutGuide.topAnchor),
            stack.bottomAnchor.constraint(equalTo: scroll.contentLayoutGuide.bottomAnchor),
            stack.heightAnchor.constraint(equalTo: scroll.frameLayoutGuide.heightAnchor)
        ])
    }

    required init?(coder: NSCoder) { nil }

    func update(type: String?, marks: Set<String>) {
        for (action, symbol, title) in Self.items where !action.isEmpty {
            let parts = action.split(separator: ":").map(String.init)
            let on = (parts[0] == "turn" && parts[1] == type) || (parts[0] == "mark" && marks.contains(parts[1]))
            buttons[action]?.configuration = Self.style(on: on, symbol: symbol, title: title)
        }
    }

    private static func style(on: Bool, symbol: String?, title: String?) -> UIButton.Configuration {
        var c: UIButton.Configuration = on ? .filled() : .plain()
        c.cornerStyle = .capsule
        c.baseForegroundColor = on ? .systemBackground : .label
        c.baseBackgroundColor = .label
        c.contentInsets = NSDirectionalEdgeInsets(top: 6, leading: 8, bottom: 6, trailing: 8)
        if let symbol {
            c.image = UIImage(systemName: symbol, withConfiguration: UIImage.SymbolConfiguration(pointSize: 16, weight: .medium))
        }
        if let title {
            var t = AttributedString(title)
            t.font = .systemFont(ofSize: 15, weight: .semibold)
            c.attributedTitle = t
        }
        return c
    }
}
