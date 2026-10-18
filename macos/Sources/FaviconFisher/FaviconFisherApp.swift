import SwiftUI
import AppKit

@main
struct FaviconFisherApp: App {
    @StateObject private var model = FisherModel()
    var body: some Scene {
        MenuBarExtra {
            FisherPanel(model: model)
        } label: {
            Image(nsImage: WebsiteIcon.image)
                .accessibilityLabel(model.busy ? "Favicon Fisher，正在抓取" : "Favicon Fisher")
                .help(model.busy ? "正在抓取图标…" : "Favicon Fisher")
        }
        .menuBarExtraStyle(.window)
        Settings { FisherSettings(model: model) }
    }
}
