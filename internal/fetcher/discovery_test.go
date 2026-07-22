package fetcher

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chius-me/favicon-fisher/internal/security"
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

func TestDiscoverCandidatesCapsHTMLIcons(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<html><head>`)
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&b, `<link rel="icon" href="/icon-%d.png">`, i)
	}
	b.WriteString(`</head></html>`)

	candidates, err := DiscoverCandidates("https://example.com/", strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("DiscoverCandidates: %v", err)
	}
	// MaxHTMLIconCandidates + 1 fallback
	if len(candidates) > security.MaxHTMLIconCandidates+1 {
		t.Fatalf("expected at most %d candidates, got %d", security.MaxHTMLIconCandidates+1, len(candidates))
	}
	htmlIcons := 0
	for _, c := range candidates {
		if c.Rel != "fallback" {
			htmlIcons++
		}
	}
	if htmlIcons > security.MaxHTMLIconCandidates {
		t.Fatalf("expected at most %d HTML icons, got %d", security.MaxHTMLIconCandidates, htmlIcons)
	}
}

func TestSizeScoreCapsHugeDimensions(t *testing.T) {
	// Should not overflow; huge declared sizes clamp.
	score := sizeScore("999999x999999")
	if score <= 0 {
		t.Fatalf("expected positive score, got %d", score)
	}
	// 16384*16384
	if score > 16384*16384 {
		t.Fatalf("expected clamped score, got %d", score)
	}
}

func TestDiscoverFromHTMLReturnsManifestOnce(t *testing.T) {
	html := strings.NewReader(`
		<html><head>
			<link rel="icon" href="/a.png">
			<link rel="manifest" href="/site.webmanifest">
		</head></html>
	`)
	result, err := DiscoverFromHTML("https://example.com/", html)
	if err != nil {
		t.Fatalf("DiscoverFromHTML: %v", err)
	}
	if result.ManifestHref != "/site.webmanifest" {
		t.Fatalf("expected manifest href, got %q", result.ManifestHref)
	}
	if len(result.Candidates) < 2 {
		t.Fatalf("expected icon + fallback, got %d", len(result.Candidates))
	}
}

func TestDiscoveryGoldenContractJSON(t *testing.T) {
	// Language-agnostic fixture: shared with Worker parity tests under testdata/golden/.
	type contract struct {
		PageURL       string            `json:"page_url"`
		HTMLFile      string            `json:"html_file"`
		ManifestHref  string            `json:"manifest_href"`
		ExpectedURLs  []string          `json:"expected_urls"`
		ExpectedRels  map[string]string `json:"expected_rels"`
		BestURL       string            `json:"best_url"`
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "discovery.json"))
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var c contract
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	html, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", c.HTMLFile))
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	result, err := DiscoverFromHTML(c.PageURL, strings.NewReader(string(html)))
	if err != nil {
		t.Fatalf("DiscoverFromHTML: %v", err)
	}
	if result.ManifestHref != c.ManifestHref {
		t.Fatalf("manifest href: want %q got %q", c.ManifestHref, result.ManifestHref)
	}
	found := map[string]Candidate{}
	for _, cand := range result.Candidates {
		found[cand.URL] = cand
	}
	for _, wantURL := range c.ExpectedURLs {
		cand, ok := found[wantURL]
		if !ok {
			t.Errorf("missing candidate %s", wantURL)
			continue
		}
		if wantRel := c.ExpectedRels[wantURL]; wantRel != "" && cand.Rel != wantRel {
			t.Errorf("rel for %s: want %q got %q", wantURL, wantRel, cand.Rel)
		}
	}
	best, err := BestCandidate(result.Candidates)
	if err != nil {
		t.Fatalf("BestCandidate: %v", err)
	}
	if best.URL != c.BestURL {
		t.Fatalf("best: want %q got %q", c.BestURL, best.URL)
	}
}

func TestDiscoverUnquotedAttributes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "unquoted.html"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	candidates, err := DiscoverCandidates("https://example.com/", strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("DiscoverCandidates: %v", err)
	}
	want := map[string]bool{
		"https://example.com/favicon.ico":     false,
		"https://example.com/apple-touch.png": false,
	}
	for _, c := range candidates {
		if _, ok := want[c.URL]; ok {
			want[c.URL] = true
		}
	}
	for u, ok := range want {
		if !ok {
			t.Errorf("missing unquoted candidate %s", u)
		}
	}
}
