package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/chius-me/favicon-fisher/internal/security"
)

func DownloadIcon(ctx context.Context, client *http.Client, iconURL string, outputDir string, sizeHint string, relHint string) (IconResult, error) {
	return DownloadIconWithPolicy(ctx, client, security.CLIPolicy, iconURL, outputDir, sizeHint, relHint, 0)
}

func DownloadIconWithPolicy(ctx context.Context, client *http.Client, policy security.Policy, iconURL string, outputDir string, sizeHint string, relHint string, index int) (IconResult, error) {
	if client == nil {
		client = security.SafeHTTPClient(security.ClientOptions{
			Timeout: 15 * time.Second,
			Policy:  policy,
		})
	}

	req, err := security.NewRequestWithPolicy(ctx, http.MethodGet, iconURL, policy)
	if err != nil {
		return IconResult{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return IconResult{}, fmt.Errorf("download icon: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return IconResult{}, fmt.Errorf("download icon failed: status %d", resp.StatusCode)
	}

	parsedURL, err := url.Parse(iconURL)
	if err != nil {
		return IconResult{}, fmt.Errorf("parse icon URL: %w", err)
	}

	ext := inferExtension(parsedURL.Path, resp.Header.Get("Content-Type"))
	filename := uniqueIconFilename(parsedURL.Hostname(), index, parsedURL.Path, sizeHint, relHint, ext)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return IconResult{}, fmt.Errorf("create output dir: %w", err)
	}

	outputPath := filepath.Join(outputDir, filename)
	file, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		// Collision: append short counter until unique.
		for n := 2; n < 100; n++ {
			alt := strings.TrimSuffix(filename, ext) + fmt.Sprintf("-%d", n) + ext
			altPath := filepath.Join(outputDir, alt)
			f2, err2 := os.OpenFile(altPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
			if err2 == nil {
				file = f2
				filename = alt
				outputPath = altPath
				err = nil
				break
			}
		}
		if err != nil {
			return IconResult{}, fmt.Errorf("create output file: %w", err)
		}
	}
	defer file.Close()

	written, err := security.CopyLimited(file, resp.Body, security.MaxIconBody)
	if err != nil {
		_ = os.Remove(outputPath)
		return IconResult{}, fmt.Errorf("write output file: %w", err)
	}

	return IconResult{
		IconURL:     iconURL,
		OutputPath:  outputPath,
		ContentType: resp.Header.Get("Content-Type"),
		Bytes:       written,
		StatusCode:  resp.StatusCode,
		Filename:    filename,
		SourceRel:   relHint,
		Sizes:       sizeHint,
	}, nil
}

// uniqueIconFilename builds a non-colliding name:
// hostname-01-basename-sizeHint-hash.ext
func uniqueIconFilename(host string, index int, urlPath, sizeHint, relHint, ext string) string {
	if host == "" {
		host = "favicon"
	}
	base := path.Base(urlPath)
	if i := strings.IndexAny(base, "?#"); i >= 0 {
		base = base[:i]
	}
	base = strings.TrimSuffix(base, path.Ext(base))
	if base == "" || base == "." || base == "/" {
		base = "icon"
	}

	parts := []string{host}
	if index > 0 {
		parts = append(parts, fmt.Sprintf("%02d", index))
	}
	parts = append(parts, base)
	if sizeHint != "" {
		parts = append(parts, strings.ReplaceAll(sizeHint, " ", "-"))
	} else if relHint != "" && relHint != "icon" && relHint != "shortcut icon" {
		parts = append(parts, strings.ReplaceAll(relHint, " ", "-"))
	}
	// Short content-addressed suffix from full path keeps same-basename icons distinct.
	sum := sha256.Sum256([]byte(urlPath))
	parts = append(parts, hex.EncodeToString(sum[:3]))

	return safeFilename(strings.Join(parts, "-"), ext)
}

func inferExtension(urlPath string, contentType string) string {
	ext := strings.ToLower(path.Ext(urlPath))
	if ext != "" && len(ext) <= 5 {
		return ext
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil {
		switch mediaType {
		case "image/png":
			return ".png"
		case "image/x-icon", "image/vnd.microsoft.icon":
			return ".ico"
		case "image/svg+xml":
			return ".svg"
		case "image/jpeg":
			return ".jpg"
		case "image/webp":
			return ".webp"
		case "image/gif":
			return ".gif"
		}
	}

	return ".bin"
}

func safeFilename(host string, ext string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "favicon"
	}
	replacer := strings.NewReplacer(
		":", "-", "/", "-", "\\", "-", " ", "-",
		"..", ".", "\x00", "",
	)
	name := replacer.Replace(host)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			return '-'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		name = "favicon"
	}
	if len(name) > 180 {
		name = name[:180]
	}
	return name + ext
}
