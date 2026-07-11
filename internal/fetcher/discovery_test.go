package fetcher

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeInputURLAddsHTTPSWhenMissing(t *testing.T) {
	normalized, err := NormalizeInputURL("example.com")
	if err != nil {
		t.Fatalf("NormalizeInputURL returned error: %v", err)
	}

	if normalized != "https://example.com" {
		t.Fatalf("expected https://example.com, got %q", normalized)
	}
}

func TestNormalizeInputURLKeepsExistingScheme(t *testing.T) {
	normalized, err := NormalizeInputURL("http://example.com")
	if err != nil {
		t.Fatalf("NormalizeInputURL returned error: %v", err)
	}

	if normalized != "http://example.com" {
		t.Fatalf("expected http://example.com, got %q", normalized)
	}
}

func TestNormalizeInputURLRejectsInvalidInput(t *testing.T) {
	_, err := NormalizeInputURL("://bad")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}

func TestDiscoverCandidatesExtractsAndResolvesLinkIcons(t *testing.T) {
	html := strings.NewReader(`
		<html><head>
			<link rel="icon" href="/favicon-32.png" sizes="32x32" type="image/png">
			<link rel="apple-touch-icon" href="https://cdn.example.com/apple.png">
		</head></html>
	`)

	candidates, err := DiscoverCandidates("https://example.com/blog/post", html)
	if err != nil {
		t.Fatalf("DiscoverCandidates returned error: %v", err)
	}

	if len(candidates) < 3 {
		t.Fatalf("expected at least 3 candidates, got %d", len(candidates))
	}

	if candidates[0].URL != "https://example.com/favicon-32.png" {
		t.Fatalf("expected resolved relative icon URL, got %q", candidates[0].URL)
	}

	foundApple := false
	for _, candidate := range candidates {
		if candidate.URL == "https://cdn.example.com/apple.png" && candidate.Rel == "apple-touch-icon" {
			foundApple = true
			break
		}
	}
	if !foundApple {
		t.Fatal("expected apple-touch-icon candidate to be included")
	}
}

func TestDiscoverCandidatesAddsFaviconICOFallback(t *testing.T) {
	html := strings.NewReader(`<html><head></head></html>`)

	candidates, err := DiscoverCandidates("https://example.com/docs", html)
	if err != nil {
		t.Fatalf("DiscoverCandidates returned error: %v", err)
	}

	foundFallback := false
	for _, candidate := range candidates {
		if candidate.URL == "https://example.com/favicon.ico" && candidate.Rel == "fallback" {
			foundFallback = true
			break
		}
	}
	if !foundFallback {
		t.Fatal("expected /favicon.ico fallback candidate")
	}
}

func TestBestCandidatePrefersStandardIconBeforeFallback(t *testing.T) {
	best, err := BestCandidate([]Candidate{
		{URL: "https://example.com/favicon.ico", Rel: "fallback", Priority: 100},
		{URL: "https://example.com/icon-32.png", Rel: "icon", Sizes: "32x32", Priority: 10},
	})
	if err != nil {
		t.Fatalf("BestCandidate returned error: %v", err)
	}

	if best.URL != "https://example.com/icon-32.png" {
		t.Fatalf("expected standard icon candidate, got %q", best.URL)
	}
}

func TestDiscoverCandidatesGoldenFixture(t *testing.T) {
	// Shared golden HTML used to keep Go discovery aligned with documented behavior.
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "sample.html"))
	if err != nil {
		t.Fatalf("read golden fixture: %v", err)
	}

	candidates, err := DiscoverCandidates("https://example.com/blog/post", strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("DiscoverCandidates: %v", err)
	}

	want := map[string]bool{
		"https://example.com/favicon-32.png":     false,
		"https://cdn.example.com/apple.png":      false,
		"https://example.com/safari.svg":         false,
		"https://example.com/favicon.ico":        false,
	}
	for _, c := range candidates {
		if _, ok := want[c.URL]; ok {
			want[c.URL] = true
		}
	}
	for url, found := range want {
		if !found {
			t.Errorf("missing candidate %s", url)
		}
	}

	if href := FindManifestHref(strings.NewReader(string(data))); href != "/site.webmanifest" {
		t.Fatalf("expected manifest href /site.webmanifest, got %q", href)
	}
}

func TestResolveURLRejectsNonHTTPSchemes(t *testing.T) {
	base, _ := url.Parse("https://example.com/")
	if got := resolveURL(base, "javascript:alert(1)"); got != "" {
		t.Fatalf("expected empty for javascript URL, got %q", got)
	}
	if got := resolveURL(base, "data:text/html,x"); got != "" {
		t.Fatalf("expected empty for data URL, got %q", got)
	}
}
