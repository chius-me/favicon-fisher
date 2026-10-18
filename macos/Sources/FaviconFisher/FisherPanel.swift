import SwiftUI
import ServiceManagement
import FisherCore

struct FisherPanel: View {
    @ObservedObject var model: FisherModel
    @FocusState private var inputFocused: Bool
    @State private var darkPreview = false
    var body: some View {
        ScrollView {
        VStack(spacing: 14) {
            HStack {
                Label { Text("Favicon Fisher") } icon: { Image(nsImage: WebsiteIcon.image) }.font(.headline)
                Spacer()
                SettingsLink { Image(systemName: "gearshape") }.buttonStyle(.plain).help("设置")
            }
            HStack {
                TextField("网址或域名", text: $model.input)
                    .textFieldStyle(.roundedBorder).focused($inputFocused).onSubmit { model.discover() }
                    .accessibilityIdentifier("websiteInput")
                Button("抓取") { model.discover() }.disabled(model.input.trimmingCharacters(in: .whitespaces).isEmpty)
            }
            HStack {
                Button("粘贴并抓取", systemImage: "doc.on.clipboard") { model.pasteAndFetch() }.font(.caption)
                Spacer()
                if model.busy { ProgressView().controlSize(.small); Button("取消") { model.cancel() } }
            }
            if let icon = model.selection {
                VStack(spacing: 9) {
                    ZStack {
                        Checkerboard(dark: darkPreview)
                        if let image = icon.image {
                            Image(nsImage: image).resizable().interpolation(.high).scaledToFit().padding(22)
                        } else {
                            VStack { Image(systemName: "doc.richtext").font(.largeTitle); Text("\(icon.artifact.format.uppercased()) 原文件"); Text("此格式暂不预览").font(.caption).foregroundStyle(.secondary) }
                        }
                    }.frame(height: 146).clipShape(RoundedRectangle(cornerRadius: 12))
                        .onDrag { model.dragProvider() }
                        .accessibilityLabel("图标预览，拖动导出原文件")
                    HStack {
                        Text(model.host).font(.subheadline.weight(.medium)).lineLimit(1)
                        Spacer()
                        Button { darkPreview.toggle() } label: { Image(systemName: darkPreview ? "sun.max" : "moon") }.buttonStyle(.plain).help("切换预览背景")
                    }
                    HStack {
                        Text(icon.artifact.format.uppercased())
                        if icon.artifact.width > 0 { Text("\(icon.artifact.width) × \(icon.artifact.height)") }
                        Text(ByteCountFormatter.string(fromByteCount: Int64(icon.artifact.bytes), countStyle: .file))
                        Spacer(); Text("已验证").foregroundStyle(.green)
                    }.font(.caption).foregroundStyle(.secondary)
                }
            } else if model.candidates.isEmpty {
                VStack(spacing: 8) {
                    Image(nsImage: WebsiteIcon.image).resizable().scaledToFit().frame(width: 40, height: 40).foregroundStyle(.tint)
                    Text("一个网址，一枚好图标").font(.headline)
                    Text("预览、复制或拖到你的项目里。\n由本机直接抓取，无需登录。")
                        .font(.caption).foregroundStyle(.secondary).multilineTextAlignment(.center)
                }.frame(height: 160)
            }
            if !model.candidates.isEmpty {
                ScrollView(.horizontal) {
                    HStack(spacing: 8) {
                        ForEach(model.candidates) { candidate in
                            Button { model.select(candidate) } label: {
                                VStack(spacing: 4) {
                                    if let image = model.loaded[candidate.id]?.image {
                                        Image(nsImage: image).resizable().scaledToFit().frame(width: 28, height: 28)
                                    } else { Image(systemName: model.failures.contains(candidate.id) ? "exclamationmark.triangle" : "photo").frame(width: 28, height: 28) }
                                    Text(model.loaded[candidate.id]?.artifact.format.uppercased() ?? candidate.source).font(.system(size: 9)).lineLimit(1)
                                }.frame(width: 54, height: 48).padding(4)
                                    .background(model.selectedID == candidate.id ? Color.accentColor.opacity(0.16) : Color.secondary.opacity(0.07))
                                    .clipShape(RoundedRectangle(cornerRadius: 8))
                            }.buttonStyle(.plain).disabled(model.busy).help(candidate.url)
                        }
                    }
                }.frame(height: 62)
            }
            if let icon = model.selection {
                HStack {
                    Picker("保存格式", selection: $model.format) {
                        ForEach(icon.artifact.allowedFormats, id: \.self) { format in Text(format == "original" ? "原格式" : format.uppercased()).tag(format) }
                    }.frame(width: 170)
                    Spacer()
                    Button("来源", systemImage: "link") { model.copyLink() }.labelStyle(.iconOnly).help("复制来源链接")
                }
                HStack {
                    Button("复制图片", systemImage: "doc.on.doc") { model.copyImage() }.disabled(icon.image == nil || model.busy)
                    Spacer()
                    Button("保存…", systemImage: "square.and.arrow.down") { model.save() }.buttonStyle(.borderedProminent).disabled(model.busy)
                }
                Text("拖动预览可导出原文件；保存可选择转换格式。") .font(.system(size: 10)).foregroundStyle(.secondary)
            }
            if let error = model.error { Text(error).font(.caption).foregroundStyle(.red).frame(maxWidth: .infinity, alignment: .leading).textSelection(.enabled) }
            HStack {
                Text(model.status).font(.caption).foregroundStyle(.secondary).lineLimit(1)
                Spacer()
                if model.savedURL != nil { Button("在 Finder 中显示") { model.revealSaved() }.font(.caption) }
            }
            Divider()
            HStack {
                Menu("最近使用") {
                    if model.history.isEmpty { Text("暂无记录") }
                    ForEach(model.history) { item in
                        Button(item.url) { model.input = item.url; model.discover() }
                    }
                    Divider(); Button("清空记录") { model.clearHistory() }
                }.menuStyle(.borderlessButton).fixedSize()
                Spacer()
                Button("关于") { NSApp.orderFrontStandardAboutPanel() }.buttonStyle(.plain)
                Button("退出") { model.cancel(); NSApp.terminate(nil) }.buttonStyle(.plain)
            }.font(.caption)
        }
        .padding(18)
        }
        .frame(width: 400, height: model.selection == nil && model.candidates.isEmpty ? 410 : 560)
        .onAppear { inputFocused = true }
        .onReceive(NotificationCenter.default.publisher(for: NSApplication.willTerminateNotification)) { _ in model.cancel() }
    }
}
private struct Checkerboard: View {
    let dark: Bool
    var body: some View {
        Canvas { context, size in
            let base = dark ? Color(white: 0.15) : Color(white: 0.97)
            context.fill(Path(CGRect(origin: .zero, size: size)), with: .color(base))
            for row in 0..<Int(size.height / 12 + 1) {
                for column in 0..<Int(size.width / 12 + 1) where (row + column) % 2 == 0 {
                    context.fill(Path(CGRect(x: column * 12, y: row * 12, width: 12, height: 12)), with: .color(dark ? .white.opacity(0.05) : .black.opacity(0.04)))
                }
            }
        }
    }
}
struct FisherSettings: View {
    @ObservedObject var model: FisherModel
    @State private var loginEnabled = SMAppService.mainApp.status == .enabled
    @State private var loginError: String?
    @AppStorage("allowFakeIP") private var allowFakeIP = false
    var body: some View {
        Form {
            Toggle("登录时启动", isOn: Binding(get: { loginEnabled }, set: { value in
                do {
                    if value { try SMAppService.mainApp.register() } else { try SMAppService.mainApp.unregister() }
                    loginEnabled = SMAppService.mainApp.status == .enabled
                    loginError = SMAppService.mainApp.status == .requiresApproval ? "请在系统设置的登录项中允许启动。" : nil
                } catch { loginError = error.localizedDescription }
            }))
            if let loginError { Text(loginError).font(.caption).foregroundStyle(.secondary) }
            Toggle("保留最近 20 个网址", isOn: $model.historyEnabled)
            Text("历史保存在本机，自动移除查询参数和片段。关闭后清空历史。") .font(.caption).foregroundStyle(.secondary)
            Button("清空历史") { model.clearHistory() }
            Divider()
            Toggle("兼容 TUN/Fake-IP 代理", isOn: $allowFakeIP)
            Text("仅在使用可信的本机 TUN 代理、且公网网址被拦截时开启。允许代理映射地址，仍阻止其他内网地址。") .font(.caption).foregroundStyle(.secondary)
            Text("本机直接抓取；不后台监听剪贴板。")
            Text("SVG 仅保存原文件；GIF 转换使用首帧。") .font(.caption).foregroundStyle(.secondary)
            Link("版本与更新", destination: URL(string: "https://github.com/chius-me/favicon-fisher/releases")!)
            Text("Favicon Fisher \(Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "dev")") .font(.caption).foregroundStyle(.secondary)
        }.padding(24).frame(width: 420)
            .onAppear { loginEnabled = SMAppService.mainApp.status == .enabled }
    }
}
