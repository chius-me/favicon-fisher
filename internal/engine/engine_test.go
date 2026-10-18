package engine

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chius-me/favicon-fisher/internal/security"
)

func TestDiscoverFetchExport(t *testing.T) {
	var imageBytes bytes.Buffer
	_ = png.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 8, 8)))
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			_, _ = w.Write([]byte(`<link rel="icon" href="/icon.png">`))
			return
		}
		requests++
		_, _ = w.Write(imageBytes.Bytes())
	}))
	defer server.Close()
	s := &Service{Client: server.Client(), Policy: security.CLIPolicy}
	ctx := context.Background()
	response := s.Handle(ctx, Request{ProtocolVersion: 1, ID: "test", Op: "discover", URL: server.URL})
	if !response.OK {
		t.Fatalf("%+v", response.Error)
	}
	discovered := response.Result.(Discovery)
	if len(discovered.Candidates) != 2 || requests != 0 {
		t.Fatalf("discovery must not fetch icons: %+v requests=%d", discovered, requests)
	}
	dir := t.TempDir()
	response = s.Handle(ctx, Request{ProtocolVersion: 1, ID: "test", Op: "fetch", URL: discovered.Candidates[0].URL, CacheDir: dir})
	if !response.OK {
		t.Fatalf("%+v", response.Error)
	}
	artifact := response.Result.(Artifact)
	if artifact.Format != "png" || artifact.Width != 8 || artifact.PreviewPath == "" {
		t.Fatalf("%+v", artifact)
	}
	response = s.Handle(ctx, Request{ProtocolVersion: 1, ID: "test", Op: "export", CacheDir: dir, Source: artifact.Path, Format: "jpg"})
	if !response.OK {
		t.Fatalf("%+v", response.Error)
	}
	exported := response.Result.(Artifact)
	data, err := os.ReadFile(filepath.Join(dir, exported.Path))
	if err != nil {
		t.Fatal(err)
	}
	_, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "jpeg" {
		t.Fatalf("format=%s err=%v", format, err)
	}
}

func TestProtocolAndPrivateDestinations(t *testing.T) {
	s := New()
	for _, req := range []Request{
		{ProtocolVersion: 2, ID: "x", Op: "capabilities"},
		{ProtocolVersion: 1, ID: "x", Op: "discover", URL: "http://127.0.0.1/"},
		{ProtocolVersion: 1, ID: "x", Op: "fetch", URL: "file:///etc/hosts", CacheDir: t.TempDir()},
	} {
		if s.Handle(context.Background(), req).OK {
			t.Fatalf("accepted %+v", req)
		}
	}
}

func TestExportRejectsTraversalAndSymlinks(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.png")
	_ = os.WriteFile(outside, []byte("secret"), 0600)
	if err := os.Symlink(outside, filepath.Join(dir, "link.png")); err != nil {
		t.Skip(err)
	}
	for _, name := range []string{"../outside.png", outside, "link.png", "."} {
		response := New().Handle(context.Background(), Request{ProtocolVersion: 1, ID: "x", Op: "export", CacheDir: dir, Source: name, Format: "original"})
		if response.OK {
			t.Fatalf("accepted %s", name)
		}
	}
}

func TestDeadlineCancelsPageFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	s := &Service{Client: server.Client(), Policy: security.CLIPolicy}
	start := time.Now()
	response := s.Handle(context.Background(), Request{ProtocolVersion: 1, ID: "x", Op: "discover", URL: server.URL, TimeoutMS: 25})
	if response.Error == nil || response.Error.Code != "NETWORK_TIMEOUT" || time.Since(start) > time.Second {
		t.Fatalf("%+v", response)
	}
}
