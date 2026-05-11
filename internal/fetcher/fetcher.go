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

func (f *Fetcher) Fetch(ctx context.Context, rawURL string, outputDir string, fetchAll bool) (Result, error) {
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

	result := Result{
		InputURL: rawURL,
		PageURL:  resp.Request.URL.String(),
	}

	if fetchAll {
		var allIcons []IconResult
		downloadedURLs := make(map[string]bool)
		
		for _, candidate := range candidates {
			if downloadedURLs[candidate.URL] {
				continue
			}
			iconRes, err := DownloadIcon(ctx, f.Client, candidate.URL, outputDir, candidate.Sizes, candidate.Rel)
			if err == nil {
				allIcons = append(allIcons, iconRes)
				downloadedURLs[candidate.URL] = true
				
				// Set the best one as the primary icon in Result
				if candidate.URL == best.URL {
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
		
		// If best failed but others succeeded, fallback to the first successful one
		if result.IconURL == "" && len(allIcons) > 0 {
			first := allIcons[0]
			result.IconURL = first.IconURL
			result.OutputPath = first.OutputPath
			result.ContentType = first.ContentType
			result.Bytes = first.Bytes
			result.StatusCode = first.StatusCode
			result.Filename = first.Filename
			result.SourceRel = first.SourceRel
			result.FallbackUsed = first.SourceRel == "fallback"
		}
		
	} else {
		iconRes, err := DownloadIcon(ctx, f.Client, best.URL, outputDir, "", "")
		if err != nil {
			return Result{}, err
		}

		result.IconURL = iconRes.IconURL
		result.OutputPath = iconRes.OutputPath
		result.ContentType = iconRes.ContentType
		result.Bytes = iconRes.Bytes
		result.StatusCode = iconRes.StatusCode
		result.Filename = iconRes.Filename
		result.SourceRel = iconRes.SourceRel
		result.FallbackUsed = best.Rel == "fallback"
	}

	return result, nil
}
