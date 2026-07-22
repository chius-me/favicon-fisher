package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chius-me/favicon-fisher/internal/security"
)

type Fetcher struct {
	Client *http.Client
	Policy security.Policy
}

func New(client *http.Client) *Fetcher {
	return NewWithPolicy(client, security.CLIPolicy)
}

// NewWithPolicy creates a Fetcher with an explicit SSRF policy.
// Web services should use security.DefaultPolicy (AllowPrivate=false).
func NewWithPolicy(client *http.Client, policy security.Policy) *Fetcher {
	if client == nil {
		client = security.SafeHTTPClient(security.ClientOptions{
			Timeout: 15 * time.Second,
			Policy:  policy,
		})
	}
	return &Fetcher{Client: client, Policy: policy}
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string, outputDir string, fetchAll bool) (Result, error) {
	preview, err := f.Preview(ctx, rawURL)
	if err != nil {
		return Result{}, err
	}

	candidates := RankCandidates(preview.Candidates)
	result := Result{
		InputURL: rawURL,
		PageURL:  preview.PageURL,
	}

	if fetchAll {
		var allIcons []IconResult
		downloadedURLs := make(map[string]bool)

		for i, candidate := range candidates {
			if downloadedURLs[candidate.URL] {
				continue
			}
			iconRes, err := DownloadIconWithPolicy(ctx, f.Client, f.Policy, candidate.URL, outputDir, candidate.Sizes, candidate.Rel, i+1)
			if err == nil {
				allIcons = append(allIcons, iconRes)
				downloadedURLs[candidate.URL] = true

				if result.IconURL == "" {
					result.IconURL = iconRes.IconURL
					result.OutputPath = iconRes.OutputPath
					result.ContentType = iconRes.ContentType
					result.Bytes = iconRes.Bytes
					result.StatusCode = iconRes.StatusCode
					result.Filename = iconRes.Filename
					result.SourceRel = iconRes.SourceRel
					result.FallbackUsed = iconRes.SourceRel == "fallback"
				}
			}
		}

		if len(allIcons) == 0 {
			return Result{}, fmt.Errorf("failed to download any icons")
		}
		result.AllIcons = allIcons
	} else {
		// Try ranked candidates until one downloads successfully.
		var lastErr error
		for i, candidate := range candidates {
			iconRes, err := DownloadIconWithPolicy(ctx, f.Client, f.Policy, candidate.URL, outputDir, candidate.Sizes, candidate.Rel, i+1)
			if err != nil {
				lastErr = err
				continue
			}
			result.IconURL = iconRes.IconURL
			result.OutputPath = iconRes.OutputPath
			result.ContentType = iconRes.ContentType
			result.Bytes = iconRes.Bytes
			result.StatusCode = iconRes.StatusCode
			result.Filename = iconRes.Filename
			result.SourceRel = iconRes.SourceRel
			result.FallbackUsed = candidate.Rel == "fallback"
			return result, nil
		}
		if lastErr != nil {
			return Result{}, lastErr
		}
		return Result{}, fmt.Errorf("failed to download any icons")
	}

	return result, nil
}

func (f *Fetcher) Preview(ctx context.Context, rawURL string) (PreviewResult, error) {
	normalized, err := NormalizeInputURL(rawURL)
	if err != nil {
		return PreviewResult{}, err
	}

	req, err := security.NewRequestWithPolicy(ctx, http.MethodGet, normalized, f.Policy)
	if err != nil {
		return PreviewResult{}, err
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return PreviewResult{}, fmt.Errorf("fetch page failed: status %d", resp.StatusCode)
	}

	body, err := security.LimitedReadAll(resp.Body, security.MaxHTMLBody)
	if err != nil {
		return PreviewResult{}, fmt.Errorf("read page body: %w", err)
	}

	pageURL := resp.Request.URL.String()
	discovered, err := DiscoverFromHTML(pageURL, bytes.NewReader(body))
	if err != nil {
		return PreviewResult{}, err
	}
	candidates := discovered.Candidates

	// Enrich with web manifest icons when present.
	if discovered.ManifestHref != "" {
		if manifestCandidates, mErr := f.fetchManifestIcons(ctx, pageURL, discovered.ManifestHref); mErr == nil {
			candidates = mergeCandidates(candidates, manifestCandidates)
		}
	}

	// Probe content types with a hard cap on network probes.
	probes := 0
	for i := range candidates {
		if candidates[i].Type != "" {
			continue
		}
		if probes >= security.MaxContentTypeProbes {
			break
		}
		// Prefer probing extensionless URLs; still fill type when only extension is known
		// if we have probe budget remaining after those.
		if !hasImageExtension(candidates[i].URL) || probes < security.MaxContentTypeProbes/2 {
			if ct := ProbeContentType(ctx, f.Client, f.Policy, candidates[i].URL); ct != "" {
				candidates[i].Type = ct
			}
			probes++
		}
	}

	if len(candidates) > security.MaxTotalCandidates {
		candidates = RankCandidates(candidates)[:security.MaxTotalCandidates]
	}

	best, err := BestCandidate(candidates)
	if err != nil {
		return PreviewResult{}, err
	}

	return PreviewResult{
		InputURL:   rawURL,
		PageURL:    pageURL,
		Best:       best,
		Candidates: RankCandidates(candidates),
	}, nil
}

type manifestJSON struct {
	Icons []struct {
		Src   string `json:"src"`
		Sizes string `json:"sizes"`
		Type  string `json:"type"`
	} `json:"icons"`
}

func (f *Fetcher) fetchManifestIcons(ctx context.Context, pageURL, href string) ([]Candidate, error) {
	resolved := resolveURLMust(pageURL, href)
	if resolved == "" {
		return nil, fmt.Errorf("invalid manifest URL")
	}

	req, err := security.NewRequestWithPolicy(ctx, http.MethodGet, resolved, f.Policy)
	if err != nil {
		return nil, err
	}
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("manifest status %d", resp.StatusCode)
	}

	data, err := security.LimitedReadAll(resp.Body, security.MaxManifestBody)
	if err != nil {
		return nil, err
	}

	var manifest manifestJSON
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	base := pageURL
	// Manifest-relative URLs resolve against the manifest URL per spec.
	if resp.Request != nil && resp.Request.URL != nil {
		base = resp.Request.URL.String()
	}

	var out []Candidate
	for _, icon := range manifest.Icons {
		if len(out) >= security.MaxManifestIcons {
			break
		}
		src := strings.TrimSpace(icon.Src)
		if src == "" {
			continue
		}
		iconURL := resolveURLMust(base, src)
		if iconURL == "" {
			continue
		}
		out = append(out, Candidate{
			URL:      iconURL,
			Rel:      "manifest",
			Sizes:    strings.TrimSpace(icon.Sizes),
			Type:     strings.TrimSpace(icon.Type),
			Priority: relPriority("manifest"),
		})
	}
	return out, nil
}

func mergeCandidates(base, extra []Candidate) []Candidate {
	seen := make(map[string]bool, len(base))
	for _, c := range base {
		seen[c.URL] = true
	}
	for _, c := range extra {
		if seen[c.URL] {
			continue
		}
		if len(base) >= security.MaxTotalCandidates {
			break
		}
		base = append(base, c)
		seen[c.URL] = true
	}
	return base
}

func hasImageExtension(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	// Strip query/fragment roughly
	if i := strings.IndexAny(lower, "?#"); i >= 0 {
		lower = lower[:i]
	}
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".svg", ".ico", ".gif", ".webp"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// ProbeContentType issues a HEAD (then tiny GET fallback) to learn Content-Type.
func ProbeContentType(ctx context.Context, client *http.Client, policy security.Policy, iconURL string) string {
	if client == nil {
		return ""
	}
	if ct := doProbe(ctx, client, policy, iconURL, http.MethodHead); ct != "" {
		return ct
	}
	return doProbe(ctx, client, policy, iconURL, http.MethodGet)
}

func doProbe(ctx context.Context, client *http.Client, policy security.Policy, iconURL, method string) string {
	req, err := security.NewRequestWithPolicy(ctx, method, iconURL, policy)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	if method == http.MethodGet {
		_, _ = security.LimitedReadAll(resp.Body, 64)
	}
	return strings.TrimSpace(resp.Header.Get("Content-Type"))
}
