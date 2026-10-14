package rasterm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"
)

func TestAppendKittyMatchesChunkEncode(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3, 3071, 3072, 3073, 6144, 10000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			data := make([]byte, n)
			for i := range data {
				data[i] = byte(i * 7)
			}
			var want bytes.Buffer
			if err := chunkEncode(&want, data); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			got := appendKitty(nil, kittyPNG, 0, 0, data)
			if !bytes.Equal(got, want.Bytes()) {
				t.Errorf("expected the output of chunkEncode, got:\n%q\nwant:\n%q", got, want.Bytes())
			}
			if len(got) > kittyLen(n) {
				t.Errorf("expected at most kittyLen %d, got: %d", kittyLen(n), len(got))
			}
		})
	}
}

// chunkEncode is the Kitty encoder before the output was built in one buffer.
// It base64 encodes all of buf, and then writes it in chunks of 4096 bytes.
func chunkEncode(w io.Writer, buf []byte) error {
	b64 := []byte(base64.StdEncoding.EncodeToString(buf))
	if _, err := fmt.Fprintf(w, "\x1b_Ga=T,f=100,m=1;\x1b\\"); err != nil {
		return err
	}
	n := len(b64)
	for i, j, m := 0, min(4096, n), 0; i < n; i, j = j, min(j+4096, n) {
		if m = 0; j < n {
			m = 1
		}
		if _, err := fmt.Fprintf(w, "\x1b_Gm=%d;%s\x1b\\", m, b64[i:j]); err != nil {
			return err
		}
	}
	return nil
}

func TestEncodeWritesOnce(t *testing.T) {
	files := testfiles(t)
	for _, typ := range []TermType{Kitty, ITerm, Sixel, Halfblock} {
		for _, f := range files {
			t.Run(typ.String()+"/"+f.name, func(t *testing.T) {
				var w countWriter
				if err := encoders[typ].Encode(&w, f.img); err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				if w.writes != 1 {
					t.Errorf("expected 1 write, got: %d", w.writes)
				}
			})
		}
	}
}

func TestPNGIsSentAsIs(t *testing.T) {
	for _, f := range testfiles(t) {
		if f.typ != "png" {
			continue
		}
		t.Run(f.name, func(t *testing.T) {
			p, err := DecodePNG(f.buf)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			b64 := base64.StdEncoding.EncodeToString(f.buf)
			var kitty bytes.Buffer
			if err := encoders[Kitty].Encode(&kitty, p); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if want := string(appendKitty(nil, kittyPNG, 0, 0, f.buf)) + "\n"; kitty.String() != want {
				t.Errorf("expected kitty to send the PNG as is")
			}
			var iterm bytes.Buffer
			if err := encoders[ITerm].Encode(&iterm, p); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if want := itermStart + b64 + "\a\n"; iterm.String() != want {
				t.Errorf("expected iterm to send the PNG as is")
			}
			// sixel cannot send a png, and must see the image that p wraps
			var a, b bytes.Buffer
			if err := encoders[Sixel].Encode(&a, p); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if err := encoders[Sixel].Encode(&b, p.Image); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if !bytes.Equal(a.Bytes(), b.Bytes()) {
				t.Errorf("expected sixel to encode the image that the PNG wraps")
			}
		})
	}
}

func TestPNGThatDoesNotMatchIsEncoded(t *testing.T) {
	var src bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	img.Set(1, 1, color.NRGBA{R: 0xff, A: 0xff})
	if err := encodePNG(&src, img); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	tests := []struct {
		name string
		img  image.Image
	}{
		{"not a png", PNG{Image: img, Data: []byte("not a png at all, not at all")}},
		{"empty", PNG{Image: img}},
		{"other size", PNG{Image: image.NewNRGBA(image.Rect(0, 0, 8, 8)), Data: src.Bytes()}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			data, err := pngData(&buf, test.img)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if !strings.HasPrefix(string(data), pngSignature) {
				t.Fatalf("expected a png, got: %q", data)
			}
			if !pngMatches(data, test.img.Bounds()) {
				t.Errorf("expected a png of size %v", test.img.Bounds().Size())
			}
			if bytes.Equal(data, test.img.(PNGImage).PNG()) {
				t.Errorf("expected the image to be encoded again")
			}
		})
	}
}

func TestPNGFastKeepsPixels(t *testing.T) {
	for _, f := range testfiles(t) {
		switch f.img.(type) {
		case *image.YCbCr, *image.CMYK:
		default:
			continue
		}
		t.Run(f.name, func(t *testing.T) {
			fast := pngFast(f.img)
			if _, ok := fast.(*image.RGBA); !ok {
				t.Fatalf("expected *image.RGBA, got: %T", fast)
			}
			b := f.img.Bounds()
			for y := b.Min.Y; y < b.Max.Y; y += 7 {
				for x := b.Min.X; x < b.Max.X; x += 7 {
					if want, got := color.RGBAModel.Convert(f.img.At(x, y)), fast.At(x, y); want != got {
						t.Fatalf("(%d, %d): expected %v, got: %v", x, y, want, got)
					}
				}
			}
		})
	}
}

// countWriter counts the calls to Write and the bytes written, and discards
// the bytes.
type countWriter struct {
	writes int
	n      int
}

// Write satisfies the [io.Writer] interface.
func (w *countWriter) Write(buf []byte) (int, error) {
	w.writes++
	w.n += len(buf)
	return len(buf), nil
}

func BenchmarkEncode(b *testing.B) {
	files := testfiles(b)
	for _, typ := range []TermType{Kitty, ITerm, Sixel, Halfblock} {
		for _, f := range files {
			b.Run(typ.String()+"/"+f.name, func(b *testing.B) {
				benchEncode(b, encoders[typ], f.img)
			})
		}
	}
}

// BenchmarkEncodePNG encodes each PNG test file wrapped in a [PNG], so that
// Kitty and iTerm send the bytes of the file as they are.
func BenchmarkEncodePNG(b *testing.B) {
	files := testfiles(b)
	for _, typ := range []TermType{Kitty, ITerm} {
		for _, f := range files {
			if f.typ != "png" {
				continue
			}
			p, err := DecodePNG(f.buf)
			if err != nil {
				b.Fatalf("expected no error, got: %v", err)
			}
			b.Run(typ.String()+"/"+f.name, func(b *testing.B) {
				benchEncode(b, encoders[typ], p)
			})
		}
	}
}

func benchEncode(b *testing.B, enc Encoder, img image.Image) {
	b.Helper()
	var w countWriter
	b.ReportAllocs()
	for b.Loop() {
		if err := enc.Encode(&w, img); err != nil {
			b.Fatalf("expected no error, got: %v", err)
		}
	}
	b.ReportMetric(float64(w.writes)/float64(b.N), "writes/op")
	b.ReportMetric(float64(w.n)/float64(b.N), "bytes/op")
}

// BenchmarkPNGCompression encodes each test file with the Kitty encoder at
// each PNG compression level. bytes/op is the size of the output.
func BenchmarkPNGCompression(b *testing.B) {
	files := testfiles(b)
	levels := []struct {
		name  string
		level png.CompressionLevel
	}{
		{"default", png.DefaultCompression},
		{"speed", png.BestSpeed},
		{"none", png.NoCompression},
	}
	prev := DefaultPNGCompression
	b.Cleanup(func() { DefaultPNGCompression = prev })
	for _, l := range levels {
		for _, f := range files {
			b.Run(l.name+"/"+f.name, func(b *testing.B) {
				DefaultPNGCompression = l.level
				benchEncode(b, encoders[Kitty], f.img)
			})
		}
	}
}
