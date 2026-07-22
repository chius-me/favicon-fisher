package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIPIgnoresProxyHeadersByDefault(t *testing.T) {
	rl := NewRateLimiter(60, time.Minute)
	defer rl.Stop()

	req := httptest.NewRequest(http.MethodGet, "/api/preview", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	req.Header.Set("CF-Connecting-IP", "198.51.100.2")

	if got := rl.ClientIP(req); got != "203.0.113.10" {
		t.Fatalf("expected direct remote IP, got %q", got)
	}
}

func TestClientIPTrustsHeadersFromTrustedProxy(t *testing.T) {
	proxies := ParseTrustedProxies("10.0.0.0/8,127.0.0.1")
	rl := NewRateLimiterWithProxies(60, time.Minute, proxies)
	defer rl.Stop()

	req := httptest.NewRequest(http.MethodGet, "/api/preview", nil)
	req.RemoteAddr = "10.0.0.5:443"
	req.Header.Set("CF-Connecting-IP", "198.51.100.9")
	if got := rl.ClientIP(req); got != "198.51.100.9" {
		t.Fatalf("expected CF-Connecting-IP, got %q", got)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/preview", nil)
	req2.RemoteAddr = "127.0.0.1:80"
	req2.Header.Set("X-Forwarded-For", "203.0.113.50, 10.0.0.5")
	if got := rl.ClientIP(req2); got != "203.0.113.50" {
		t.Fatalf("expected left-most XFF client, got %q", got)
	}
}

func TestParseTrustedProxies(t *testing.T) {
	nets := ParseTrustedProxies("10.0.0.1, 192.168.0.0/16, bogon")
	if len(nets) != 2 {
		t.Fatalf("expected 2 networks, got %d", len(nets))
	}
}

func TestAllowWithRetry(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)
	defer rl.Stop()
	if _, ok := rl.AllowWithRetry("a"); !ok {
		t.Fatal("first should allow")
	}
	if _, ok := rl.AllowWithRetry("a"); !ok {
		t.Fatal("second should allow")
	}
	retry, ok := rl.AllowWithRetry("a")
	if ok {
		t.Fatal("third should deny")
	}
	if retry < 1 {
		t.Fatalf("expected positive retry-after, got %d", retry)
	}
}
