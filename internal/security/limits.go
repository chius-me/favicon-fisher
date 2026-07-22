package security

import (
	"fmt"
	"io"
)

const (
	// MaxHTMLBody is the maximum page HTML size read during discovery.
	MaxHTMLBody int64 = 5 << 20 // 5 MiB
	// MaxIconBody is the maximum icon binary size accepted for download/proxy.
	MaxIconBody int64 = 5 << 20 // 5 MiB
	// MaxManifestBody is the maximum web manifest size.
	MaxManifestBody int64 = 1 << 20 // 1 MiB
	// MaxJSONBody is the maximum inbound API JSON request size.
	MaxJSONBody int64 = 64 << 10 // 64 KiB

	// MaxHTMLIconCandidates caps <link rel=icon…> entries from a single page.
	MaxHTMLIconCandidates = 32
	// MaxManifestIcons caps icons[] entries from a web manifest.
	MaxManifestIcons = 32
	// MaxTotalCandidates is the hard cap after merge + fallback.
	MaxTotalCandidates = 48
	// MaxContentTypeProbes limits HEAD/GET probes during preview.
	MaxContentTypeProbes = 12
	// MaxRedirectHops is the maximum number of redirects followed per request.
	MaxRedirectHops = 5

	// MaxImageDimension is the maximum width or height accepted for decode.
	MaxImageDimension = 4096
	// MaxImagePixels is the maximum width×height accepted for decode.
	MaxImagePixels = 16_000_000
)

// LimitedReadAll reads at most limit bytes from r and errors if more data is present.
func LimitedReadAll(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("invalid limit")
	}
	limited := io.LimitReader(r, limit+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response body exceeds %d bytes", limit)
	}
	return data, nil
}

// LimitReader wraps r so at most limit bytes can be read (for streaming copies).
func LimitReader(r io.Reader, limit int64) io.Reader {
	return io.LimitReader(r, limit+1)
}

// CopyLimited copies from src to dst with a hard size cap.
func CopyLimited(dst io.Writer, src io.Reader, limit int64) (int64, error) {
	written, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if err != nil {
		return written, err
	}
	if written > limit {
		return written, fmt.Errorf("response body exceeds %d bytes", limit)
	}
	return written, nil
}
