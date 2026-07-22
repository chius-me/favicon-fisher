package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/chius-me/favicon-fisher/internal/security"
)

func testHandler(origin *httptest.Server) *Handler {
	// httptest is loopback — allow private for unit tests.
	policy := security.CLIPolicy
	return NewHandlerWithOptions(HandlerOptions{
		Client: origin.Client(),
		Signer: security.NewSigner("test-secret", time.Hour),
		Policy: &policy,
	})
}

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
		case "/favicon.ico":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer origin.Close()

	handler := testHandler(origin)
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
	if len(resp.Icons) < 2 {
		t.Fatalf("expected at least 2 previewable icons, got %d", len(resp.Icons))
	}
	if resp.Icons[0].Token == "" {
		t.Fatal("expected signed token on icons")
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

	handler := testHandler(origin)
	iconURL := origin.URL + "/favicon.png"
	token := handler.signer.Sign(iconURL)

	body, _ := json.Marshal(DownloadRequest{IconURL: iconURL, Format: "jpg", Token: token})
	req := httptest.NewRequest(http.MethodPost, "/api/download", bytes.NewReader(body))
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

func TestDownloadHandlerRejectsMissingToken(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer origin.Close()

	handler := testHandler(origin)
	body, _ := json.Marshal(DownloadRequest{IconURL: origin.URL + "/x.png", Format: "png"})
	req := httptest.NewRequest(http.MethodPost, "/api/download", bytes.NewReader(body))
	rr := httptest.NewRecorder()
	handler.Download(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestPreviewRejectsPrivateURLWhenPolicyStrict(t *testing.T) {
	policy := security.DefaultPolicy
	handler := NewHandlerWithOptions(HandlerOptions{
		Signer: security.NewSigner("test", time.Hour),
		Policy: &policy,
		Client: security.SafeHTTPClient(security.ClientOptions{
			Timeout: 5 * time.Second,
			Policy:  policy,
		}),
	})
	req := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewBufferString(`{"url":"http://127.0.0.1/"}`))
	rr := httptest.NewRecorder()
	handler.Preview(rr, req)
	if rr.Code == http.StatusOK {
		t.Fatalf("expected failure for private URL, got 200: %s", rr.Body.String())
	}
}

func TestProxyHandlerServesSignedIcon(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("png-proxy-bytes"))
	}))
	defer origin.Close()

	handler := testHandler(origin)
	iconURL := origin.URL + "/favicon.png"
	token := handler.signer.Sign(iconURL)

	req := httptest.NewRequest(http.MethodGet, "/api/proxy?url="+url.QueryEscape(iconURL)+"&token="+url.QueryEscape(token), nil)
	rr := httptest.NewRecorder()
	handler.Proxy(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected image/png, got %q", got)
	}
	if rr.Body.String() != "png-proxy-bytes" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

func TestProxyHandlerRejectsMissingToken(t *testing.T) {
	handler := testHandler(httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})))
	req := httptest.NewRequest(http.MethodGet, "/api/proxy?url=https://example.com/x.png", nil)
	rr := httptest.NewRecorder()
	handler.Proxy(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}
}

func TestAllowedTypesForUnknownReturnsEmpty(t *testing.T) {
	if got := allowedTypesFor("https://example.com/path", "text/html"); len(got) != 0 {
		t.Fatalf("expected empty allowed types for HTML, got %v", got)
	}
	if got := allowedTypesFor("https://example.com/icon.png", ""); len(got) == 0 {
		t.Fatal("expected types for .png URL")
	}
	if got := allowedTypesFor("http://192.168.1.1/admin", ""); len(got) != 0 {
		t.Fatalf("expected empty for extensionless private-looking URL, got %v", got)
	}
}

func TestPreviewFiltersNonImageCandidates(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			// Private-looking and non-image candidates should not all become previewable.
			_, _ = w.Write([]byte(`<html><head>
				<link rel="icon" href="/ok.png" type="image/png">
				<link rel="icon" href="http://192.168.1.1/router">
				<link rel="icon" href="/page.html" type="text/html">
			</head></html>`))
		case "/ok.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer origin.Close()

	handler := testHandler(origin)
	req := httptest.NewRequest(http.MethodPost, "/api/preview", bytes.NewBufferString(`{"url":"`+origin.URL+`"}`))
	rr := httptest.NewRecorder()
	handler.Preview(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rr.Code, rr.Body.String())
	}
	var resp PreviewResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, icon := range resp.Icons {
		if strings.Contains(icon.IconURL, "192.168") {
			t.Fatalf("private icon URL should be filtered: %s", icon.IconURL)
		}
		if strings.HasSuffix(icon.IconURL, ".html") {
			t.Fatalf("html candidate should be filtered: %s", icon.IconURL)
		}
	}
}
