package security

import (
	"net/netip"
	"strings"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	cases := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"192.168.1.1", true},
		{"172.16.0.1", true},
		{"169.254.169.254", true},
		{"100.64.0.1", true},
		{"0.0.0.0", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"::1", true},
		{"fc00::1", true},
		{"2001:4860:4860::8888", false},
	}
	for _, tc := range cases {
		ip := netip.MustParseAddr(tc.ip)
		if got := IsBlockedIP(ip); got != tc.blocked {
			t.Errorf("IsBlockedIP(%s)=%v want %v", tc.ip, got, tc.blocked)
		}
	}
}

func TestValidateFetchURLRejectsBadSchemes(t *testing.T) {
	for _, raw := range []string{"file:///etc/passwd", "ftp://example.com", "gopher://x", "javascript:alert(1)"} {
		if _, err := ValidateFetchURL(raw); err == nil {
			t.Errorf("expected error for %q", raw)
		}
	}
}

func TestValidateFetchURLRejectsLoopbackHost(t *testing.T) {
	if _, err := ValidateFetchURL("http://127.0.0.1/"); err == nil {
		t.Fatal("expected loopback to be rejected")
	}
	if _, err := ValidateFetchURL("http://[::1]/"); err == nil {
		t.Fatal("expected IPv6 loopback to be rejected")
	}
}

func TestValidateFetchURLAllowsPublicHTTPS(t *testing.T) {
	// Use an IP literal that is public so we don't depend on DNS in unit tests.
	u, err := ValidateFetchURL("https://8.8.8.8/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Host != "8.8.8.8" {
		t.Fatalf("unexpected host %q", u.Host)
	}
}

func TestLimitedReadAll(t *testing.T) {
	out, err := LimitedReadAll(strings.NewReader("hello world"), 100)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "hello world" {
		t.Fatalf("got %q", out)
	}
	_, err = LimitedReadAll(strings.NewReader("hello world"), 5)
	if err == nil {
		t.Fatal("expected oversize error")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("unexpected error: %v", err)
	}
	// Ensure exact size is accepted
	exact, err := LimitedReadAll(strings.NewReader("12345"), 5)
	if err != nil {
		t.Fatal(err)
	}
	if string(exact) != "12345" {
		t.Fatalf("got %q", exact)
	}
}
