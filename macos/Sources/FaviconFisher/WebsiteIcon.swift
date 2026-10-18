import AppKit

enum WebsiteIcon {
    // Browser frame + one favicon; deliberately omit traffic lights and page detail.
    // Optical sizing keeps the favicon legible at the menu bar's 18-point size.
    static let image: NSImage = {
        let image = NSImage(size: NSSize(width: 18, height: 18), flipped: false) { _ in
            NSColor.black.setStroke()
            let window = NSBezierPath(roundedRect: NSRect(x: 1.5, y: 2.5, width: 15, height: 13), xRadius: 2, yRadius: 2)
            window.lineWidth = 1.5
            window.stroke()
            let divider = NSBezierPath()
            divider.lineWidth = 1.5
            divider.move(to: NSPoint(x: 1.5, y: 9.5))
            divider.line(to: NSPoint(x: 16.5, y: 9.5))
            divider.stroke()
            NSColor.black.setFill()
            NSBezierPath(roundedRect: NSRect(x: 4, y: 11, width: 3, height: 3), xRadius: 0.5, yRadius: 0.5).fill()
            return true
        }
        image.isTemplate = true
        image.accessibilityDescription = "Favicon Fisher"
        return image
    }()
}
