// Package engine exposes the versioned, single-request desktop helper protocol.
package engine

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/chius-me/favicon-fisher/internal/convert"
	"github.com/chius-me/favicon-fisher/internal/fetcher"
	"github.com/chius-me/favicon-fisher/internal/security"
)

const ProtocolVersion = 1

var Version = "dev"

type Request struct {
	ProtocolVersion int    `json:"protocol_version"`
	ID              string `json:"id"`
	Op              string `json:"op"`
	URL             string `json:"url,omitempty"`
	CacheDir        string `json:"cache_dir,omitempty"`
	Source          string `json:"source,omitempty"`
	Format          string `json:"format,omitempty"`
	TimeoutMS       int    `json:"timeout_ms,omitempty"`
	AllowFakeIP     bool   `json:"allow_fake_ip,omitempty"`
}
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Response struct {
	ProtocolVersion int      `json:"protocol_version"`
	ID              string   `json:"id"`
	OK              bool     `json:"ok"`
	Result          any      `json:"result,omitempty"`
	Error           *Failure `json:"error,omitempty"`
}
type Candidate struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	Source        string `json:"source"`
	DeclaredSizes string `json:"declared_sizes"`
}
type Discovery struct {
	PageURL    string      `json:"page_url"`
	Candidates []Candidate `json:"candidates"`
}
type Artifact struct {
	Path           string   `json:"path"`
	PreviewPath    string   `json:"preview_path,omitempty"`
	Format         string   `json:"format"`
	Width          int      `json:"width"`
	Height         int      `json:"height"`
	Bytes          int      `json:"bytes"`
	AllowedFormats []string `json:"allowed_formats"`
}
type Service struct {
	Client *http.Client
	Policy security.Policy
}

func New() *Service { return NewWithFakeIP(false) }

func NewWithFakeIP(allow bool) *Service {
	// Explicit direct networking: desktop does not inherit proxy environment variables.
	policy := security.Policy{AllowFakeIP: allow}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &Service{Client: security.SafeHTTPClient(security.ClientOptions{Transport: transport, Policy: policy, Timeout: 10 * time.Second}), Policy: policy}
}

func (s *Service) Handle(ctx context.Context, req Request) Response {
	response := Response{ProtocolVersion: ProtocolVersion, ID: req.ID}
	fail := func(code, message string) Response { response.Error = &Failure{code, message}; return response }
	if req.ProtocolVersion != ProtocolVersion {
		return fail("PROTOCOL_MISMATCH", "The app and engine versions are incompatible.")
	}
	if req.ID == "" || len(req.ID) > 128 {
		return fail("INVALID_REQUEST", "A request ID is required.")
	}
	timeout := req.TimeoutMS
	if timeout <= 0 || timeout > 30000 {
		timeout = 15000
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()
	var result any
	var err error
	switch req.Op {
	case "capabilities":
		result = map[string]any{"engine_version": Version, "source_formats": []string{"png", "jpg", "gif", "webp", "svg", "ico"}, "output_formats": []string{"original", "png", "jpg"}}
	case "discover":
		var normalized string
		normalized, err = fetcher.NormalizeInputURL(req.URL)
		if err != nil {
			return fail("INVALID_URL", "Enter a valid website URL.")
		}
		if _, err = security.ValidateFetchURLWithContext(ctx, normalized, s.Policy); err == nil {
			f := fetcher.NewWithPolicy(s.Client, s.Policy)
			f.SkipProbes = true
			var preview fetcher.PreviewResult
			preview, err = f.Preview(ctx, normalized)
			if err == nil {
				d := Discovery{PageURL: preview.PageURL, Candidates: []Candidate{}}
				for i, c := range preview.Candidates {
					d.Candidates = append(d.Candidates, Candidate{fmt.Sprintf("icon-%d", i), c.URL, c.Rel, c.Sizes})
				}
				result = d
			}
		}
	case "fetch", "export":
		result, err = s.artifact(ctx, req)
	default:
		return fail("INVALID_REQUEST", "Unknown engine operation.")
	}
	if err != nil {
		var networkErr net.Error
		switch {
		case errors.Is(err, context.Canceled):
			return fail("CANCELLED", "The operation was cancelled.")
		case errors.Is(err, context.DeadlineExceeded), errors.As(err, &networkErr) && networkErr.Timeout():
			return fail("NETWORK_TIMEOUT", "The website took too long to respond.")
		default:
			code := "FETCH_FAILED"
			if req.Op == "export" {
				code = "EXPORT_FAILED"
			}
			return fail(code, security.PublicError(err))
		}
	}
	response.OK = true
	response.Result = result
	return response
}

func (s *Service) artifact(ctx context.Context, req Request) (Artifact, error) {
	var result Artifact
	if !filepath.IsAbs(req.CacheDir) {
		return result, fmt.Errorf("absolute cache directory required")
	}
	stat, err := os.Lstat(req.CacheDir)
	if err != nil {
		return result, fmt.Errorf("cache unavailable")
	}
	if !stat.IsDir() || stat.Mode()&os.ModeSymlink != 0 {
		return result, fmt.Errorf("invalid cache directory")
	}
	root, err := os.OpenRoot(req.CacheDir)
	if err != nil {
		return result, fmt.Errorf("cache unavailable")
	}
	defer root.Close()
	var data []byte
	var info convert.Info
	if req.Op == "fetch" {
		data, info, err = fetcher.ReadIcon(ctx, s.Client, s.Policy, req.URL)
	} else {
		if req.Source == "" || req.Source == "." || filepath.Base(req.Source) != req.Source {
			return result, fmt.Errorf("invalid source path")
		}
		var stat os.FileInfo
		stat, err = root.Lstat(req.Source)
		if err != nil || !stat.Mode().IsRegular() {
			return result, fmt.Errorf("invalid source file")
		}
		var file *os.File
		file, err = root.Open(req.Source)
		if err != nil {
			return result, fmt.Errorf("source unavailable")
		}
		defer file.Close()
		data, err = security.LimitedReadAll(file, security.MaxIconBody)
		if err == nil {
			info, err = convert.Inspect(data)
		}
		if err == nil && req.Format != "original" {
			if req.Format != "png" && req.Format != "jpg" {
				return result, fmt.Errorf("unsupported output format")
			}
			var converted convert.Result
			converted, err = convert.Convert(data, "", "icon."+info.Format, req.Format)
			if err == nil {
				data = converted.Data
				info.Format = req.Format
			}
		}
	}
	if err != nil {
		return result, err
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	result = Artifact{Format: info.Format, Width: info.Width, Height: info.Height, Bytes: len(data), AllowedFormats: []string{"original"}}
	result.Path, err = writeArtifact(root, data, info.Format)
	if err != nil {
		return result, err
	}
	if info.Format != "svg" && info.Format != "ico" {
		result.AllowedFormats = append(result.AllowedFormats, "png", "jpg")
		// A PNG preview normalizes GIF/WebP and prevents platform decoder differences.
		preview, e := convert.Thumbnail(data)
		if e != nil {
			return result, e
		}
		result.PreviewPath, err = writeArtifact(root, preview, "png")
	} else if info.Format == "ico" {
		result.PreviewPath = result.Path
	}
	return result, err
}

func writeArtifact(root *os.Root, data []byte, ext string) (string, error) {
	var key [12]byte
	if _, err := rand.Read(key[:]); err != nil {
		return "", err
	}
	name := hex.EncodeToString(key[:]) + "." + ext
	file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", fmt.Errorf("cannot write cache")
	}
	_, err = file.Write(data)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		_ = root.Remove(name)
		return "", fmt.Errorf("cannot write cache")
	}
	return name, nil
}
