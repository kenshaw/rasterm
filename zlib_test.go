package rasterm

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"io"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestKittyZlib(t *testing.T) {
	flat := flatImage(256, 256)    // 256 KiB of RGBA that compresses well
	noise := noiseImage(256, 256)  // 256 KiB of RGBA that does not compress
	small := flatImage(64, 64)     // 16 KiB, under zlibMin
	noise64 := noiseImage(256, 64) // 64 KiB, one probe
	flatTop := flatOverNoise(256, 256, 64)
	tests := []struct {
		name string
		img  image.Image
		mode ZlibMode
		z    bool
	}{
		{"auto flat rgba", RGBA{Image: flat, Data: packRGBA(flat)}, ZlibAuto, true},
		{"auto flat rgb", RGB{Image: flat, Data: packRGB(flat)}, ZlibAuto, true},
		{"auto noise rgba", RGBA{Image: noise, Data: packRGBA(noise)}, ZlibAuto, false},
		{"auto small", RGBA{Image: small, Data: packRGBA(small)}, ZlibAuto, false},
		// exactly one probe of noise is checked, and stays raw
		{"auto noise one probe", RGBA{Image: noise64, Data: packRGBA(noise64)}, ZlibAuto, false},
		// a flat top over a noise bottom compresses at the first check, and
		// goes raw at a later one
		{"auto flat top", RGBA{Image: flatTop, Data: packRGBA(flatTop)}, ZlibAuto, false},
		{"always noise", RGBA{Image: noise, Data: packRGBA(noise)}, ZlibAlways, true},
		{"always small", RGB{Image: small, Data: packRGB(small)}, ZlibAlways, true},
		{"never flat", RGBA{Image: flat, Data: packRGBA(flat)}, ZlibNever, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := (KittyEncoder{Zlib: test.mode}).Encode(&buf, test.img); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			keys, data := decodeKitty(t, buf.String())
			if z := keys["o"] == "z"; z != test.z {
				t.Fatalf("expected o=z to be %t, got keys: %v", test.z, keys)
			}
			var want []byte
			switch img := test.img.(type) {
			case RGB:
				want = img.Data
			case RGBA:
				want = img.Data
			}
			if !bytes.Equal(data, want) {
				t.Errorf("expected the sequence to decode to the %d bytes of pixels, got %d bytes", len(want), len(data))
			}
			if b := test.img.Bounds(); keys["s"] != strconv.Itoa(b.Dx()) || keys["v"] != strconv.Itoa(b.Dy()) {
				t.Errorf("expected s=%d and v=%d, got: %v", b.Dx(), b.Dy(), keys)
			}
		})
	}
}

func TestKittyZlibNotForPNG(t *testing.T) {
	// a png is compressed already, and is sent as its bytes in every mode
	flat := flatImage(256, 256)
	data := mustPNG(t, flat)
	for _, mode := range []ZlibMode{ZlibAuto, ZlibAlways} {
		var buf bytes.Buffer
		if err := (KittyEncoder{Zlib: mode}).Encode(&buf, PNG{Image: flat, Data: data}); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if want := string(appendKitty(nil, kittyPNG, 0, 0, false, data)) + "\n"; buf.String() != want {
			t.Errorf("mode %d: expected the png as is, with no o=z", mode)
		}
	}
}

// kittyKeyRE matches a key and a value in the control data of a Kitty escape
// sequence.
var kittyKeyRE = regexp.MustCompile(`([a-zA-Z])=([^,;]*)`)

// decodeKitty returns the keys of the first Kitty escape sequence in s, and
// the data of all of the sequences, base64 decoded and, for o=z, inflated.
// It fails unless each chunk but the last has m=1 and the last has m=0.
func decodeKitty(t *testing.T, s string) (map[string]string, []byte) {
	t.Helper()
	seqs := regexp.MustCompile("\x1b_G([^;]*);([^\x1b]*)\x1b\\\\").FindAllStringSubmatch(s, -1)
	if len(seqs) < 2 {
		t.Fatalf("expected a start and at least one chunk, got %d sequences", len(seqs))
	}
	keys := make(map[string]string)
	for _, m := range kittyKeyRE.FindAllStringSubmatch(seqs[0][1], -1) {
		keys[m[1]] = m[2]
	}
	var b64 strings.Builder
	for i, seq := range seqs[1:] {
		want := "m=1"
		if i == len(seqs)-2 {
			want = "m=0"
		}
		if seq[1] != want {
			t.Fatalf("chunk %d: expected %s, got: %s", i, want, seq[1])
		}
		b64.WriteString(seq[2])
	}
	data, err := base64.StdEncoding.DecodeString(b64.String())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if keys["o"] == "z" {
		zr, err := zlib.NewReader(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if data, err = io.ReadAll(zr); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	}
	return keys, data
}

// flatImage returns an image of w x h pixels in bands of a few colors, as a
// rendered page or an icon is.
func flatImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x / 32 * 40), G: uint8(y / 32 * 40), B: 0x80, A: 0xff})
		}
	}
	return img
}

// flatOverNoise returns an image of w x h pixels whose top rows are flat and
// whose other rows are noise, as a photo with a clear sky is.
func flatOverNoise(w, h, top int) *image.NRGBA {
	img := noiseImage(w, h)
	draw.Draw(img, image.Rect(0, 0, w, top), flatImage(w, top), image.Point{}, draw.Src)
	return img
}

// noiseImage returns an image of w x h pixels of random colors, which zlib
// cannot compress.
func noiseImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	r := rand.New(rand.NewPCG(1, 2))
	for i := range img.Pix {
		img.Pix[i] = uint8(r.Uint32())
	}
	return img
}

// BenchmarkKittyZlib sends the raw pixels of each test file, and of a flat
// and a noise image, with each zlib mode. bytes/op is the size that the
// terminal reads.
func BenchmarkKittyZlib(b *testing.B) {
	type input struct {
		name string
		img  image.Image
	}
	inputs := []input{
		{"flat.rgba", flatImage(1024, 1024)},
		{"noise.rgba", noiseImage(1024, 1024)},
	}
	for _, f := range testfiles(b) {
		inputs = append(inputs, input{f.name, f.img})
	}
	modes := []struct {
		name string
		mode ZlibMode
	}{{"auto", ZlibAuto}, {"always", ZlibAlways}, {"never", ZlibNever}}
	for _, in := range inputs {
		img := RGBA{Image: in.img, Data: packRGBA(in.img)}
		for _, m := range modes {
			b.Run(m.name+"/"+in.name, func(b *testing.B) {
				benchEncode(b, KittyEncoder{Zlib: m.mode}, img)
			})
		}
	}
}
