package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPreviewHandlerReturnsRecommendedIconAndCandidates(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<html><head><link rel="icon" href="/favicon.png" sizes="32x32"><link rel="apple-touch-icon" href="/apple.png" sizes="180x180"></head></html>`))
		case "/favicon.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png-icon"))
		case "/apple.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("apple-icon"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer origin.Close()

	handler := NewHandler(origin.Client())
	req := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewBufferString(`{"url":"`+origin.URL+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Preview(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}

	var resp PreviewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}

	if resp.RecommendedIconURL != origin.URL+"/favicon.png" {
		t.Fatalf("expected recommended icon %q, got %q", origin.URL+"/favicon.png", resp.RecommendedIconURL)
	}
	if len(resp.Icons) != 3 {
		t.Fatalf("expected 3 previewable icons, got %d", len(resp.Icons))
	}
	if resp.Icons[0].SourceRel != "icon" {
		t.Fatalf("expected first icon rel=icon, got %q", resp.Icons[0].SourceRel)
	}
}

func TestDownloadHandlerConvertsToRequestedFormat(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.RGBA{B: 255, A: 255})
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("encode png: %v", err)
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(buf.Bytes())
	}))
	defer origin.Close()

	handler := NewHandler(origin.Client())
	req := httptest.NewRequest(http.MethodPost, "/api/download", bytes.NewBufferString(`{"icon_url":"`+origin.URL+`/favicon.png","format":"jpg"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.Download(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "image/jpeg" {
		t.Fatalf("expected image/jpeg content-type, got %q", got)
	}
	if got := rr.Header().Get("Content-Disposition"); got == "" || !strings.Contains(got, ".jpg") {
		t.Fatalf("expected .jpg download filename, got %q", got)
	}

	_, format, err := image.Decode(bytes.NewReader(rr.Body.Bytes()))
	if err != nil {
		t.Fatalf("decode response image: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("expected jpeg payload, got %q", format)
	}
}
