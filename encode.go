package rasterm

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/mattn/go-sixel"
)

// DefaultJPEGQuality is the default JPEG encode quality.
var DefaultJPEGQuality = 93

// Encoder provides a common interface for terminal graphic encoders.
type Encoder interface {
	Available() bool
	Encode(io.Writer, image.Image) error
}

// KittyEncoder is a Kitty terminal graphics encoder.
type KittyEncoder struct {
	NoNewline bool
}

// NewKittyEncoder creates a Kitty terminal graphics encoder.
//
// See: https://sw.kovidgoyal.net/kitty/graphics-protocol.html
func NewKittyEncoder() Encoder {
	return KittyEncoder{}
}

// Available satisfies the [Encoder] interface.
func (KittyEncoder) Available() bool {
	return !otherTermGraphics("kitty") &&
		(hasTermGraphics("kitty") ||
			(os.Getenv("KITTY_PID") != "" && os.Getenv("KITTY_WINDOW_ID") != "") ||
			strings.ToLower(os.Getenv("TERM")) == "xterm-kitty" ||
			strings.ToLower(os.Getenv("TERM_PROGRAM")) == "ghostty")
}

// Encode satisfies the [Encoder] interface.
//
// If img is a [PNGImage], an [RGBImage] or an [RGBAImage], its bytes are
// sent as they are, in that order of preference. Otherwise the image is sent
// as a PNG. The output is built in memory and written to w in one call.
func (r KittyEncoder) Encode(w io.Writer, img image.Image) error {
	src := getBuffer()
	defer putBuffer(src)
	format, data, err := kittyData(src, img)
	if err != nil {
		return err
	}
	out := getBuffer()
	defer putBuffer(out)
	out.Grow(kittyLen(len(data)) + 1)
	b := img.Bounds()
	buf := appendKitty(out.AvailableBuffer(), format, b.Dx(), b.Dy(), data)
	if !r.NoNewline {
		buf = append(buf, '\n')
	}
	_, err = w.Write(buf)
	return err
}

// ITermEncoder is a iTerm terminal graphics encoder.
//
// See: https://iterm2.com/documentation-images.html
type ITermEncoder struct {
	NoNewline bool
}

// NewITermEncoder creates a iTerm terminal graphics encoder.
func NewITermEncoder() Encoder {
	return ITermEncoder{}
}

// Available satisfies the [Encoder] interface.
func (ITermEncoder) Available() bool {
	return !otherTermGraphics("iterm") &&
		(hasTermGraphics("iterm") ||
			strings.ToLower(os.Getenv("TERM")) == "mintty" ||
			strings.ToLower(os.Getenv("LC_TERMINAL")) == "iterm2" ||
			strings.ToLower(os.Getenv("TERM_PROGRAM")) == "wezterm")
}

// Encode satisfies the [Encoder] interface.
//
// If img is a [PNGImage] or a [JPEGImage], its bytes are sent as they are.
// Otherwise a paletted image is sent as a PNG, and any other image as a JPEG.
// The output is built in memory and written to w in one call.
func (r ITermEncoder) Encode(w io.Writer, img image.Image) error {
	src := getBuffer()
	defer putBuffer(src)
	data, err := itermData(src, img)
	if err != nil {
		return err
	}
	out := getBuffer()
	defer putBuffer(out)
	out.Grow(len(itermStart) + base64.StdEncoding.EncodedLen(len(data)) + 2)
	buf := append(out.AvailableBuffer(), itermStart...)
	buf = base64.StdEncoding.AppendEncode(buf, data)
	buf = append(buf, '\a')
	if !r.NoNewline {
		buf = append(buf, '\n')
	}
	_, err = w.Write(buf)
	return err
}

// itermStart starts an iTerm inline image.
const itermStart = "\x1b]1337;File=inline=1:"

// itermData returns the file that the iTerm encoder sends for img.
func itermData(buf *bytes.Buffer, img image.Image) ([]byte, error) {
	if data, ok := pngOf(img); ok {
		return data, nil
	}
	if data, ok := jpegOf(img); ok {
		return data, nil
	}
	img = unwrap(img)
	if _, ok := img.(*image.Paletted); ok {
		return pngData(buf, img)
	}
	if err := jpegEncode(buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SixelEncoder is a Sixel terminal graphics encoder.
//
// See: https://saitoha.github.io/libsixel/
type SixelEncoder struct {
	NoNewline bool
}

// NewSixelEncoder creates a Sixel terminal graphics encoder.
func NewSixelEncoder() Encoder {
	return SixelEncoder{}
}

// Available satisfies the [Encoder] interface.
func (SixelEncoder) Available() bool {
	return !otherTermGraphics("sixel") && (hasTermGraphics("sixel") || hasSixelSupport())
}

// Encode satisfies the [Encoder] interface.
//
// The output and the newline are built in memory and written to w in one
// call. go-sixel before v0.0.14 made a call to Write for each few bytes when w
// was not an [os.File].
func (r SixelEncoder) Encode(w io.Writer, img image.Image) error {
	buf := getBuffer()
	defer putBuffer(buf)
	if err := sixel.NewEncoder(buf).Encode(unwrap(img)); err != nil {
		return err
	}
	if !r.NoNewline {
		buf.WriteByte('\n')
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// DefaultEncoder wraps multiple terminal graphic encoders.
type DefaultEncoder struct {
	v    []Encoder
	r    Encoder
	once sync.Once
}

// NewDefaultEncoder creates a wrapper for multiple terminal graphic encoders.
func NewDefaultEncoder(v ...Encoder) *DefaultEncoder {
	return &DefaultEncoder{
		v: v,
	}
}

// init selects the first available encoder. It leaves r.r nil when none is
// available.
func (r *DefaultEncoder) init() {
	for _, z := range r.v {
		if z.Available() {
			r.r = z
			return
		}
	}
}

// Available satisfies the [Encoder] interface.
func (r *DefaultEncoder) Available() bool {
	r.once.Do(r.init)
	return r.r != nil
}

// Encode satisfies the [Encoder] interface.
func (r *DefaultEncoder) Encode(w io.Writer, img image.Image) error {
	if r.once.Do(r.init); r.r == nil {
		return ErrTermGraphicsNotAvailable
	}
	return r.r.Encode(w, img)
}

// jpegEncode encodes a image to w as a jpeg using [DefaultJPEGQuality].
func jpegEncode(w io.Writer, img image.Image) error {
	return jpeg.Encode(w, img, &jpeg.Options{
		Quality: DefaultJPEGQuality,
	})
}

// The formats of the Kitty protocol.
const (
	kittyRGB  = 24
	kittyRGBA = 32
	kittyPNG  = 100
)

// kittyData returns the Kitty format of img and the bytes to send for it.
func kittyData(buf *bytes.Buffer, img image.Image) (int, []byte, error) {
	if data, ok := pngOf(img); ok {
		return kittyPNG, data, nil
	}
	if data, ok := rgbOf(img); ok {
		return kittyRGB, data, nil
	}
	if data, ok := rgbaOf(img); ok {
		return kittyRGBA, data, nil
	}
	data, err := pngData(buf, img)
	return kittyPNG, data, err
}

// kittyStartMax is the length of the longest start of a Kitty image, with a
// width and a height of 10 digits each.
const kittyStartMax = len("\x1b_Ga=T,f=100,s=,v=,m=1;\x1b\\") + 20

// kittyChunk is the number of bytes of PNG in each chunk. Its base64
// encoding is 4096 bytes, which is the largest chunk that the Kitty protocol
// allows.
const kittyChunk = 3072

// kittyLen returns the largest length of the output of [appendKitty] for n
// bytes of data.
func kittyLen(n int) int {
	chunks := (n + kittyChunk - 1) / kittyChunk
	return kittyStartMax + chunks*len("\x1b_Gm=0;\x1b\\") + base64.StdEncoding.EncodedLen(n)
}

// appendKitty appends the Kitty escape sequences that send data to buf. The
// first sequence names the format, and for raw pixels the width w and the
// height h. Each chunk after it holds the base64 encoding of [kittyChunk]
// bytes of data.
func appendKitty(buf []byte, format, w, h int, data []byte) []byte {
	buf = append(buf, "\x1b_Ga=T,f="...)
	buf = strconv.AppendInt(buf, int64(format), 10)
	if format != kittyPNG {
		buf = append(buf, ",s="...)
		buf = strconv.AppendInt(buf, int64(w), 10)
		buf = append(buf, ",v="...)
		buf = strconv.AppendInt(buf, int64(h), 10)
	}
	buf = append(buf, ",m=1;\x1b\\"...)
	for i := 0; i < len(data); i += kittyChunk {
		j, m := min(i+kittyChunk, len(data)), byte('1')
		if j == len(data) {
			m = '0'
		}
		buf = append(buf, "\x1b_Gm="...)
		buf = append(buf, m, ';')
		buf = base64.StdEncoding.AppendEncode(buf, data[i:j])
		buf = append(buf, "\x1b\\"...)
	}
	return buf
}

// hasTermGraphics returns true when the $ENV{TERM_GRAPHICS} is the specified
// type.
func hasTermGraphics(typ string) bool {
	return typ == termGraphics()
}

// otherTermGraphics returns true when $ENV{TERM_GRAPHICS} names a protocol
// other than typ, or none. The encoder for typ is then not available,
// whatever else the environment says, so that the variable selects one
// protocol (D3).
func otherTermGraphics(typ string) bool {
	return otherProtocol(termGraphics(), typ)
}

// otherProtocol returns true when v, a value of $ENV{TERM_GRAPHICS}, names a
// protocol other than typ, or none. A value that names no protocol, such as
// the empty string, leaves each encoder to detect its protocol.
func otherProtocol(v, typ string) bool {
	switch v {
	case "kitty", "iterm", "sixel", "halfblock", "none":
		return v != typ
	}
	return false
}

// termGraphics returns $ENV{TERM_GRAPHICS} as [normalize] returns it. It
// reads the environment once.
var termGraphics = sync.OnceValue(func() string {
	return normalize(os.Getenv("TERM_GRAPHICS"))
})
