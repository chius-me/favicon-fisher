import AppKit
import SwiftUI
import ImageIO
import UniformTypeIdentifiers
import FisherCore

struct LoadedIcon {
    let artifact: Artifact
    let directory: URL
    let image: NSImage?
}

@MainActor final class FisherModel: ObservableObject {
    @Published var input = ""
    @Published var busy = false
    @Published var status = "粘贴网址，捞取网站图标。"
    @Published var error: String?
    @Published var candidates: [Candidate] = []
    @Published var selectedID: String?
    @Published var loaded: [String: LoadedIcon] = [:]
    @Published var failures: Set<String> = []
    @Published var format = "original"
    @Published var pageURL = ""
    @Published var history: [HistoryItem]
    @Published var savedURL: URL?
    @Published var historyEnabled: Bool {
        didSet { historyStore.enabled = historyEnabled; history = historyStore.load() }
    }
    private let historyStore = HistoryStore()
    private var engine: EngineClient { EngineClient(allowFakeIP: UserDefaults.standard.bool(forKey: "allowFakeIP")) }
    private var task: Task<Void, Never>?
    private var generation = UUID()
    private var currentCache: URL?
    private let cacheRoot: URL
    var selection: LoadedIcon? { selectedID.flatMap { loaded[$0] } }
    var selectedCandidate: Candidate? { candidates.first { $0.id == selectedID } }
    var host: String { URL(string: pageURL)?.host ?? pageURL }

    init() {
        history = historyStore.load(); historyEnabled = historyStore.enabled
        cacheRoot = FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("dev.chius.favicon-fisher", isDirectory: true)
        try? FileManager.default.createDirectory(at: cacheRoot, withIntermediateDirectories: true,
                                                 attributes: [.posixPermissions: 0o700])
        pruneCache()
    }
    func cancel() {
        generation = UUID(); task?.cancel(); task = nil; busy = false; status = "已取消"
    }
    func pasteAndFetch() {
        guard let value = NSPasteboard.general.string(forType: .string) else { return }
        input = value.trimmingCharacters(in: .whitespacesAndNewlines); discover()
    }
    func discover() {
        let target = input.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !target.isEmpty else { return }
        cancel(); let token = generation
        busy = true; error = nil; savedURL = nil; status = "正在读取网页…"
        // Keep previous results visible until discovery succeeds.
        task = Task {
            do {
                let result: Discovery = try await engine.call("discover", url: target)
                try Task.checkCancellation()
                guard token == generation else { return }
                let directory = cacheRoot.appendingPathComponent(UUID().uuidString, isDirectory: true)
                try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true,
                                                        attributes: [.posixPermissions: 0o700])
                currentCache = directory; pageURL = result.pageURL; candidates = result.candidates
                loaded = [:]; failures = []; selectedID = nil; format = "original"
                status = "正在验证候选图标…"
                let deadline = Date().addingTimeInterval(15)
                // Load ranked batches of at most three. Stop after the first usable batch.
                for start in stride(from: 0, to: result.candidates.count, by: 3) {
                    if Date() >= deadline { break }
                    let batch = Array(result.candidates[start..<min(start + 3, result.candidates.count)])
                    let client = engine
                    let remaining = max(1, min(10_000, Int(deadline.timeIntervalSinceNow * 1000)))
                    let outcomes = await withTaskGroup(of: (String, Artifact?).self) { group in
                        for candidate in batch {
                            group.addTask {
                                let artifact: Artifact? = try? await client.call("fetch", url: candidate.url, cache: directory, timeout: remaining)
                                return (candidate.id, artifact)
                            }
                        }
                        var results: [(String, Artifact?)] = []
                        for await item in group { results.append(item) }
                        return results
                    }
                    try Task.checkCancellation()
                    guard token == generation else { return }
                    for (id, artifact) in outcomes {
                        if let artifact { install(artifact, id: id, directory: directory) }
                        else { failures.insert(id) }
                    }
                    if let first = candidates.first(where: { loaded[$0.id]?.image != nil }) ?? candidates.first(where: { loaded[$0.id] != nil }) {
                        selectedID = first.id; break
                    }
                }
                guard token == generation else { return }
                busy = false
                if selection != nil {
                    status = "已找到可用图标"; history = historyStore.record(result.pageURL)
                } else {
                    error = "未能取得有效图标。可点选其他候选重试。"; status = "验证未完成"
                }
            } catch {
                guard token == generation else { return }
                busy = false
                if !(error is CancellationError) { self.error = error.localizedDescription; status = "抓取失败" }
            }
        }
    }
    func select(_ candidate: Candidate) {
        guard !busy, let directory = currentCache else { return }
        selectedID = candidate.id; format = "original"; savedURL = nil; error = nil
        if loaded[candidate.id] != nil { return }
        busy = true; status = "正在读取候选图标…"
        let token = generation
        task = Task {
            do {
                let artifact: Artifact = try await engine.call("fetch", url: candidate.url, cache: directory, timeout: 10_000)
                try Task.checkCancellation(); guard token == generation else { return }
                install(artifact, id: candidate.id, directory: directory)
                failures.remove(candidate.id); history = historyStore.record(pageURL); status = "图标已就绪"
            } catch {
                guard token == generation else { return }
                failures.insert(candidate.id); self.error = error.localizedDescription
            }
            guard token == generation else { return }; busy = false
        }
    }
    private func install(_ artifact: Artifact, id: String, directory: URL) {
        guard (try? EngineClient.artifactURL(artifact.path, in: directory)) != nil else { failures.insert(id); return }
        var image: NSImage?
        if let path = artifact.previewPath, let url = try? EngineClient.artifactURL(path, in: directory) {
            image = Self.thumbnail(url)
        }
        // SVG is deliberately not handed to WebKit/NSImage; it stays a file-only artifact.
        loaded[id] = LoadedIcon(artifact: artifact, directory: directory, image: image)
    }
    private static func thumbnail(_ url: URL) -> NSImage? {
        guard let source = CGImageSourceCreateWithURL(url as CFURL, [kCGImageSourceShouldCache: false] as CFDictionary),
              let props = CGImageSourceCopyPropertiesAtIndex(source, 0, nil) as? [CFString: Any],
              let width = props[kCGImagePropertyPixelWidth] as? Int,
              let height = props[kCGImagePropertyPixelHeight] as? Int,
              width > 0, height > 0, width <= 4096, height <= 4096, width * height <= 16_000_000,
              let image = CGImageSourceCreateThumbnailAtIndex(source, 0, [
                kCGImageSourceCreateThumbnailFromImageAlways: true,
                kCGImageSourceThumbnailMaxPixelSize: 512,
                kCGImageSourceShouldCacheImmediately: true
              ] as CFDictionary) else { return nil }
        return NSImage(cgImage: image, size: NSSize(width: image.width, height: image.height))
    }
    func copyImage() {
        guard selection?.image != nil else { return }
        export(action: .copy)
    }
    func save() { export(action: .save) }
    private enum ExportAction { case save, copy }
    private func export(action: ExportAction) {
        guard let icon = selection, !busy else { return }
        let selectedFormat = action == .copy && icon.artifact.allowedFormats.contains("png") ? "png" : format
        let token = generation; busy = true; error = nil; status = "正在准备图标…"
        task = Task {
            do {
                let artifact: Artifact = try await engine.call("export", cache: icon.directory,
                    source: icon.artifact.path, format: selectedFormat)
                try Task.checkCancellation(); guard token == generation else { return }
                let file = try EngineClient.artifactURL(artifact.path, in: icon.directory)
                if action == .copy {
                    if artifact.format == "png" {
                        NSPasteboard.general.clearContents()
                        NSPasteboard.general.setData(try Data(contentsOf: file), forType: .png)
                    } else if let image = icon.image {
                        NSPasteboard.general.clearContents(); NSPasteboard.general.writeObjects([image])
                    }
                    status = "已复制图片"
                } else {
                    let panel = NSSavePanel()
                    panel.nameFieldStringValue = "\(host.isEmpty ? "favicon" : host).\(artifact.format)"
                    if let type = UTType(filenameExtension: artifact.format) { panel.allowedContentTypes = [type] }
                    NSApp.activate(ignoringOtherApps: true)
                    if panel.runModal() == .OK, let destination = panel.url {
                        // NSSavePanel has already confirmed any overwrite; atomic write preserves the old file on failure.
                        try Data(contentsOf: file).write(to: destination, options: .atomic)
                        savedURL = destination; status = "已保存"
                    } else { status = "已取消保存" }
                }
            } catch {
                guard token == generation else { return }; self.error = error.localizedDescription
            }
            guard token == generation else { return }; busy = false
        }
    }
    func dragProvider() -> NSItemProvider {
        guard let icon = selection, let file = try? EngineClient.artifactURL(icon.artifact.path, in: icon.directory) else { return NSItemProvider() }
        // This is the original file, intentionally independent of the save-format selector.
        let provider = NSItemProvider(contentsOf: file) ?? NSItemProvider()
        provider.suggestedName = "\(host).\(icon.artifact.format)"
        return provider
    }
    func copyLink() {
        guard let url = selectedCandidate?.url else { return }
        NSPasteboard.general.clearContents(); NSPasteboard.general.setString(url, forType: .string); status = "已复制来源链接"
    }
    func clearHistory() { historyStore.clear(); history = [] }
    func revealSaved() { if let savedURL { NSWorkspace.shared.activateFileViewerSelecting([savedURL]) } }

    private func pruneCache() {
        // Only at launch: never invalidate a live drag or an in-flight operation.
        let fm = FileManager.default
        let dirs = (try? fm.contentsOfDirectory(at: cacheRoot, includingPropertiesForKeys: [.contentModificationDateKey, .isDirectoryKey, .isSymbolicLinkKey])) ?? []
        var entries: [(URL, Date, Int)] = []
        for directory in dirs {
            guard let values = try? directory.resourceValues(forKeys: [.contentModificationDateKey, .isDirectoryKey, .isSymbolicLinkKey]),
                  values.isDirectory == true, values.isSymbolicLink != true else { continue }
            let files = (try? fm.contentsOfDirectory(at: directory, includingPropertiesForKeys: [.fileSizeKey])) ?? []
            let size = files.reduce(0) { $0 + ((try? $1.resourceValues(forKeys: [.fileSizeKey]).fileSize) ?? 0) }
            entries.append((directory, values.contentModificationDate ?? .distantPast, size))
        }
        var total = entries.reduce(0) { $0 + $1.2 }
        for entry in entries.sorted(by: { $0.1 < $1.1 }) {
            if total > 100 * 1024 * 1024 || Date().timeIntervalSince(entry.1) > 7 * 86400 {
                try? fm.removeItem(at: entry.0); total -= entry.2
            }
        }
    }
}
