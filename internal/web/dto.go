package web

import "strings"

type PreviewRequest struct {
	URL string `json:"url"`
}

type IconPreview struct {
	IconURL      string   `json:"icon_url"`
	SourceRel    string   `json:"source_rel"`
	Sizes        string   `json:"sizes,omitempty"`
	ContentType  string   `json:"content_type,omitempty"`
	AllowedTypes []string `json:"allowed_types,omitempty"`
}

type PreviewResponse struct {
	InputURL           string        `json:"input_url"`
	PageURL            string        `json:"page_url"`
	RecommendedIconURL string        `json:"recommended_icon_url"`
	Icons              []IconPreview `json:"icons"`
}

type DownloadRequest struct {
	IconURL string `json:"icon_url"`
	Format  string `json:"format"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func allowedTypesFor(iconURL string, contentType string) []string {
	urlLower := strings.ToLower(iconURL)
	contentLower := strings.ToLower(contentType)

	if strings.HasSuffix(urlLower, ".svg") || strings.Contains(contentLower, "image/svg+xml") {
		return []string{"svg"}
	}
	if strings.HasSuffix(urlLower, ".ico") || strings.Contains(contentLower, "image/x-icon") || strings.Contains(contentLower, "image/vnd.microsoft.icon") {
		return []string{"ico"}
	}
	if strings.HasSuffix(urlLower, ".jpg") || strings.HasSuffix(urlLower, ".jpeg") || strings.Contains(contentLower, "image/jpeg") {
		return []string{"jpg", "png"}
	}
	if strings.HasSuffix(urlLower, ".png") || strings.Contains(contentLower, "image/png") {
		return []string{"png", "jpg"}
	}
	if strings.HasSuffix(urlLower, ".gif") || strings.Contains(contentLower, "image/gif") {
		return []string{"png", "jpg"}
	}
	return nil
}
