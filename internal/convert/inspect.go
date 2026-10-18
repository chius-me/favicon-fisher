package convert

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"strings"

	"github.com/chius-me/favicon-fisher/internal/security"
)

type Info struct {
	Format string `json:"format"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Inspect validates bounded raster data before it is saved or presented to a decoder.
// SVG is parsed as XML, never rendered here. ICO validates its directory and frames.
func Inspect(data []byte) (Info, error) {
	info := Info{Format: SniffFormat(data)}
	if len(data) > int(security.MaxIconBody) {
		return info, fmt.Errorf("image exceeds size limit")
	}
	switch info.Format {
	case "svg":
		dec := xml.NewDecoder(bytes.NewReader(data))
		depth, roots := 0, 0
		for {
			token, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return info, fmt.Errorf("invalid SVG")
			}
			switch v := token.(type) {
			case xml.Directive:
				return info, fmt.Errorf("SVG directives are not supported")
			case xml.StartElement:
				if depth == 0 {
					roots++
					if v.Name.Local != "svg" {
						return info, fmt.Errorf("invalid SVG root")
					}
				}
				depth++
				if depth > 128 {
					return info, fmt.Errorf("SVG nesting exceeds limit")
				}
			case xml.EndElement:
				depth--
			case xml.CharData:
				if depth == 0 && strings.TrimSpace(string(v)) != "" {
					return info, fmt.Errorf("invalid SVG content")
				}
			}
		}
		if roots != 1 || depth != 0 {
			return info, fmt.Errorf("invalid SVG")
		}
	case "ico":
		count := int(binary.LittleEndian.Uint16(data[4:6]))
		if len(data) < 6+16*count {
			return info, fmt.Errorf("truncated ICO directory")
		}
		for i := 0; i < count; i++ {
			entry := data[6+16*i : 6+16*(i+1)]
			w, h := int(entry[0]), int(entry[1])
			if w == 0 {
				w = 256
			}
			if h == 0 {
				h = 256
			}
			size, offset := uint64(binary.LittleEndian.Uint32(entry[8:12])), uint64(binary.LittleEndian.Uint32(entry[12:16]))
			if offset < uint64(6+16*count) || size < 8 || offset+size > uint64(len(data)) {
				return info, fmt.Errorf("invalid ICO frame bounds")
			}
			frame := data[offset : offset+size]
			if looksLikePNG(frame) {
				frameInfo, err := Inspect(frame)
				if err != nil || frameInfo.Width != w || frameInfo.Height != h {
					return info, fmt.Errorf("invalid ICO PNG frame")
				}
			} else {
				if len(frame) < 40 {
					return info, fmt.Errorf("invalid ICO bitmap")
				}
				header := int(binary.LittleEndian.Uint32(frame[:4]))
				bw, bh := int(int32(binary.LittleEndian.Uint32(frame[4:8]))), int(int32(binary.LittleEndian.Uint32(frame[8:12])))
				bits := int(binary.LittleEndian.Uint16(frame[14:16]))
				if header < 40 || header > len(frame) || bw != w || bh != h*2 || binary.LittleEndian.Uint16(frame[12:14]) != 1 || binary.LittleEndian.Uint32(frame[16:20]) != 0 {
					return info, fmt.Errorf("invalid ICO bitmap dimensions")
				}
				if bits != 1 && bits != 4 && bits != 8 && bits != 24 && bits != 32 {
					return info, fmt.Errorf("unsupported ICO bitmap")
				}
				palette := 0
				if bits <= 8 {
					palette = 1 << bits
					if used := int(binary.LittleEndian.Uint32(frame[32:36])); used > 0 {
						if used > palette {
							return info, fmt.Errorf("invalid ICO palette")
						}
						palette = used
					}
				}
				if header+palette*4+((w*bits+31)/32)*4*h > len(frame) {
					return info, fmt.Errorf("truncated ICO bitmap")
				}
			}
			if w*h > info.Width*info.Height {
				info.Width = w
				info.Height = h
			}
		}
	case "png", "jpg", "gif", "webp":
		if err := assertDecodableImageLimits(data); err != nil {
			return info, err
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return info, fmt.Errorf("invalid image: %w", err)
		}
		info.Width = img.Bounds().Dx()
		info.Height = img.Bounds().Dy()
	default:
		return info, fmt.Errorf("unrecognized image format")
	}
	return info, nil
}
