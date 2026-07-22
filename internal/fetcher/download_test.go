package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDownloadIconSavesPNGAndReturnsMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-bytes"))
	}))
	defer server.Close()

	result, err := DownloadIcon(context.Background(), server.Client(), server.URL+"/favicon.png", t.TempDir(), "", "")
	if err != nil {
		t.Fatalf("DownloadIcon returned error: %v", err)
	}

	if result.ContentType != "image/png" {
		t.Fatalf("expected image/png, got %q", result.ContentType)
	}
	if result.Bytes != int64(len("png-bytes")) {
		t.Fatalf("expected %d bytes, got %d", len("png-bytes"), result.Bytes)
	}
	if filepath.Ext(result.OutputPath) != ".png" {
		t.Fatalf("expected .png output path, got %q", result.OutputPath)
	}
	if _, err := os.Stat(result.OutputPath); err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
}

func TestDownloadIconInfersICOExtensionFromContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/x-icon")
		_, _ = w.Write([]byte("ico-bytes"))
	}))
	defer server.Close()

	result, err := DownloadIcon(context.Background(), server.Client(), server.URL+"/asset", t.TempDir(), "", "")
	if err != nil {
		t.Fatalf("DownloadIcon returned error: %v", err)
	}

	if filepath.Ext(result.OutputPath) != ".ico" {
		t.Fatalf("expected .ico output path, got %q", result.OutputPath)
	}
	if !strings.HasSuffix(result.Filename, ".ico") {
		t.Fatalf("expected filename to end with .ico, got %q", result.Filename)
	}
}

func TestDownloadIconReturnsErrorOnNonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "missing", http.StatusNotFound)
	}))
	defer server.Close()

	_, err := DownloadIcon(context.Background(), server.Client(), server.URL+"/missing.ico", t.TempDir(), "", "")
	if err == nil {
		t.Fatal("expected error for non-success status")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected status code in error, got %v", err)
	}
}

func TestFetcherPreviewReturnsBestCandidateAndAllCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><link rel="icon" href="/favicon.png"><link rel="apple-touch-icon" href="/apple.png"></head></html>`))
		case "/favicon.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png-bytes"))
		case "/apple.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("apple-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	fetcher := New(server.Client())
	preview, err := fetcher.Preview(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}

	if preview.InputURL != server.URL {
		t.Fatalf("expected input URL %q, got %q", server.URL, preview.InputURL)
	}
	if preview.PageURL != server.URL {
		t.Fatalf("expected page URL %q, got %q", server.URL, preview.PageURL)
	}
	if preview.Best.URL != server.URL+"/favicon.png" {
		t.Fatalf("expected best URL %q, got %q", server.URL+"/favicon.png", preview.Best.URL)
	}
	if len(preview.Candidates) != 3 {
		t.Fatalf("expected 3 candidates including fallback, got %d", len(preview.Candidates))
	}
}

func TestFetcherFetchReturnsDiscoveredIconMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><link rel="icon" href="/favicon.png"></head></html>`))
		case "/favicon.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png-bytes"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	fetcher := New(server.Client())
	result, err := fetcher.Fetch(context.Background(), server.URL, t.TempDir(), false)
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}

	if result.InputURL != server.URL {
		t.Fatalf("expected input URL %q, got %q", server.URL, result.InputURL)
	}
	if result.PageURL != server.URL {
		t.Fatalf("expected page URL %q, got %q", server.URL, result.PageURL)
	}
	if result.IconURL != server.URL+"/favicon.png" {
		t.Fatalf("expected icon URL %q, got %q", server.URL+"/favicon.png", result.IconURL)
	}
	if result.SourceRel != "icon" {
		t.Fatalf("expected source rel icon, got %q", result.SourceRel)
	}
	if result.FallbackUsed {
		t.Fatal("expected fallbackUsed to be false")
	}
}

func TestFetchFallsBackWhenBestCandidate404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head>
				<link rel="icon" href="/broken.png" sizes="32x32">
				<link rel="apple-touch-icon" href="/ok.png" sizes="180x180">
			</head></html>`))
		case "/broken.png":
			http.NotFound(w, r)
		case "/ok.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("ok-png"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	fetcher := New(server.Client())
	result, err := fetcher.Fetch(context.Background(), server.URL, t.TempDir(), false)
	if err != nil {
		t.Fatalf("Fetch should fall back to secondary candidate: %v", err)
	}
	if result.IconURL != server.URL+"/ok.png" {
		t.Fatalf("expected fallback to ok.png, got %q", result.IconURL)
	}
}

func TestFetchAllWritesDistinctFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head>
				<link rel="icon" href="/a.png">
				<link rel="icon" href="/b.png">
			</head></html>`))
		case "/a.png", "/b.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png-" + r.URL.Path))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	fetcher := New(server.Client())
	result, err := fetcher.Fetch(context.Background(), server.URL, dir, true)
	if err != nil {
		t.Fatalf("Fetch all: %v", err)
	}
	if len(result.AllIcons) < 2 {
		t.Fatalf("expected at least 2 icons, got %d", len(result.AllIcons))
	}
	seen := map[string]bool{}
	for _, icon := range result.AllIcons {
		if seen[icon.Filename] {
			t.Fatalf("duplicate filename %q", icon.Filename)
		}
		seen[icon.Filename] = true
		if _, err := os.Stat(icon.OutputPath); err != nil {
			t.Fatalf("missing file %s: %v", icon.OutputPath, err)
		}
	}
}
