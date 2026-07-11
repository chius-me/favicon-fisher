package security

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultTokenTTL is how long preview-issued download/proxy tokens remain valid.
	DefaultTokenTTL = 15 * time.Minute
	// EnvSigningSecret is the environment variable for a stable HMAC secret.
	EnvSigningSecret = "FVF_SIGNING_SECRET"
)

// Signer issues and verifies short-lived HMAC tokens bound to an icon URL.
type Signer struct {
	secret []byte
	ttl    time.Duration
}

// NewSigner creates a signer. If secret is empty, FVF_SIGNING_SECRET is used;
// if that is also empty, a random ephemeral secret is generated (tokens invalid after restart).
func NewSigner(secret string, ttl time.Duration) *Signer {
	if ttl <= 0 {
		ttl = DefaultTokenTTL
	}
	sec := []byte(strings.TrimSpace(secret))
	if len(sec) == 0 {
		sec = []byte(strings.TrimSpace(os.Getenv(EnvSigningSecret)))
	}
	if len(sec) == 0 {
		sec = make([]byte, 32)
		if _, err := rand.Read(sec); err != nil {
			// Extremely unlikely; fall back to a process-unique weak secret.
			sec = []byte(fmt.Sprintf("favicon-fisher-ephemeral-%d", time.Now().UnixNano()))
		} else {
			// Store hex form so logs can tell it is random without printing raw bytes.
			sec = []byte(hex.EncodeToString(sec))
		}
	}
	return &Signer{secret: sec, ttl: ttl}
}

// Sign returns a token for iconURL.
// Format: <expiryUnix>.<base64url(hmac)>
func (s *Signer) Sign(iconURL string) string {
	exp := time.Now().Add(s.ttl).Unix()
	mac := s.mac(iconURL, exp)
	return fmt.Sprintf("%d.%s", exp, base64.RawURLEncoding.EncodeToString(mac))
}

// Verify checks that token authorizes iconURL and has not expired.
func (s *Signer) Verify(iconURL, token string) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("token is required")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return errors.New("invalid token")
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return errors.New("invalid token")
	}
	if time.Now().Unix() > exp {
		return errors.New("token expired")
	}
	expected, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return errors.New("invalid token")
	}
	mac := s.mac(iconURL, exp)
	if !hmac.Equal(expected, mac) {
		return errors.New("invalid token")
	}
	return nil
}

func (s *Signer) mac(iconURL string, exp int64) []byte {
	h := hmac.New(sha256.New, s.secret)
	_, _ = h.Write([]byte(iconURL))
	_, _ = h.Write([]byte{'\n'})
	_, _ = h.Write([]byte(strconv.FormatInt(exp, 10)))
	return h.Sum(nil)
}
