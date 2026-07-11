package security

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

func TestSignerRoundTrip(t *testing.T) {
	s := NewSigner("test-secret-key", time.Minute)
	url := "https://cdn.example.com/favicon.png"
	token := s.Sign(url)
	if err := s.Verify(url, token); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if err := s.Verify("https://evil.example.com/x.png", token); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestSignerExpired(t *testing.T) {
	s := &Signer{secret: []byte("test-secret-key"), ttl: time.Minute}
	exp := time.Now().Add(-time.Minute).Unix()
	mac := s.mac("https://example.com/a.png", exp)
	token := fmt.Sprintf("%d.%s", exp, base64.RawURLEncoding.EncodeToString(mac))
	if err := s.Verify("https://example.com/a.png", token); err == nil {
		t.Fatal("expected expired token error")
	}
}

func TestSignerRejectsEmptyToken(t *testing.T) {
	s := NewSigner("secret", time.Minute)
	if err := s.Verify("https://example.com/a.png", ""); err == nil {
		t.Fatal("expected error for empty token")
	}
}
