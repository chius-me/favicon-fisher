package security

import (
	"errors"
	"strings"
)

// PublicError returns a client-safe error message, stripping internal/network details.
func PublicError(err error) string {
	if err == nil {
		return "unknown error"
	}
	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "private or reserved"),
		strings.Contains(lower, "blocked address"),
		strings.Contains(lower, "redirect blocked"),
		strings.Contains(lower, "only http and https"):
		return "URL is not allowed"
	case strings.Contains(lower, "token"):
		return msg
	case strings.Contains(lower, "rate limit"):
		return "rate limit exceeded"
	case strings.Contains(lower, "exceeds"):
		return "response too large"
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "deadline exceeded"):
		return "upstream request timed out"
	case strings.Contains(lower, "input url is required") || strings.Contains(lower, "url is required"):
		return "url is required"
	case strings.Contains(lower, "no favicon") || strings.Contains(lower, "no previewable"):
		return "no favicon candidates found"
	case strings.Contains(lower, "invalid json"):
		return "invalid JSON body"
	case strings.Contains(lower, "status"):
		return "upstream request failed"
	case strings.Contains(lower, "decode source") || strings.Contains(lower, "unsupported output") ||
		strings.Contains(lower, "svg sources") || strings.Contains(lower, "ico sources") ||
		strings.Contains(lower, "svg output"):
		// Conversion errors are safe to surface.
		return msg
	default:
		// Avoid leaking dial errors, DNS details, raw URLs with credentials, etc.
		if errors.Is(err, errors.New("")) {
			return "request failed"
		}
		if strings.Contains(lower, "connection") || strings.Contains(lower, "lookup") ||
			strings.Contains(lower, "dial") || strings.Contains(lower, "tls") ||
			strings.Contains(lower, "eof") || strings.Contains(lower, "refused") {
			return "upstream request failed"
		}
		// Keep short validation-style messages; clamp long ones.
		if len(msg) > 120 || strings.Contains(msg, "://") {
			return "request failed"
		}
		return msg
	}
}
