package rasterm

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"strings"
	"testing"
)

func TestJPEGIsSentAsIs(t *testing.T) {
	for _, f := range testfiles(t) {
		if f.typ != "jpeg" {
			continue
		}
		t.Run(f.name, func(t *testing.T) {
			j, err := DecodeJPEG(f.buf)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			var iterm bytes.Buffer
			if err := encoders[ITerm].Encode(&iterm, j); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if want := itermStart + base64.StdEncoding.EncodeToString(f.buf) + "\a\n"; iterm.String() != want {
				t.Errorf("expected iterm to send the JPEG as is")
			}
			// the other encoders cannot send a jpeg, and must see the image
			// that j wraps
			for _, typ := range []TermType{Kitty, Sixel, Halfblock} {
				if !sameOutput(t, typ, j, j.Image) {
					t.Errorf("expected %s to encode the image that the JPEG wraps", typ)
				}
			}
		})
	}
}

func TestJPEGMatches(t *testing.T) {
	var jpg []byte
	var bounds image.Rectangle
	for _, f := range testfiles(t) {
		if f.typ == "jpeg" {
			jpg, bounds = f.buf, f.img.Bounds()
			break
		}
	}
	if jpg == nil {
		t.Fatal("expected a jpeg in testdata")
	}
	if !jpegMatches(jpg, bounds) {
		t.Fatalf("expected the jpeg to match its bounds %v", bounds)
	}
	if jpegMatches(jpg, bounds.Inset(1)) {
		t.Errorf("expected the jpeg not to match other bounds")
	}
	// a fill byte before a marker is allowed
	filled := append([]byte{0xff, 0xd8, 0xff}, jpg[2:]...)
	if !jpegMatches(filled, bounds) {
		t.Errorf("expected a fill byte to be skipped")
	}
	// each prefix that ends before the frame header must fail, and not panic
	end := 0
	for n := range len(jpg) {
		if jpegMatches(jpg[:n], bounds) {
			end = n
			break
		}
	}
	if end == 0 {
		t.Fatal("expected a prefix of the jpeg to hold the frame header")
	}
	for _, data := range [][]byte{nil, {0xff}, {0xff, 0xd8}, []byte("not a jpeg"), {0xff, 0xd8, 0xff, 0xd9}} {
		if jpegMatches(data, bounds) {
			t.Errorf("expected %q not to match", data)
		}
	}
}

func FuzzJPEGMatches(f *testing.F) {
	f.Add([]byte{0xff, 0xd8, 0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x02, 0x00, 0x03, 0x01, 0x01, 0x11, 0x00})
	f.Add([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x02, 0xff, 0xc2, 0x00, 0x07, 0x08, 0x00, 0x02, 0x00, 0x03})
	f.Fuzz(func(t *testing.T, data []byte) {
		jpegMatches(data, image.Rect(0, 0, 3, 2))
	})
}

func TestRawIsSentAsIs(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for i := range 6 {
		img.SetNRGBA(i%3, i/3, color.NRGBA{R: uint8(i * 40), G: 0x80, B: uint8(255 - i*40), A: uint8(0x40 + i*30)})
	}
	rgb, rgba := packRGB(img), packRGBA(img)
	tests := []struct {
		name   string
		img    image.Image
		format int
		data   []byte
		others []TermType
	}{
		{"rgb", RGB{Image: img, Data: rgb}, kittyRGB, rgb, []TermType{ITerm, Sixel, Halfblock}},
		{"rgba", RGBA{Image: img, Data: rgba}, kittyRGBA, rgba, []TermType{ITerm, Sixel, Halfblock}},
		// iterm sends the png, so only sixel and half blocks see the image
		{"png first", pngAndRGB{RGB{Image: img, Data: rgb}, mustPNG(t, img)}, kittyPNG, mustPNG(t, img), []TermType{Sixel, Halfblock}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := encoders[Kitty].Encode(&buf, test.img); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			want := string(appendKitty(nil, test.format, 3, 2, false, test.data)) + "\n"
			if buf.String() != want {
				t.Errorf("expected:\n%q\ngot:\n%q", want, buf.String())
			}
			// the other encoders cannot send raw pixels, and must see the
			// image that is wrapped
			for _, typ := range test.others {
				if !sameOutput(t, typ, test.img, img) {
					t.Errorf("expected %s to encode the wrapped image", typ)
				}
			}
		})
	}
	if s := string(appendKitty(nil, kittyRGB, 3, 2, false, nil)); !strings.HasPrefix(s, "\x1b_Ga=T,f=24,s=3,v=2,m=1;") {
		t.Errorf("expected the width and height in the start of a raw image, got: %q", s)
	}
}

func TestRawThatDoesNotMatchIsEncoded(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 3, 2))
	for _, wrapped := range []image.Image{
		RGB{Image: img, Data: make([]byte, 3*6-1)},
		RGBA{Image: img, Data: make([]byte, 3*6)},
		RGBA{Image: img, Data: make([]byte, 4*6+1)},
	} {
		var buf bytes.Buffer
		if err := encoders[Kitty].Encode(&buf, wrapped); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if !strings.HasPrefix(buf.String(), "\x1b_Ga=T,f=100,m=1;") {
			t.Errorf("%T: expected a png, got: %.40q", wrapped, buf.String())
		}
	}
}

// pngAndRGB is an image that holds both a PNG and raw pixels.
type pngAndRGB struct {
	RGB

	png []byte
}

// PNG satisfies the [PNGImage] interface.
func (p pngAndRGB) PNG() []byte {
	return p.png
}

// sameOutput returns true when the encoder for typ writes the same bytes for a
// and b.
func sameOutput(t *testing.T, typ TermType, a, b image.Image) bool {
	t.Helper()
	var x, y bytes.Buffer
	if err := encoders[typ].Encode(&x, a); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if err := encoders[typ].Encode(&y, b); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return bytes.Equal(x.Bytes(), y.Bytes())
}

func mustPNG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := encodePNG(&buf, img); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return buf.Bytes()
}

// packRGB returns the pixels of img as [RGBImage] describes them.
func packRGB(img image.Image) []byte {
	b := img.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy()*3)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			out = append(out, c.R, c.G, c.B)
		}
	}
	return out
}

// packRGBA returns the pixels of img as [RGBAImage] describes them.
func packRGBA(img image.Image) []byte {
	b := img.Bounds()
	out := make([]byte, 0, b.Dx()*b.Dy()*4)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			out = append(out, c.R, c.G, c.B, c.A)
		}
	}
	return out
}

// BenchmarkEncodeJPEG encodes each JPEG test file wrapped in a [JPEG], so
// that iTerm sends the bytes of the file as they are.
func BenchmarkEncodeJPEG(b *testing.B) {
	for _, f := range testfiles(b) {
		if f.typ != "jpeg" {
			continue
		}
		j, err := DecodeJPEG(f.buf)
		if err != nil {
			b.Fatalf("expected no error, got: %v", err)
		}
		b.Run("iterm/"+f.name, func(b *testing.B) {
			benchEncode(b, encoders[ITerm], j)
		})
	}
}

// BenchmarkEncodeRaw encodes each test file wrapped in an [RGB] and in an
// [RGBA], so that Kitty sends the pixels as they are. bytes/op shows the size
// that the terminal reads, which is larger than a PNG.
func BenchmarkEncodeRaw(b *testing.B) {
	for _, f := range testfiles(b) {
		rgb := RGB{Image: f.img, Data: packRGB(f.img)}
		rgba := RGBA{Image: f.img, Data: packRGBA(f.img)}
		b.Run("kitty/rgb/"+f.name, func(b *testing.B) {
			benchEncode(b, encoders[Kitty], rgb)
		})
		b.Run("kitty/rgba/"+f.name, func(b *testing.B) {
			benchEncode(b, encoders[Kitty], rgba)
		})
	}
}
