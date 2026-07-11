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
