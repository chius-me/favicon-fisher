package convert

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"
)

func TestConvertPNGToJPG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode PNG fixture: %v", err)
	}

	result, err := Convert(buf.Bytes(), "image/png", "icon.png", "jpg")
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	if result.ContentType != "image/jpeg" {
		t.Fatalf("expected image/jpeg, got %q", result.ContentType)
	}
	if filepath.Ext(result.Filename) != ".jpg" {
		t.Fatalf("expected .jpg filename, got %q", result.Filename)
	}
	if len(result.Data) == 0 {
		t.Fatal("expected converted bytes")
	}

	_, format, err := image.DecodeConfig(bytes.NewReader(result.Data))
	if err != nil {
		t.Fatalf("decode converted image: %v", err)
	}
	if format != "jpeg" {
		t.Fatalf("expected jpeg payload, got %q", format)
	}
}

func TestConvertPassesThroughSVG(t *testing.T) {
	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"></svg>`)

	result, err := Convert(svg, "image/svg+xml", "icon.svg", "svg")
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}

	if result.ContentType != "image/svg+xml" {
		t.Fatalf("expected image/svg+xml, got %q", result.ContentType)
	}
	if result.Filename != "icon.svg" {
		t.Fatalf("expected icon.svg filename, got %q", result.Filename)
	}
	if string(result.Data) != string(svg) {
		t.Fatal("expected SVG bytes to pass through unchanged")
	}
}
