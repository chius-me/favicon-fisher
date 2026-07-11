package security

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// DefaultUserAgent is sent on all outbound HTTP requests.
const DefaultUserAgent = "FaviconFisher/1.0 (+https://github.com/chius-me/favicon-fisher)"

// Policy controls outbound fetch restrictions.
type Policy struct {
	// AllowPrivate permits loopback/private/link-local destinations (CLI / tests).
	// Web and Worker deployments must keep this false.
	AllowPrivate bool
}

// DefaultPolicy rejects private addresses (safe for public web services).
var DefaultPolicy = Policy{AllowPrivate: false}

// CLIPolicy allows private addresses for local developer use.
var CLIPolicy = Policy{AllowPrivate: true}

// ValidateFetchURL ensures the URL is http(s) and, unless policy allows private,
// does not resolve to a blocked address.
func ValidateFetchURL(raw string) (*url.URL, error) {
	return ValidateFetchURLWithPolicy(raw, DefaultPolicy)
}

// ValidateFetchURLWithPolicy is like ValidateFetchURL with an explicit policy.
func ValidateFetchURLWithPolicy(raw string, policy Policy) (*url.URL, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("URL is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("only http and https URLs are allowed")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("invalid URL host")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("URLs with userinfo are not allowed")
	}

	host := parsed.Hostname()
	if host == "" {
		return nil, fmt.Errorf("invalid URL host")
	}

	if policy.AllowPrivate {
		return parsed, nil
	}

	if ip, err := netip.ParseAddr(host); err == nil {
		if IsBlockedIP(ip) {
			return nil, fmt.Errorf("requests to private or reserved addresses are not allowed")
		}
		return parsed, nil
	}

	// Resolve hostname and reject if any address is blocked.
	ips, err := net.DefaultResolver.LookupIPAddr(context.Background(), host)
	if err != nil {
		return nil, fmt.Errorf("resolve host: %w", err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("host resolved to no addresses")
	}
	for _, ipAddr := range ips {
		addr, ok := netip.AddrFromSlice(ipAddr.IP)
		if !ok {
			return nil, fmt.Errorf("invalid resolved address")
		}
		if IsBlockedIP(addr.Unmap()) {
			return nil, fmt.Errorf("requests to private or reserved addresses are not allowed")
		}
	}

	return parsed, nil
}

// IsBlockedIP reports whether ip is loopback, private, link-local, or otherwise non-public.
func IsBlockedIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if ip.IsMulticast() || ip.IsUnspecified() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	// CGNAT / shared address space (RFC 6598)
	if ip.Is4() {
		a := ip.As4()
		if a[0] == 100 && a[1] >= 64 && a[1] <= 127 {
			return true
		}
		// 0.0.0.0/8
		if a[0] == 0 {
			return true
		}
		// IETF protocol assignments 192.0.0.0/24
		if a[0] == 192 && a[1] == 0 && a[2] == 0 {
			return true
		}
		// TEST-NET
		if a[0] == 192 && a[1] == 0 && a[2] == 2 {
			return true
		}
		if a[0] == 198 && a[1] == 51 && a[2] == 100 {
			return true
		}
		if a[0] == 203 && a[1] == 0 && a[2] == 113 {
			return true
		}
		// Benchmarking 198.18.0.0/15
		if a[0] == 198 && (a[1] == 18 || a[1] == 19) {
			return true
		}
	}
	if ip.Is6() {
		a := ip.As16()
		if a[0]&0xfe == 0xfc {
			return true
		}
	}
	return false
}

// SafeTransport returns an HTTP transport that dials only non-blocked IPs (unless allowPrivate).
func SafeTransport(base *http.Transport, policy Policy) *http.Transport {
	if base == nil {
		base = http.DefaultTransport.(*http.Transport).Clone()
	} else {
		base = base.Clone()
	}

	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	base.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		if ip, err := netip.ParseAddr(host); err == nil {
			if !policy.AllowPrivate && IsBlockedIP(ip) {
				return nil, fmt.Errorf("blocked address")
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}

		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		var firstErr error
		for _, ipAddr := range ips {
			addrIP, ok := netip.AddrFromSlice(ipAddr.IP)
			if !ok {
				continue
			}
			unmapped := addrIP.Unmap()
			if !policy.AllowPrivate && IsBlockedIP(unmapped) {
				if firstErr == nil {
					firstErr = fmt.Errorf("blocked address")
				}
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(unmapped.String(), port))
			if err == nil {
				return conn, nil
			}
			firstErr = err
		}
		if firstErr == nil {
			firstErr = fmt.Errorf("no safe addresses for host")
		}
		return nil, firstErr
	}

	return base
}

// ClientOptions configures SafeHTTPClient.
type ClientOptions struct {
	Timeout   time.Duration
	Transport *http.Transport
	Policy    Policy
}

// SafeHTTPClient builds a client with SSRF-aware dialing, redirects, timeout, and UA injection.
func SafeHTTPClient(opts ClientOptions) *http.Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 15 * time.Second
	}
	transport := SafeTransport(opts.Transport, opts.Policy)
	policy := opts.Policy
	return &http.Client{
		Timeout:   opts.Timeout,
		Transport: &userAgentTransport{base: transport, ua: DefaultUserAgent},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if _, err := ValidateFetchURLWithPolicy(req.URL.String(), policy); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			return nil
		},
	}
}

type userAgentTransport struct {
	base http.RoundTripper
	ua   string
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if cloned.Header.Get("User-Agent") == "" {
		cloned.Header.Set("User-Agent", t.ua)
	}
	return t.base.RoundTrip(cloned)
}

// NewRequest builds a request after SSRF validation using DefaultPolicy.
func NewRequest(ctx context.Context, method, rawURL string) (*http.Request, error) {
	return NewRequestWithPolicy(ctx, method, rawURL, DefaultPolicy)
}

// NewRequestWithPolicy builds a request after SSRF validation with an explicit policy.
func NewRequestWithPolicy(ctx context.Context, method, rawURL string, policy Policy) (*http.Request, error) {
	if _, err := ValidateFetchURLWithPolicy(rawURL, policy); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	return req, nil
}
