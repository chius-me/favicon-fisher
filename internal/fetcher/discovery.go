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

func DiscoverCandidates(pageURL string, body io.Reader) ([]Candidate, error) {
	parsedPageURL, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("parse page URL: %w", err)
	}

	root, err := html.Parse(body)
	if err != nil {
		return nil, fmt.Errorf("parse HTML: %w", err)
	}

	var candidates []Candidate
	seen := map[string]bool{}

	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "link" {
			rel := strings.ToLower(strings.TrimSpace(getAttr(node, "rel")))
			href := strings.TrimSpace(getAttr(node, "href"))
			if href != "" && isIconRel(rel) {
				resolved := resolveURL(parsedPageURL, href)
				if resolved != "" && !seen[resolved] {
					candidate := Candidate{
						URL:      resolved,
						Rel:      rel,
						Sizes:    strings.TrimSpace(getAttr(node, "sizes")),
						Type:     strings.TrimSpace(getAttr(node, "type")),
						Priority: relPriority(rel),
					}
					candidates = append(candidates, candidate)
					seen[resolved] = true
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)

	fallback := parsedPageURL.ResolveReference(&url.URL{Path: "/favicon.ico"}).String()
	if !seen[fallback] {
		candidates = append(candidates, Candidate{
			URL:      fallback,
			Rel:      "fallback",
			Priority: relPriority("fallback"),
		})
	}

	return candidates, nil
}

func BestCandidate(candidates []Candidate) (Candidate, error) {
	if len(candidates) == 0 {
		return Candidate{}, errors.New("no favicon candidates found")
	}

	sorted := append([]Candidate(nil), candidates...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Priority != sorted[j].Priority {
			return sorted[i].Priority < sorted[j].Priority
		}
		return sizeScore(sorted[i].Sizes) > sizeScore(sorted[j].Sizes)
	})

	return sorted[0], nil
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
	return base.ResolveReference(parsed).String()
}

func sizeScore(sizes string) int {
	maxScore := 0
	for _, part := range strings.Fields(strings.ToLower(sizes)) {
		pieces := strings.Split(part, "x")
		if len(pieces) != 2 {
			continue
		}
		w, errW := strconv.Atoi(pieces[0])
		h, errH := strconv.Atoi(pieces[1])
		if errW != nil || errH != nil {
			continue
		}
		score := w * h
		if score > maxScore {
			maxScore = score
		}
	}
	return maxScore
}
