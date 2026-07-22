package convert

import "testing"

func FuzzSniffFormat(f *testing.F) {
	f.Add([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})
	f.Add([]byte{0xff, 0xd8, 0xff})
	f.Add([]byte("GIF89a"))
	f.Add([]byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>"))
	f.Add([]byte("not an image"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = SniffFormat(data)
	})
}

func FuzzConvert(f *testing.F) {
	f.Add([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}, "png")
	f.Add([]byte("<svg></svg>"), "svg")
	f.Add([]byte("hello"), "png")
	f.Fuzz(func(t *testing.T, data []byte, format string) {
		// Convert must not panic; errors are expected for garbage input.
		_, _ = Convert(data, "application/octet-stream", "icon.bin", format)
	})
}
