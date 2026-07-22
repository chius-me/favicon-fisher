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
	"github.com/chius-me/favicon-fisher/internal/security"
)

type Handler struct {
	client  *http.Client
	fetcher *fetcher.Fetcher
	signer  *security.Signer
	policy  security.Policy
}

// HandlerOptions configures the web API handler.
type HandlerOptions struct {
	Client *http.Client
	Signer *security.Signer
	// Policy defaults to DefaultPolicy (SSRF-safe, no private IPs).
	Policy *security.Policy
}

func NewHandler(client *http.Client) *Handler {
	return NewHandlerWithOptions(HandlerOptions{Client: client})
}

func NewHandlerWithOptions(opts HandlerOptions) *Handler {
	policy := security.DefaultPolicy
	if opts.Policy != nil {
		policy = *opts.Policy
	}
	client := opts.Client
	if client == nil {
		client = security.SafeHTTPClient(security.ClientOptions{
			Timeout: 15 * time.Second,
			Policy:  policy,
		})
	}
	signer := opts.Signer
	if signer == nil {
		signer = security.NewSigner("", security.DefaultTokenTTL)
	}
	return &Handler{
		client:  client,
		fetcher: fetcher.NewWithPolicy(client, policy),
		signer:  signer,
		policy:  policy,
	}
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	var req PreviewRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.URL) == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "url is required"})
		return
	}

	result, err := h.fetcher.Preview(r.Context(), req.URL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: security.PublicError(err)})
		return
	}

	icons := make([]IconPreview, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		contentType := candidate.Type
		if contentType == "" {
			contentType = detectContentTypeFromURL(candidate.URL)
		}
		allowed := allowedTypesFor(candidate.URL, contentType)
		if len(allowed) == 0 {
			continue
		}
		icons = append(icons, IconPreview{
			IconURL:      candidate.URL,
			Token:        h.signer.Sign(candidate.URL),
			SourceRel:    candidate.Rel,
			Sizes:        candidate.Sizes,
			ContentType:  contentType,
			AllowedTypes: allowed,
		})
	}
	if len(icons) == 0 {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "no favicon candidates found"})
		return
	}

	recommended := result.Best.URL
	found := false
	for _, icon := range icons {
		if icon.IconURL == recommended {
			found = true
			break
		}
	}
	if !found {
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
	if err := decodeJSONBody(r, &req); err != nil {
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
	if err := h.signer.Verify(req.IconURL, req.Token); err != nil {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: security.PublicError(err)})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp, err := h.downloadSource(ctx, req.IconURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: security.PublicError(err)})
		return
	}
	defer resp.Body.Close()

	body, err := security.LimitedReadAll(resp.Body, security.MaxIconBody)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: security.PublicError(err)})
		return
	}

	filename := sourceFilename(req.IconURL)
	converted, err := convert.Convert(body, resp.Header.Get("Content-Type"), filename, req.Format)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: security.PublicError(err)})
		return
	}

	w.Header().Set("Content-Type", converted.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", converted.Filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(converted.Data)
}

// Proxy serves a signed same-origin preview of a remote icon (no conversion).
// Prevents the browser from fetching arbitrary (including private) icon URLs directly.
func (h *Handler) Proxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	iconURL := strings.TrimSpace(r.URL.Query().Get("url"))
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if iconURL == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "url parameter is required"})
		return
	}
	if token == "" {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: "token is required"})
		return
	}
	if err := h.signer.Verify(iconURL, token); err != nil {
		writeJSON(w, http.StatusForbidden, errorResponse{Error: security.PublicError(err)})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	resp, err := h.downloadSource(ctx, iconURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: security.PublicError(err)})
		return
	}
	defer resp.Body.Close()

	body, err := security.LimitedReadAll(resp.Body, security.MaxIconBody)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: security.PublicError(err)})
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=300")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) downloadSource(ctx context.Context, iconURL string) (*http.Response, error) {
	req, err := security.NewRequestWithPolicy(ctx, http.MethodGet, iconURL, h.policy)
	if err != nil {
		return nil, err
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
	if i := strings.IndexAny(base, "?#"); i >= 0 {
		base = base[:i]
	}
	if base == "." || base == "/" || base == "" {
		return "icon"
	}
	return base
}

func detectContentTypeFromURL(raw string) string {
	lower := strings.ToLower(raw)
	if i := strings.IndexAny(lower, "?#"); i >= 0 {
		lower = lower[:i]
	}
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".svg"):
		return "image/svg+xml"
	case strings.HasSuffix(lower, ".ico"):
		return "image/x-icon"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	default:
		return ""
	}
}

func decodeJSONBody(r *http.Request, dst any) error {
	defer r.Body.Close()
	limited := io.LimitReader(r.Body, security.MaxJSONBody+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if int64(len(data)) > security.MaxJSONBody {
		return fmt.Errorf("request body too large")
	}
	return json.Unmarshal(data, dst)
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, errorResponse{Error: "method not allowed"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
