package convert

import (
	"bytes"
	"image"
	"image/png"

	"golang.org/x/image/draw"
)

// Thumbnail normalizes supported rasters to a bounded PNG for the native UI.
func Thumbnail(data []byte) ([]byte, error) {
	if err := assertDecodableImageLimits(data); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w > 512 || h > 512 {
		if w >= h {
			h = max(1, h*512/w)
			w = 512
		} else {
			w = max(1, w*512/h)
			h = 512
		}
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)
		img = dst
	}
	var out bytes.Buffer
	err = png.Encode(&out, img)
	return out.Bytes(), err
}
