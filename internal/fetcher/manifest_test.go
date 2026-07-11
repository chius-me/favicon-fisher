package fetcher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestPreviewIncludesManifestIcons(t *testing.T) {
	html, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "sample.html"))
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "site.webmanifest"))
	if err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(html)
		case "/site.webmanifest":
			w.Header().Set("Content-Type", "application/manifest+json")
			_, _ = w.Write(manifest)
		case "/favicon-32.png", "/icons/icon-192.png", "/safari.svg", "/favicon.ico":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("x"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	f := New(server.Client())
	preview, err := f.Preview(context.Background(), server.URL+"/")
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	foundManifest := false
	for _, c := range preview.Candidates {
		if c.Rel == "manifest" && c.URL == server.URL+"/icons/icon-192.png" {
			foundManifest = true
		}
	}
	if !foundManifest {
		t.Fatalf("expected manifest icon in candidates, got %+v", preview.Candidates)
	}
}
