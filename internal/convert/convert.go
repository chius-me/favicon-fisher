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
)

type Result struct {
	Data        []byte
	ContentType string
	Filename    string
}

func Convert(data []byte, contentType string, filename string, format string) (Result, error) {
	target := normalizeFormat(format)
	if target == "" {
		return Result{}, fmt.Errorf("format is required")
	}

	sourceExt := normalizeFormat(strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), "."))
	if sourceExt == "jpeg" {
		sourceExt = "jpg"
	}
	if sourceExt == "svg" {
		if target != "svg" {
			return Result{}, fmt.Errorf("svg sources currently support svg output only")
		}
		return Result{
			Data:        data,
			ContentType: "image/svg+xml",
			Filename:    replaceExt(filename, ".svg"),
		}, nil
	}
	if sourceExt == "ico" {
		if target != "ico" {
			return Result{}, fmt.Errorf("ico sources currently support ico output only")
		}
		return Result{
			Data:        data,
			ContentType: contentTypeOrDefault(contentType, "image/x-icon"),
			Filename:    replaceExt(filename, ".ico"),
		}, nil
	}

	if target == "svg" {
		if isSVG(contentType, sourceExt, data) {
			return Result{
				Data:        data,
				ContentType: "image/svg+xml",
				Filename:    replaceExt(filename, ".svg"),
			}, nil
		}
		return Result{}, fmt.Errorf("svg output is only supported for svg sources")
	}

	if target == sourceExt && target != "" && target != "jpg" && target != "png" {
		return Result{
			Data:        data,
			ContentType: contentType,
			Filename:    filename,
		}, nil
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

func normalizeFormat(format string) string {
	format = strings.TrimSpace(strings.ToLower(format))
	format = strings.TrimPrefix(format, ".")
	if format == "jpeg" {
		return "jpg"
	}
	return format
}

func isSVG(contentType string, sourceExt string, data []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "image/svg+xml") || sourceExt == "svg" {
		return true
	}
	trimmed := strings.TrimSpace(string(data))
	return strings.HasPrefix(trimmed, "<svg") || strings.HasPrefix(trimmed, "<?xml") && strings.Contains(trimmed, "<svg")
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
