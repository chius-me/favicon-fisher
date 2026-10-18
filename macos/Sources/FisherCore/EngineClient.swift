import Foundation
import Darwin

public struct Candidate: Codable, Identifiable, Sendable {
    public let id: String
    public let url: String
    public let source: String
    public let declaredSizes: String
    enum CodingKeys: String, CodingKey { case id, url, source; case declaredSizes = "declared_sizes" }
}
public struct Discovery: Decodable, Sendable {
    public let pageURL: String
    public let candidates: [Candidate]
    enum CodingKeys: String, CodingKey { case pageURL = "page_url"; case candidates }
}
public struct Artifact: Decodable, Sendable {
    public let path: String
    public let previewPath: String?
    public let format: String
    public let width: Int
    public let height: Int
    public let bytes: Int
    public let allowedFormats: [String]
    enum CodingKeys: String, CodingKey {
        case path, format, width, height, bytes
        case previewPath = "preview_path"; case allowedFormats = "allowed_formats"
    }
}
public struct Capabilities: Decodable, Sendable {
    public let engineVersion: String
    enum CodingKeys: String, CodingKey { case engineVersion = "engine_version" }
}
private struct Request: Encodable {
    let protocol_version = 1
    let id: String
    let op: String
    let url: String?
    let cache_dir: String?
    let source: String?
    let format: String?
    let timeout_ms: Int
    let allow_fake_ip: Bool
}
private struct Envelope<T: Decodable>: Decodable {
    let protocol_version: Int
    let id: String
    let ok: Bool
    let result: T?
    let error: EngineFailure?
}
public struct EngineFailure: Decodable, LocalizedError, Sendable {
    public let code: String
    public let message: String
    public var errorDescription: String? {
        switch code {
        case "NETWORK_TIMEOUT": return "网站响应超时，请重试。"
        case "CANCELLED": return "已取消。"
        case "INVALID_URL": return "请输入有效的网址或域名。"
        case "PROTOCOL_MISMATCH": return "应用与引擎版本不一致，请重新安装。"
        default: return message
        }
    }
}
public enum ClientError: LocalizedError {
    case missingEngine, invalidResponse, unsafePath, failed, timeout
    public var errorDescription: String? {
        switch self {
        case .missingEngine: return "应用内置引擎缺失，请重新安装。"
        case .invalidResponse: return "引擎返回了无效结果。"
        case .unsafePath: return "缓存文件无效，请重新抓取。"
        case .failed: return "引擎未能完成任务，请重试。"
        case .timeout: return "操作超时，请重试。"
        }
    }
}

public struct EngineClient: Sendable {
    private let executable: URL
    private let allowFakeIP: Bool
    public init(allowFakeIP: Bool = false, executable: URL = Bundle.main.bundleURL.appendingPathComponent("Contents/Helpers/fvf-engine")) {
        self.executable = executable
        self.allowFakeIP = allowFakeIP
    }
    public func call<T: Decodable & Sendable>(_ operation: String, url: String? = nil, cache: URL? = nil,
                                             source: String? = nil, format: String? = nil,
                                             timeout: Int = 15_000) async throws -> T {
        guard FileManager.default.isExecutableFile(atPath: executable.path) else { throw ClientError.missingEngine }
        let id = UUID().uuidString
        let request = Request(id: id, op: operation, url: url, cache_dir: cache?.path,
                              source: source, format: format, timeout_ms: timeout, allow_fake_ip: allowFakeIP)
        var input = try JSONEncoder().encode(request); input.append(10)
        let runner = EngineProcess()
        let output = try await withTaskCancellationHandler {
            try await Task.detached(priority: .userInitiated) {
                try runner.execute(executable: executable, input: input, timeout: Double(timeout) / 1000 + 2)
            }.value
        } onCancel: { runner.cancel() }
        try Task.checkCancellation()
        let envelope = try JSONDecoder().decode(Envelope<T>.self, from: output)
        guard envelope.protocol_version == 1, envelope.id == id else { throw ClientError.invalidResponse }
        guard envelope.ok, let result = envelope.result else { throw envelope.error ?? EngineFailure(code: "FAILED", message: "操作失败。") }
        return result
    }

    public static func artifactURL(_ name: String, in directory: URL) throws -> URL {
        guard !name.isEmpty, name != ".", name != "..", !name.contains("/"), !name.contains("\\") else { throw ClientError.unsafePath }
        let root = directory.resolvingSymlinksInPath().standardizedFileURL
        let url = root.appendingPathComponent(name)
        let values = try url.resourceValues(forKeys: [.isRegularFileKey, .isSymbolicLinkKey])
        guard values.isRegularFile == true, values.isSymbolicLink != true,
              url.resolvingSymlinksInPath().deletingLastPathComponent() == root else { throw ClientError.unsafePath }
        return url
    }
}

// One process per request. Both pipes are drained concurrently with hard byte limits.
private final class EngineProcess: @unchecked Sendable {
    private let lock = NSLock()
    private var process: Process?
    private var cancelled = false
    private var timedOut = false
    func cancel(timeout: Bool = false) {
        lock.lock(); cancelled = true; timedOut = timedOut || timeout
        let running = process
        if running?.isRunning == true { running?.terminate() }
        lock.unlock()
        if let running {
            DispatchQueue.global().asyncAfter(deadline: .now() + 0.5) {
                if running.isRunning { kill(running.processIdentifier, SIGKILL) }
            }
        }
    }
    func execute(executable: URL, input: Data, timeout: TimeInterval) throws -> Data {
        let task = Process(), stdin = Pipe(), stdout = Pipe(), stderr = Pipe()
        task.executableURL = executable
        task.standardInput = stdin; task.standardOutput = stdout; task.standardError = stderr
        task.environment = ["PATH": "/usr/bin:/bin", "LANG": "en_US.UTF-8"]
        lock.lock()
        if cancelled { lock.unlock(); throw CancellationError() }
        process = task
        do { try task.run() } catch { process = nil; lock.unlock(); throw error }
        lock.unlock()
        let watchdog = DispatchWorkItem { [weak self] in self?.cancel(timeout: true) }
        DispatchQueue.global().asyncAfter(deadline: .now() + timeout, execute: watchdog)
        let out = Collector(), err = Collector()
        out.start(stdout.fileHandleForReading, limit: 1_048_576) { self.cancel() }
        err.start(stderr.fileHandleForReading, limit: 65_536) { self.cancel() }
        do { try stdin.fileHandleForWriting.write(contentsOf: input) } catch { cancel() }
        task.waitUntilExit()
        watchdog.cancel()
        try? stdin.fileHandleForWriting.close()
        let bytes = out.finish(); _ = err.finish()
        lock.lock(); let wasCancelled = cancelled, expired = timedOut; process = nil; lock.unlock()
        if expired { throw ClientError.timeout }
        if wasCancelled { throw CancellationError() }
        guard task.terminationStatus == 0, let bytes else { throw ClientError.failed }
        return bytes
    }
}
private final class Collector: @unchecked Sendable {
    private let finished = DispatchSemaphore(value: 0)
    private var data: Data?
    func start(_ handle: FileHandle, limit: Int, overflow: @escaping @Sendable () -> Void) {
        DispatchQueue.global().async {
            defer { try? handle.close(); self.finished.signal() }
            var buffer = Data()
            do {
                while let chunk = try handle.read(upToCount: 16_384), !chunk.isEmpty {
                    guard buffer.count + chunk.count <= limit else { overflow(); return }
                    buffer.append(chunk)
                }
                self.data = buffer
            } catch { overflow() }
        }
    }
    func finish() -> Data? { finished.wait(); return data }
}
