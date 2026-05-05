package fetcher

import (
	"context"
	"fmt"
	"net/http"
)

type Fetcher struct {
	Client *http.Client
}

func New(client *http.Client) *Fetcher {
	if client == nil {
		client = &http.Client{}
	}
	return &Fetcher{Client: client}
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string, outputDir string) (Result, error) {
	normalized, err := NormalizeInputURL(rawURL)
	if err != nil {
		return Result{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalized, nil)
	if err != nil {
		return Result{}, fmt.Errorf("build page request: %w", err)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("fetch page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, fmt.Errorf("fetch page failed: status %d", resp.StatusCode)
	}

	candidates, err := DiscoverCandidates(resp.Request.URL.String(), resp.Body)
	if err != nil {
		return Result{}, err
	}

	best, err := BestCandidate(candidates)
	if err != nil {
		return Result{}, err
	}

	result, err := DownloadIcon(ctx, f.Client, best.URL, outputDir)
	if err != nil {
		return Result{}, err
	}

	result.InputURL = rawURL
	result.PageURL = resp.Request.URL.String()
	result.IconURL = best.URL
	result.SourceRel = best.Rel
	result.FallbackUsed = best.Rel == "fallback"
	return result, nil
}
