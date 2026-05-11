package fetcher

type Candidate struct {
	URL      string
	Rel      string
	Sizes    string
	Type     string
	Priority int
}

type Result struct {
	InputURL     string       `json:"input_url"`
	PageURL      string       `json:"page_url"`
	IconURL      string       `json:"icon_url"`
	OutputPath   string       `json:"output_path"`
	ContentType  string       `json:"content_type"`
	Bytes        int64        `json:"bytes"`
	StatusCode   int          `json:"status_code"`
	Filename     string       `json:"filename"`
	SourceRel    string       `json:"source_rel"`
	FallbackUsed bool         `json:"fallback_used"`
	AllIcons     []IconResult `json:"all_icons,omitempty"`
}

type IconResult struct {
	IconURL     string `json:"icon_url"`
	OutputPath  string `json:"output_path"`
	ContentType string `json:"content_type"`
	Bytes       int64  `json:"bytes"`
	StatusCode  int    `json:"status_code"`
	Filename    string `json:"filename"`
	SourceRel   string `json:"source_rel"`
	Sizes       string `json:"sizes,omitempty"`
}
