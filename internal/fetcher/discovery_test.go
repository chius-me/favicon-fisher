package fetcher

import (
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
