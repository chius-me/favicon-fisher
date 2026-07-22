package security

import (
	"testing"
	"time"
)

func FuzzSignerRoundTrip(f *testing.F) {
	f.Add("https://cdn.example.com/favicon.png")
	f.Add("")
	f.Add("http://[::1]/icon")
	f.Fuzz(func(t *testing.T, iconURL string) {
		s := NewSigner("fuzz-secret-key-not-for-prod", time.Minute)
		token := s.Sign(iconURL)
		if err := s.Verify(iconURL, token); err != nil {
			t.Fatalf("round-trip failed for %q: %v", iconURL, err)
		}
		// Mutate token slightly — must not panic.
		_ = s.Verify(iconURL, token+"x")
		_ = s.Verify(iconURL+"x", token)
		_ = s.VerifyFor(iconURL, token, "other-purpose")
	})
}

func FuzzParseTrustedProxies(f *testing.F) {
	f.Add("10.0.0.0/8")
	f.Add("127.0.0.1,192.168.0.0/16")
	f.Add("not-a-cidr")
	f.Add("")
	f.Fuzz(func(t *testing.T, raw string) {
		_ = ParseTrustedProxies(raw)
	})
}
