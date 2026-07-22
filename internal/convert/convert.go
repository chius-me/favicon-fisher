package convert

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/chius-me/favicon-fisher/internal/security"
)

type Result struct {
	Data        []byte
	ContentType string
	Filename    string
}

func Convert(data []byte, contentType string, filename string, format string) (Result, error) {
	target := normalizeFormat(format)
	switch target {
	case "png", "jpg", "svg", "ico":
	case "":
		return Result{}, fmt.Errorf("format is required")
	default:
		return Result{}, fmt.Errorf("unsupported output format: %s", format)
	}

	sourceExt := normalizeFormat(strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), "."))
	if sourceExt == "jpeg" {
		sourceExt = "jpg"
	}

	sniffed := SniffFormat(data)

	// Prefer magic-byte detection over filename extension when they conflict.
	if sniffed != "" {
		sourceExt = sniffed
	}

	// SVG passthrough only when content actually looks like SVG.
	if target == "svg" || sourceExt == "svg" {
		if sniffed != "svg" {
			return Result{}, fmt.Errorf("svg output is only supported for svg sources")
		}
		if target != "svg" {
			return Result{}, fmt.Errorf("svg sources currently support svg output only")
		}
		return Result{
			Data:        data,
			ContentType: "image/svg+xml",
			Filename:    replaceExt(filename, ".svg"),
		}, nil
	}

	// ICO passthrough only when ICONDIR header matches.
	if target == "ico" || sourceExt == "ico" {
		if sniffed != "ico" {
			return Result{}, fmt.Errorf("ico output is only supported for ico sources")
		}
		if target != "ico" {
			return Result{}, fmt.Errorf("ico sources currently support ico output only")
		}
		return Result{
			Data:        data,
			ContentType: contentTypeOrDefault(contentType, "image/x-icon"),
			Filename:    replaceExt(filename, ".ico"),
		}, nil
	}

	// Require a recognizable raster format before decode.
	switch sniffed {
	case "png", "jpg", "gif", "webp":
	case "":
		return Result{}, fmt.Errorf("unrecognized image format")
	default:
		return Result{}, fmt.Errorf("unsupported source format: %s", sniffed)
	}

	if err := assertDecodableImageLimits(data); err != nil {
		return Result{}, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{}, fmt.Errorf("decode source image: %w", err)
	}

	var buf bytes.Buffer
	switch target {
	case "png":
		if err := png.Encode(&buf, img); err != nil {
			return Result{}, fmt.Errorf("encode png: %w", err)
		}
		return Result{
			Data:        buf.Bytes(),
			ContentType: "image/png",
			Filename:    replaceExt(filename, ".png"),
		}, nil
	case "jpg":
		flattened := flattenToWhite(img)
		if err := jpeg.Encode(&buf, flattened, &jpeg.Options{Quality: 90}); err != nil {
			return Result{}, fmt.Errorf("encode jpeg: %w", err)
		}
		return Result{
			Data:        buf.Bytes(),
			ContentType: "image/jpeg",
			Filename:    replaceExt(filename, ".jpg"),
		}, nil
	default:
		return Result{}, fmt.Errorf("unsupported output format: %s", format)
	}
}

// assertDecodableImageLimits rejects decompression bombs before full decode.
func assertDecodableImageLimits(data []byte) error {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode source config: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("image dimensions exceed limits")
	}
	if cfg.Width > security.MaxImageDimension || cfg.Height > security.MaxImageDimension {
		return fmt.Errorf("image dimensions exceed limits")
	}
	pixels := int64(cfg.Width) * int64(cfg.Height)
	if pixels > security.MaxImagePixels {
		return fmt.Errorf("image dimensions exceed limits")
	}
	return nil
}

// SniffFormat returns a short format name (png, jpg, gif, webp, ico, svg) from magic bytes.
// Empty string means unknown / non-image.
func SniffFormat(data []byte) string {
	if looksLikePNG(data) {
		return "png"
	}
	if looksLikeJPEG(data) {
		return "jpg"
	}
	if looksLikeGIF(data) {
		return "gif"
	}
	if looksLikeWebP(data) {
		return "webp"
	}
	if looksLikeICO(data) {
		return "ico"
	}
	if looksLikeSVG(data) {
		return "svg"
	}
	return ""
}

func looksLikePNG(data []byte) bool {
	return len(data) >= 8 &&
		data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4e && data[3] == 0x47 &&
		data[4] == 0x0d && data[5] == 0x0a && data[6] == 0x1a && data[7] == 0x0a
}

func looksLikeJPEG(data []byte) bool {
	return len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff
}

func looksLikeGIF(data []byte) bool {
	return len(data) >= 6 &&
		(string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a")
}

func looksLikeWebP(data []byte) bool {
	return len(data) >= 12 &&
		string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP"
}

func looksLikeSVG(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	// Reject DOCTYPE/entity-heavy payloads early; require an <svg root-ish marker.
	if strings.Contains(lower, "<!doctype") || strings.Contains(lower, "<!entity") {
		return false
	}
	if strings.HasPrefix(lower, "<svg") {
		return true
	}
	if strings.HasPrefix(lower, "<?xml") && strings.Contains(lower, "<svg") {
		return true
	}
	return false
}

func looksLikeICO(data []byte) bool {
	// ICONDIR: reserved(0) + type(1=ICO) + count(>=1)
	if len(data) < 6 {
		return false
	}
	if data[0] != 0 || data[1] != 0 {
		return false
	}
	if data[2] != 1 || data[3] != 0 {
		return false
	}
	count := int(data[4]) | int(data[5])<<8
	return count >= 1 && count <= 64
}

func normalizeFormat(format string) string {
	format = strings.TrimSpace(strings.ToLower(format))
	format = strings.TrimPrefix(format, ".")
	if format == "jpeg" {
		return "jpg"
	}
	return format
}

func replaceExt(filename string, ext string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	if base == "" {
		base = "icon"
	}
	return base + ext
}

func contentTypeOrDefault(contentType string, fallback string) string {
	if strings.TrimSpace(contentType) == "" {
		return fallback
	}
	return contentType
}

func flattenToWhite(src image.Image) image.Image {
	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)
	draw.Draw(dst, bounds, &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	draw.Draw(dst, bounds, src, bounds.Min, draw.Over)
	return dst
}
