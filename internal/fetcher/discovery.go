package fetcher

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"github.com/chius-me/favicon-fisher/internal/security"
)

func NormalizeInputURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("input URL is required")
	}

	if !strings.Contains(trimmed, "://") {
		trimmed = "https://" + trimmed
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("parse input URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid URL: %s", raw)
	}

	return parsed.String(), nil
}

// DiscoveryResult holds icon candidates and the optional manifest href from one HTML parse.
type DiscoveryResult struct {
	Candidates   []Candidate
	ManifestHref string
}

// DiscoverFromHTML parses HTML once and returns icon candidates plus manifest href.
func DiscoverFromHTML(pageURL string, body io.Reader) (DiscoveryResult, error) {
	parsedPageURL, err := url.Parse(pageURL)
	if err != nil {
		return DiscoveryResult{}, fmt.Errorf("parse page URL: %w", err)
	}

	root, err := html.Parse(body)
	if err != nil {
		return DiscoveryResult{}, fmt.Errorf("parse HTML: %w", err)
	}

	var candidates []Candidate
	seen := map[string]bool{}
	var manifestHref string
	htmlCount := 0

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "link" {
			rel := strings.ToLower(strings.TrimSpace(getAttr(node, "rel")))
			href := strings.TrimSpace(getAttr(node, "href"))
			if rel == "manifest" && manifestHref == "" && href != "" {
				manifestHref = href
			}
			if href != "" && isIconRel(rel) && htmlCount < security.MaxHTMLIconCandidates {
				resolved := resolveURL(parsedPageURL, href)
				if resolved != "" && !seen[resolved] {
					candidates = append(candidates, Candidate{
						URL:      resolved,
						Rel:      rel,
						Sizes:    strings.TrimSpace(getAttr(node, "sizes")),
						Type:     strings.TrimSpace(getAttr(node, "type")),
						Priority: relPriority(rel),
					})
					seen[resolved] = true
					htmlCount++
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)

	fallback := parsedPageURL.ResolveReference(&url.URL{Path: "/favicon.ico"}).String()
	if !seen[fallback] && len(candidates) < security.MaxTotalCandidates {
		candidates = append(candidates, Candidate{
			URL:      fallback,
			Rel:      "fallback",
			Priority: relPriority("fallback"),
		})
	}

	if len(candidates) > security.MaxTotalCandidates {
		candidates = candidates[:security.MaxTotalCandidates]
	}

	return DiscoveryResult{Candidates: candidates, ManifestHref: manifestHref}, nil
}

func DiscoverCandidates(pageURL string, body io.Reader) ([]Candidate, error) {
	result, err := DiscoverFromHTML(pageURL, body)
	if err != nil {
		return nil, err
	}
	return result.Candidates, nil
}

// RankCandidates returns candidates sorted by priority then size (best first).
func RankCandidates(candidates []Candidate) []Candidate {
	sorted := append([]Candidate(nil), candidates...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority < sorted[j].Priority
		}
		return sizeScore(sorted[i].Sizes) > sizeScore(sorted[j].Sizes)
	})
	return sorted
}

func BestCandidate(candidates []Candidate) (Candidate, error) {
	if len(candidates) == 0 {
		return Candidate{}, errors.New("no favicon candidates found")
	}
	return RankCandidates(candidates)[0], nil
}

func getAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func isIconRel(rel string) bool {
	for _, part := range strings.Fields(rel) {
		switch part {
		case "icon", "shortcut", "apple-touch-icon", "apple-touch-icon-precomposed", "mask-icon":
			return true
		}
	}
	return false
}

func relPriority(rel string) int {
	switch rel {
	case "icon", "shortcut icon":
		return 10
	case "manifest":
		return 15
	case "apple-touch-icon", "apple-touch-icon-precomposed":
		return 20
	case "mask-icon":
		return 30
	case "fallback":
		return 100
	default:
		if strings.Contains(rel, "icon") {
			return 40
		}
		return 90
	}
}

func resolveURL(base *url.URL, href string) string {
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}
	// Only allow http(s) icon targets (block javascript:, data:, file:, etc.)
	resolved := base.ResolveReference(parsed)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}

func resolveURLMust(baseURL, href string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	return resolveURL(base, href)
}

// FindManifestHref returns the first <link rel="manifest"> href from HTML body.
func FindManifestHref(body io.Reader) string {
	result, err := DiscoverFromHTML("https://example.invalid/", body)
	if err != nil {
		return ""
	}
	return result.ManifestHref
}

func sizeScore(sizes string) int64 {
	var maxScore int64
	const maxDim = 16384
	for _, part := range strings.Fields(strings.ToLower(sizes)) {
		pieces := strings.Split(part, "x")
		if len(pieces) != 2 {
			continue
		}
		w, errW := strconv.Atoi(pieces[0])
		h, errH := strconv.Atoi(pieces[1])
		if errW != nil || errH != nil || w <= 0 || h <= 0 {
			continue
		}
		if w > maxDim {
			w = maxDim
		}
		if h > maxDim {
			h = maxDim
		}
		score := int64(w) * int64(h)
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore
}
