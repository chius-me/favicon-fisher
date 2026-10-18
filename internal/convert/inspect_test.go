package convert

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"
)

func TestWebPDecodeAndConvert(t *testing.T) {
	// A one-pixel VP8 image: exercises a real decoder, not just the RIFF signature.
	data, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	info, err := Inspect(data)
	if err != nil {
		t.Fatal(err)
	}
	if info.Format != "webp" || info.Width != 1 || info.Height != 1 {
		t.Fatalf("%+v", info)
	}
	result, err := Convert(data, "image/webp", "one.webp", "png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(result.Data)); err != nil {
		t.Fatal(err)
	}
}
func TestInspectRejectsMalformedImages(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("<html>login page</html>"),
		[]byte("<svgBroken></svgBroken>"),
		[]byte("<svg><g></svg>"),
		[]byte("<svg></svg><svg></svg>"),
		{0, 0, 1, 0, 1, 0},
		{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a},
	} {
		if _, err := Inspect(data); err == nil {
			t.Fatalf("accepted malformed %q", data)
		}
	}
}
func TestInspectRejectsDimensionsBeforeDecode(t *testing.T) {
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4097, 1))); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(b.Bytes()); err == nil {
		t.Fatal("accepted oversized image")
	}
}
func TestThumbnailBounds(t *testing.T) {
	var b bytes.Buffer
	_ = png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 1024, 256)))
	data, err := Thumbnail(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 512 || config.Height != 128 {
		t.Fatalf("%+v", config)
	}
}
