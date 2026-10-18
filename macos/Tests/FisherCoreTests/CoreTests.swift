import XCTest
@testable import FisherCore

final class CoreTests: XCTestCase {
    func testHistoryRemovesSecretsDeduplicatesAndCaps() {
        let suite = "fvf-tests-\(UUID().uuidString)"
        let defaults = UserDefaults(suiteName: suite)!
        defer { defaults.removePersistentDomain(forName: suite) }
        let store = HistoryStore(defaults: defaults)
        store.record("https://user:password@example.com/path?token=secret#private")
        XCTAssertEqual(store.load().first?.url, "https://example.com/path")
        store.record("https://example.com/path?different=1")
        XCTAssertEqual(store.load().count, 1)
        for i in 0..<30 { store.record("https://example.com/\(i)") }
        XCTAssertEqual(store.load().count, 20)
        store.enabled = false
        XCTAssertTrue(store.load().isEmpty)
        store.enabled = true
        XCTAssertTrue(store.load().isEmpty)
    }
    func testArtifactCannotEscapeCache() throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: directory) }
        let valid = directory.appendingPathComponent("image.png")
        try Data([1, 2, 3]).write(to: valid)
        XCTAssertEqual(try EngineClient.artifactURL("image.png", in: directory).lastPathComponent, "image.png")
        for path in ["../secret", "/etc/hosts", ".", "", "sub/image.png"] {
            XCTAssertThrowsError(try EngineClient.artifactURL(path, in: directory))
        }
        try FileManager.default.createSymbolicLink(at: directory.appendingPathComponent("link"), withDestinationURL: valid)
        XCTAssertThrowsError(try EngineClient.artifactURL("link", in: directory))
    }
    func testBundledEngineProtocol() async throws {
        guard let path = ProcessInfo.processInfo.environment["FVF_TEST_ENGINE"] else { throw XCTSkip("Set FVF_TEST_ENGINE to run helper integration tests") }
        let client = EngineClient(executable: URL(fileURLWithPath: path))
        let capabilities: Capabilities = try await client.call("capabilities")
        XCTAssertFalse(capabilities.engineVersion.isEmpty)
        do {
            let _: Discovery = try await client.call("discover", url: "http://[::1]/")
            XCTFail("Private addresses must be rejected")
        } catch let failure as EngineFailure { XCTAssertEqual(failure.code, "FETCH_FAILED") }
    }

    func testCancellationTerminatesHelperPromptly() async throws {
        let script = FileManager.default.temporaryDirectory.appendingPathComponent("fvf-cancel-\(UUID().uuidString)")
        try Data("#!/bin/sh\nread request\nexec /bin/sleep 30\n".utf8).write(to: script)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        defer { try? FileManager.default.removeItem(at: script) }
        let client = EngineClient(executable: script)
        let task = Task { let _: Capabilities = try await client.call("capabilities") }
        try await Task.sleep(nanoseconds: 100_000_000)
        let started = Date(); task.cancel()
        do { try await task.value; XCTFail("Expected cancellation") } catch { XCTAssertTrue(error is CancellationError) }
        XCTAssertLessThan(Date().timeIntervalSince(started), 2)
    }

    func testWrongResponseIDIsRejected() async throws {
        let script = FileManager.default.temporaryDirectory.appendingPathComponent("fvf-invalid-\(UUID().uuidString)")
        let body = "#!/bin/sh\nread request\necho '{\"protocol_version\":1,\"id\":\"wrong\",\"ok\":true,\"result\":{\"engine_version\":\"x\"}}'\n"
        try Data(body.utf8).write(to: script)
        try FileManager.default.setAttributes([.posixPermissions: 0o700], ofItemAtPath: script.path)
        defer { try? FileManager.default.removeItem(at: script) }
        do {
            let _: Capabilities = try await EngineClient(executable: script).call("capabilities")
            XCTFail("Expected invalid response")
        } catch { XCTAssertTrue(error is ClientError) }
    }
}
