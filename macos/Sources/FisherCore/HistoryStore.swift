import Foundation

public struct HistoryItem: Codable, Identifiable, Equatable, Sendable {
    public var id: String { url }
    public let url: String
    public let date: Date
}
public struct HistoryStore {
    private let defaults: UserDefaults
    public init(defaults: UserDefaults = .standard) { self.defaults = defaults }
    public var enabled: Bool {
        get { defaults.object(forKey: "historyEnabled") as? Bool ?? true }
        nonmutating set { defaults.set(newValue, forKey: "historyEnabled"); if !newValue { clear() } }
    }
    public func load() -> [HistoryItem] {
        guard enabled, let data = defaults.data(forKey: "recentSites") else { return [] }
        return (try? JSONDecoder().decode([HistoryItem].self, from: data)) ?? []
    }
    @discardableResult public func record(_ raw: String) -> [HistoryItem] {
        guard enabled, var parts = URLComponents(string: raw),
              ["https", "http"].contains(parts.scheme ?? ""), parts.host != nil else { return load() }
        parts.query = nil; parts.fragment = nil; parts.user = nil; parts.password = nil
        guard let clean = parts.string else { return load() }
        var items = load().filter { $0.url != clean }
        items.insert(HistoryItem(url: clean, date: Date()), at: 0)
        items = Array(items.prefix(20))
        defaults.set(try? JSONEncoder().encode(items), forKey: "recentSites")
        return items
    }
    public func clear() { defaults.removeObject(forKey: "recentSites") }
}
