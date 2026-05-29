// Native macOS wrapper for the Memory Flow standalone server.
//
// Spawns the embedded `memory_flow serve` binary on a free loopback port, waits
// for it to come up, then hosts the UI in a WKWebView window (no browser
// chrome). Quitting the app stops the server. Follows the ORT local-client
// recipe: pre-warm the server before showing the WebView; keep Cmd+S out of the
// menu so the SPA owns it.

import Cocoa
import WebKit

// Picks a free TCP port on 127.0.0.1 by binding to port 0 and reading back the
// assigned port. There is a small race before the Go server rebinds it, but for
// a single-user localhost app that is acceptable.
func freeLoopbackPort() -> Int {
    let fd = socket(AF_INET, SOCK_STREAM, 0)
    guard fd >= 0 else { return 8080 }
    defer { close(fd) }
    var addr = sockaddr_in()
    addr.sin_family = sa_family_t(AF_INET)
    addr.sin_addr.s_addr = inet_addr("127.0.0.1")
    addr.sin_port = 0
    let bound = withUnsafePointer(to: &addr) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
            Darwin.bind(fd, $0, socklen_t(MemoryLayout<sockaddr_in>.size))
        }
    }
    guard bound == 0 else { return 8080 }
    var len = socklen_t(MemoryLayout<sockaddr_in>.size)
    var out = sockaddr_in()
    let ok = withUnsafeMutablePointer(to: &out) {
        $0.withMemoryRebound(to: sockaddr.self, capacity: 1) {
            getsockname(fd, $0, &len)
        }
    }
    guard ok == 0 else { return 8080 }
    return Int(UInt16(bigEndian: out.sin_port))
}

final class GoServer {
    private let process = Process()
    let port: Int
    var url: URL { URL(string: "http://127.0.0.1:\(port)/")! }

    init() {
        self.port = freeLoopbackPort()
    }

    func start() {
        // The Go binary is bundled at Contents/Resources/memory_flow.
        guard let bin = Bundle.main.url(forResource: "memory_flow", withExtension: nil) else {
            fatalError("bundled memory_flow binary not found")
        }
        process.executableURL = bin
        process.arguments = ["serve"]
        var env = ProcessInfo.processInfo.environment
        env["PORT"] = String(port)
        process.environment = env
        do {
            try process.run()
        } catch {
            NSLog("failed to start memory_flow: \(error)")
        }
    }

    // Polls the server until it responds or the timeout elapses.
    func waitUntilReady(timeout: TimeInterval = 15) -> Bool {
        let deadline = Date().addingTimeInterval(timeout)
        while Date() < deadline {
            let sem = DispatchSemaphore(value: 0)
            var ok = false
            var req = URLRequest(url: url)
            req.timeoutInterval = 1
            let task = URLSession.shared.dataTask(with: req) { _, resp, _ in
                if let http = resp as? HTTPURLResponse, http.statusCode == 200 { ok = true }
                sem.signal()
            }
            task.resume()
            _ = sem.wait(timeout: .now() + 1.5)
            if ok { return true }
            Thread.sleep(forTimeInterval: 0.3)
        }
        return false
    }

    func stop() {
        if process.isRunning { process.terminate() }
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate, WKNavigationDelegate {
    private var window: NSWindow!
    private var webView: WKWebView!
    private let server = GoServer()

    func applicationDidFinishLaunching(_ note: Notification) {
        setupMenu()
        server.start()
        // Pre-warm: wait for the server off the main thread, then build the window.
        DispatchQueue.global().async {
            let ready = self.server.waitUntilReady()
            DispatchQueue.main.async { self.showWindow(ready: ready) }
        }
    }

    private func showWindow(ready: Bool) {
        let config = WKWebViewConfiguration()
        webView = WKWebView(frame: .zero, configuration: config)
        webView.navigationDelegate = self

        window = NSWindow(
            contentRect: NSRect(x: 0, y: 0, width: 1200, height: 800),
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered, defer: false)
        window.title = "Memory Flow"
        window.contentView = webView
        window.center()
        window.setFrameAutosaveName("MemoryFlowMainWindow")
        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)

        if ready {
            webView.load(URLRequest(url: server.url))
        } else {
            let html = "<body style='font-family:-apple-system;padding:3rem;color:#444'>"
                + "<h2>Memory Flow failed to start</h2>"
                + "<p>The local server did not come up. Try reopening the app.</p></body>"
            webView.loadHTMLString(html, baseURL: nil)
        }
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ app: NSApplication) -> Bool { true }

    func applicationWillTerminate(_ note: Notification) {
        server.stop()
    }

    // Minimal menu: app menu (Quit) + Edit (so copy/paste/select-all work in the
    // WebView). Deliberately no File>Save — the SPA owns Cmd+S.
    private func setupMenu() {
        let mainMenu = NSMenu()

        let appItem = NSMenuItem()
        mainMenu.addItem(appItem)
        let appMenu = NSMenu()
        appItem.submenu = appMenu
        appMenu.addItem(withTitle: "Hide Memory Flow", action: #selector(NSApplication.hide(_:)), keyEquivalent: "h")
        appMenu.addItem(.separator())
        appMenu.addItem(withTitle: "Quit Memory Flow", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")

        let editItem = NSMenuItem()
        mainMenu.addItem(editItem)
        let editMenu = NSMenu(title: "Edit")
        editItem.submenu = editMenu
        editMenu.addItem(withTitle: "Undo", action: Selector(("undo:")), keyEquivalent: "z")
        editMenu.addItem(withTitle: "Redo", action: Selector(("redo:")), keyEquivalent: "Z")
        editMenu.addItem(.separator())
        editMenu.addItem(withTitle: "Cut", action: #selector(NSText.cut(_:)), keyEquivalent: "x")
        editMenu.addItem(withTitle: "Copy", action: #selector(NSText.copy(_:)), keyEquivalent: "c")
        editMenu.addItem(withTitle: "Paste", action: #selector(NSText.paste(_:)), keyEquivalent: "v")
        editMenu.addItem(withTitle: "Select All", action: #selector(NSText.selectAll(_:)), keyEquivalent: "a")

        NSApp.mainMenu = mainMenu
    }
}

let app = NSApplication.shared
app.setActivationPolicy(.regular)
let delegate = AppDelegate()
app.delegate = delegate
app.run()
