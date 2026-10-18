// A browser window with its favicon highlighted: the same mark as WebsiteIcon.swift.
import AppKit
import Foundation

let destination = URL(fileURLWithPath: CommandLine.arguments[1], isDirectory: true)
try FileManager.default.createDirectory(at: destination, withIntermediateDirectories: true)
for size in [16, 32, 128, 256, 512] {
    for scale in [1, 2] {
        let pixels = size * scale
        let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: pixels, pixelsHigh: pixels,
                                  bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                                  colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
        NSGraphicsContext.saveGraphicsState()
        NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)
        let transform = AffineTransform(scale: CGFloat(pixels) / 64)
        (transform as NSAffineTransform).concat()
        let background = NSBezierPath(roundedRect: NSRect(x: 4, y: 4, width: 56, height: 56), xRadius: 13, yRadius: 13)
        NSColor(calibratedRed: 0.10, green: 0.30, blue: 0.34, alpha: 1).setFill()
        background.fill()
        NSColor.white.setStroke()
        let window = NSBezierPath(roundedRect: NSRect(x: 14, y: 18, width: 36, height: 28), xRadius: 4, yRadius: 4)
        window.lineWidth = 3
        window.stroke()
        let divider = NSBezierPath()
        divider.lineWidth = 3
        divider.move(to: NSPoint(x: 14, y: 32))
        divider.line(to: NSPoint(x: 50, y: 32))
        divider.stroke()
        NSColor.white.setFill()
        NSBezierPath(roundedRect: NSRect(x: 20, y: 36, width: 6, height: 6), xRadius: 1, yRadius: 1).fill()
        NSGraphicsContext.restoreGraphicsState()
        let suffix = scale == 2 ? "@2x" : ""
        try rep.representation(using: .png, properties: [:])!.write(to: destination.appendingPathComponent("icon_\(size)x\(size)\(suffix).png"))
    }
}
