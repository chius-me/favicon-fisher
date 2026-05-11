package fetcher

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func DownloadIcon(ctx context.Context, client *http.Client, iconURL string, outputDir string, sizeHint string, relHint string) (IconResult, error) {
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, iconURL, nil)
	if err != nil {
		return IconResult{}, fmt.Errorf("build icon request: %w", err)
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
	
	// Create suffix based on size and rel to avoid conflicts when downloading all
	suffix := ""
	if sizeHint != "" {
		suffix = "-" + strings.ReplaceAll(sizeHint, " ", "-")
	} else if relHint != "" && relHint != "icon" && relHint != "shortcut icon" {
		suffix = "-" + strings.ReplaceAll(relHint, " ", "-")
	}
	
	filename := safeFilename(parsedURL.Hostname()+suffix, ext)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return IconResult{}, fmt.Errorf("create output dir: %w", err)
	}

	outputPath := filepath.Join(outputDir, filename)
	file, err := os.Create(outputPath)
	if err != nil {
		return IconResult{}, fmt.Errorf("create output file: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, resp.Body)
	if err != nil {
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

func inferExtension(urlPath string, contentType string) string {
	ext := strings.ToLower(path.Ext(urlPath))
	if ext != "" {
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
		}
	}

	return ".bin"
}

func safeFilename(host string, ext string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = "favicon"
	}
	replacer := strings.NewReplacer(":", "-", "/", "-", "\\", "-", " ", "-")
	return replacer.Replace(host) + ext
}
