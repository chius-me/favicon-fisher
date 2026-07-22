package fetcher

import (
	"strings"
	"testing"
)

func FuzzNormalizeInputURL(f *testing.F) {
	f.Add("example.com")
	f.Add("https://example.com/path")
	f.Add("http://[::1]/")
	f.Add("://bad")
	f.Add("")
	f.Add("  github.com  ")
	f.Fuzz(func(t *testing.T, raw string) {
		_, _ = NormalizeInputURL(raw)
	})
}

func FuzzDiscoverCandidates(f *testing.F) {
	f.Add(`<link rel="icon" href="/favicon.png">`)
	f.Add(`<link rel=icon href=/favicon.ico>`)
	f.Add(`<html><head></head></html>`)
	f.Add(strings.Repeat(`<link rel="icon" href="/i.png">`, 100))
	f.Fuzz(func(t *testing.T, html string) {
		// Must not panic on arbitrary HTML; errors are fine.
		_, _ = DiscoverCandidates("https://example.com/", strings.NewReader(html))
	})
}

func FuzzSizeScore(f *testing.F) {
	f.Add("32x32")
	f.Add("any")
	f.Add("999999x999999")
	f.Add("0x0 16x16")
	f.Fuzz(func(t *testing.T, sizes string) {
		_ = sizeScore(sizes)
	})
}

func FuzzSafeFilename(f *testing.F) {
	f.Add("example.com", ".png")
	f.Add("../etc/passwd", ".ico")
	f.Add("a/b\\c:d", ".svg")
	f.Fuzz(func(t *testing.T, host, ext string) {
		name := safeFilename(host, ext)
		if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "\x00") {
			t.Fatalf("unsafe filename: %q", name)
		}
	})
}
