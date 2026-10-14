package rasterm

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

func TestHalfblockEncode(t *testing.T) {
	red := color.NRGBA{R: 0xff, A: 0xff}
	blue := color.NRGBA{B: 0xff, A: 0xff}
	faint := color.NRGBA{R: 0xff, A: 0x10}
	tests := []struct {
		name string
		w, h int
		pix  []color.NRGBA
		exp  string
	}{
		{
			"two pixels", 1, 2,
			[]color.NRGBA{red, blue},
			"\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m▀\x1b[0m\n",
		},
		{
			"same colors are set once", 2, 2,
			[]color.NRGBA{red, red, blue, blue},
			"\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m▀▀\x1b[0m\n",
		},
		{
			"transparent bottom", 1, 2,
			[]color.NRGBA{red, faint},
			"\x1b[38;2;255;0;0m▀\x1b[0m\n",
		},
		{
			"transparent top", 1, 2,
			[]color.NRGBA{faint, blue},
			"\x1b[38;2;0;0;255m▄\x1b[0m\n",
		},
		{
			"transparent after a color", 2, 2,
			[]color.NRGBA{red, faint, blue, faint},
			"\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m▀\x1b[49m \x1b[0m\n",
		},
		{
			"odd height", 1, 3,
			[]color.NRGBA{red, blue, red},
			"\x1b[38;2;255;0;0m\x1b[48;2;0;0;255m▀\x1b[0m\n\x1b[38;2;255;0;0m▀\x1b[0m\n",
		},
		{
			"empty", 0, 0,
			nil,
			"\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// the bounds do not start at the origin, as after a crop
			img := image.NewNRGBA(image.Rect(5, 7, 5+test.w, 7+test.h))
			for i, c := range test.pix {
				img.SetNRGBA(5+i%test.w, 7+i/test.w, c)
			}
			var buf bytes.Buffer
			if err := encoders[Halfblock].Encode(&buf, img); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if s := buf.String(); s != test.exp {
				t.Errorf("expected:\n%q\ngot:\n%q", test.exp, s)
			}
		})
	}
}

func TestHalfblockNoNewline(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 2))
	var buf bytes.Buffer
	if err := (HalfblockEncoder{NoNewline: true}).Encode(&buf, img); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if exp, s := " \x1b[0m", buf.String(); s != exp {
		t.Errorf("expected %q, got: %q", exp, s)
	}
}

func TestHalfblockMatchesSlowPath(t *testing.T) {
	// a paletted image goes through draw.Draw, and must give the same output
	// as the same pixels in an *image.NRGBA
	for _, f := range testfiles(t) {
		t.Run(f.name, func(t *testing.T) {
			b := f.img.Bounds()
			slow := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
			for y := b.Min.Y; y < b.Max.Y; y++ {
				for x := b.Min.X; x < b.Max.X; x++ {
					slow.Set(x-b.Min.X, y-b.Min.Y, f.img.At(x, y))
				}
			}
			var a, c bytes.Buffer
			if err := encoders[Halfblock].Encode(&a, f.img); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if err := encoders[Halfblock].Encode(&c, slow); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if !bytes.Equal(a.Bytes(), c.Bytes()) {
				t.Errorf("expected the same output from %T and *image.NRGBA", f.img)
			}
		})
	}
}

func TestHalfblockTermType(t *testing.T) {
	var typ TermType
	if err := typ.UnmarshalText([]byte("halfblock")); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if typ != Halfblock {
		t.Fatalf("expected Halfblock, got: %v", typ)
	}
	buf, err := typ.MarshalText()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if s := string(buf); s != "halfblock" {
		t.Errorf("expected halfblock, got: %q", s)
	}
}
