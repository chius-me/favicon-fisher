package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/chius-me/favicon-fisher/internal/convert"
	"github.com/chius-me/favicon-fisher/internal/fetcher"
)

type Handler struct {
	client  *http.Client
	fetcher *fetcher.Fetcher
}

func NewHandler(client *http.Client) *Handler {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Handler{
		client:  client,
		fetcher: fetcher.New(client),
	}
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "url is required"})
		return
	}

	result, err := h.fetcher.Preview(r.Context(), req.URL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
		return
	}

	icons := make([]IconPreview, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		contentType := detectContentTypeFromURL(candidate.URL)
		allowed := allowedTypesFor(candidate.URL, contentType)
		if len(allowed) == 0 {
			continue
		}
		icons = append(icons, IconPreview{
			IconURL:      candidate.URL,
			SourceRel:    candidate.Rel,
			Sizes:        candidate.Sizes,
			ContentType:  contentType,
			AllowedTypes: allowed,
		})
	}
	if len(icons) == 0 {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "no previewable icons found"})
		return
	}

	recommended := result.Best.URL
	if len(allowedTypesFor(result.Best.URL, detectContentTypeFromURL(result.Best.URL))) == 0 {
		recommended = icons[0].IconURL
	}

	writeJSON(w, http.StatusOK, PreviewResponse{
		InputURL:           result.InputURL,
		PageURL:            result.PageURL,
		RecommendedIconURL: recommended,
		Icons:              icons,
	})
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.IconURL) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "icon_url is required"})
		return
	}
	if strings.TrimSpace(req.Format) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "format is required"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp, err := h.downloadSource(ctx, req.IconURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: err.Error()})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: fmt.Sprintf("read icon body: %v", err)})
		return
	}

	filename := sourceFilename(req.IconURL)
	converted, err := convert.Convert(body, resp.Header.Get("Content-Type"), filename, req.Format)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	w.Header().Set("Content-Type", converted.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", converted.Filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(converted.Data)
}

func (h *Handler) downloadSource(ctx context.Context, iconURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, iconURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build icon request: %w", err)
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download icon: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("download icon failed: status %d", resp.StatusCode)
	}
	return resp, nil
}

func sourceFilename(iconURL string) string {
	base := path.Base(iconURL)
	if base == "." || base == "/" || base == "" {
		return "icon"
	}
	if strings.Contains(base, "?") {
		base = strings.Split(base, "?")[0]
	}
	if base == "" {
		return "icon"
	}
	return base
}

func detectContentTypeFromURL(raw string) string {
	switch strings.ToLower(path.Ext(raw)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	default:
		return ""
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
