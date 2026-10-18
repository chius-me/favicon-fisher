// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "FaviconFisher",
    platforms: [.macOS(.v14)],
    products: [.executable(name: "FaviconFisher", targets: ["FaviconFisher"])],
    targets: [
        .target(name: "FisherCore"),
        .executableTarget(name: "FaviconFisher", dependencies: ["FisherCore"]),
        .testTarget(name: "FisherCoreTests", dependencies: ["FisherCore"])
    ]
)
