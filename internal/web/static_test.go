package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chius-me/favicon-fisher/internal/security"
)

func TestMuxServesHomeAndFavicon(t *testing.T) {
	handler := NewHandlerWithOptions(HandlerOptions{
		Signer: security.NewSigner("test-secret-at-least-32-characters!!", time.Hour),
	})
	mux := NewMuxWithLimiter(handler, StaticFS(), nil)

	homeReq := httptest.NewRequest(http.MethodGet, "/", nil)
	homeRR := httptest.NewRecorder()
	mux.ServeHTTP(homeRR, homeReq)
	if homeRR.Code != http.StatusOK {
		t.Fatalf("GET /: expected 200, got %d body=%s", homeRR.Code, homeRR.Body.String())
	}
	home := homeRR.Body.String()
	if !strings.Contains(home, `rel="icon"`) {
		t.Fatalf("GET / HTML missing rel=%q icon link: %s", "icon", home)
	}
	if !strings.Contains(home, "/favicon.svg") {
		t.Fatalf("GET / HTML missing favicon URL: %s", home)
	}

	iconReq := httptest.NewRequest(http.MethodGet, "/favicon.svg", nil)
	iconRR := httptest.NewRecorder()
	mux.ServeHTTP(iconRR, iconReq)
	if iconRR.Code != http.StatusOK {
		t.Fatalf("GET /favicon.svg: expected 200, got %d body=%s", iconRR.Code, iconRR.Body.String())
	}
	icon := iconRR.Body.String()
	if icon == "" {
		t.Fatal("GET /favicon.svg: empty body")
	}
	if !strings.Contains(icon, "<svg") {
		t.Fatalf("GET /favicon.svg: expected SVG body, got %q", icon)
	}
}
