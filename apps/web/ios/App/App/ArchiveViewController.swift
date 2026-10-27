import UIKit
import Capacitor

class ArchiveViewController: CAPBridgeViewController {
    static let didLayout = Notification.Name("ArchiveDidLayout")
    private var pending: URL?

    override func capacitorDidLoad() {
        bridge?.registerPluginInstance(NativePlugin())
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        let shell = UIColor(named: "Shell")
        view.backgroundColor = shell
        webView?.isOpaque = false
        webView?.backgroundColor = shell
        webView?.scrollView.backgroundColor = shell
        webView?.scrollView.keyboardDismissMode = .interactive
        if #available(iOS 26.0, *) {
            webView?.scrollView.topEdgeEffect.isHidden = true
            webView?.scrollView.bottomEdgeEffect.isHidden = true
        }
        if let url = pending { open(url) }
    }

    override func viewDidLayoutSubviews() {
        super.viewDidLayoutSubviews()
        NotificationCenter.default.post(name: Self.didLayout, object: nil)
    }

    func open(_ url: URL) {
        guard isViewLoaded, let base = bridge?.config.serverURL, var to = URLComponents(url: base, resolvingAgainstBaseURL: false) else {
            pending = url
            return
        }
        pending = nil
        to.path = "/" + (url.host ?? "") + url.path
        to.query = url.query
        if let target = to.url { webView?.load(URLRequest(url: target)) }
    }
}
