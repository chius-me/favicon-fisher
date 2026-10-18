package fetcher

import (
	"context"
	"fmt"
	"net/http"

	"github.com/chius-me/favicon-fisher/internal/convert"
	"github.com/chius-me/favicon-fisher/internal/security"
)

// ReadIcon is shared by downloads and the desktop engine. It never trusts MIME or suffix.
func ReadIcon(ctx context.Context, client *http.Client, policy security.Policy, rawURL string) ([]byte, convert.Info, error) {
	var info convert.Info
	req, err := security.NewRequestWithPolicy(ctx, http.MethodGet, rawURL, policy)
	if err != nil {
		return nil, info, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, info, fmt.Errorf("download icon failed: status %d", resp.StatusCode)
	}
	data, err := security.LimitedReadAll(resp.Body, security.MaxIconBody)
	if err != nil {
		return nil, info, err
	}
	info, err = convert.Inspect(data)
	return data, info, err
}
