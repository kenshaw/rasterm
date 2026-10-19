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

	// Zlib is how raw pixels are compressed. The zero value is [ZlibAuto].
	Zlib ZlibMode
}

// NewKittyEncoder creates a Kitty terminal graphics encoder.
//
// See: https://sw.kovidgoyal.net/kitty/graphics-protocol.html
func NewKittyEncoder() Encoder {
	return KittyEncoder{}
}

// Available satisfies the [Encoder] interface.
//
// In tmux, the variables of the environment can belong to another terminal,
// so the encoder asks tmux for the terminal of its client (D21).
func (KittyEncoder) Available() bool {
	switch {
	case otherTermGraphics("kitty"):
		return false
	case hasTermGraphics("kitty"):
		return true
	case inTmux():
		return tmuxClientOnce().protocol == "kitty"
	}
	return os.Getenv("KITTY_PID") != "" && os.Getenv("KITTY_WINDOW_ID") != "" ||
		strings.ToLower(os.Getenv("TERM")) == "xterm-kitty" ||
		strings.ToLower(os.Getenv("TERM_PROGRAM")) == "ghostty"
}

// Encode satisfies the [Encoder] interface.
//
// If img is a [PNGImage], an [RGBImage] or an [RGBAImage], its bytes are
// sent as they are, in that order of preference. Otherwise the image is sent
// as a PNG. Raw pixels are compressed with zlib as [KittyEncoder.Zlib] says.
// In tmux, each escape sequence is wrapped in the DCS passthrough of tmux
// (D20). The output is built in memory and written to w in one call.
func (r KittyEncoder) Encode(w io.Writer, img image.Image) error {
	src := getBuffer()
	defer putBuffer(src)
	format, data, err := kittyData(src, img)
	if err != nil {
		return err
	}
	var z bool
	if format != kittyPNG {
		// src holds no png here, so the zlib stream can use it
		if data, z, err = zlibData(src, data, r.Zlib); err != nil {
			return err
		}
	}
	out := getBuffer()
	defer putBuffer(out)
	b := img.Bounds()
	k := kittyImage{format: format, w: b.Dx(), h: b.Dy(), z: z, tmux: inTmux()}
	if k.tmux {
		// tmux does not know about an image, and draws over it, so the
		// image is drawn with placeholder cells, which tmux keeps as text
		c := tmuxClientOnce()
		if cols, rows, ok := kittyCells(k.w, k.h, c.cellWidth, c.cellHeight); ok {
			k.id, k.cols, k.rows = newKittyID(), cols, rows
		}
	}
	out.Grow(kittyLen(len(data), k.tmux) + kittyPlaceholderLen(k.cols, k.rows) + 1)
	buf := appendKitty(out.AvailableBuffer(), k, data)
	if k.id != 0 {
		buf = appendKittyPlaceholder(buf, k.id, k.cols, k.rows)
	}
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
//
// In tmux, the variables of the environment can belong to another terminal,
// so the encoder asks tmux for the terminal of its client (D21).
func (ITermEncoder) Available() bool {
	switch {
	case otherTermGraphics("iterm"):
		return false
	case hasTermGraphics("iterm"):
		return true
	case inTmux():
		return tmuxClientOnce().protocol == "iterm"
	}
	return strings.ToLower(os.Getenv("TERM")) == "mintty" ||
		strings.ToLower(os.Getenv("LC_TERMINAL")) == "iterm2" ||
		strings.ToLower(os.Getenv("TERM_PROGRAM")) == "wezterm"
}

// Encode satisfies the [Encoder] interface.
//
// If img is a [PNGImage] or a [JPEGImage], its bytes are sent as they are.
// Otherwise a paletted image is sent as a PNG, and any other image as a JPEG.
// In tmux, the escape sequence is wrapped in the DCS passthrough of tmux
// (D20). The output is built in memory and written to w in one call.
func (r ITermEncoder) Encode(w io.Writer, img image.Image) error {
	src := getBuffer()
	defer putBuffer(src)
	data, err := itermData(src, img)
	if err != nil {
		return err
	}
	out := getBuffer()
	defer putBuffer(out)
	start, end := itermSeq(inTmux())
	out.Grow(len(start) + base64.StdEncoding.EncodedLen(len(data)) + len(end) + 1)
	buf := append(out.AvailableBuffer(), start...)
	buf = base64.StdEncoding.AppendEncode(buf, data)
	buf = append(buf, end...)
	if !r.NoNewline {
		buf = append(buf, '\n')
	}
	_, err = w.Write(buf)
	return err
}

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
// width and a height of 10 digits each, in tmux.
const kittyStartMax = len(apcTmuxStart+"a=T,f=100,s=,v=,o=z,m=1;"+apcTmuxEnd) + 20

// kittyChunk is the number of bytes of PNG in each chunk. Its base64
// encoding is 4096 bytes, which is the largest chunk that the Kitty protocol
// allows.
const kittyChunk = 3072

// kittyLen returns the largest length of the output of [appendKitty] for n
// bytes of data, in tmux when tmux is true.
func kittyLen(n int, tmux bool) int {
	start, end := kittySeq(tmux)
	chunks := (n + kittyChunk - 1) / kittyChunk
	return kittyStartMax + chunks*len(start+"m=0;"+end) + base64.StdEncoding.EncodedLen(n)
}

// kittyImage holds the keys of the first escape sequence of a Kitty image.
type kittyImage struct {
	// format is the format of the data, and w and h are the width and the
	// height of raw pixels.
	format, w, h int
	// z is true when the data is one zlib stream.
	z bool
	// tmux is true when each sequence goes in the DCS passthrough of tmux.
	tmux bool
	// id, cols and rows are set for a virtual placement, which Unicode
	// placeholder cells draw. id is 0 for an image that is drawn at once.
	id         uint32
	cols, rows int
}

// appendKitty appends the Kitty escape sequences that send data to buf. The
// first sequence holds the keys of k. Each chunk after it holds the base64
// encoding of [kittyChunk] bytes of data. In tmux, each sequence is wrapped
// in the DCS passthrough of tmux on its own, so that each is far below the
// input limit of tmux. For a virtual placement, each sequence has q=2, so
// that the terminal writes no response for tmux to read as input.
func appendKitty(buf []byte, k kittyImage, data []byte) []byte {
	start, end := kittySeq(k.tmux)
	buf = append(buf, start...)
	buf = append(buf, "a=T,f="...)
	buf = strconv.AppendInt(buf, int64(k.format), 10)
	if k.format != kittyPNG {
		buf = append(buf, ",s="...)
		buf = strconv.AppendInt(buf, int64(k.w), 10)
		buf = append(buf, ",v="...)
		buf = strconv.AppendInt(buf, int64(k.h), 10)
	}
	if k.z {
		buf = append(buf, ",o=z"...)
	}
	if k.id != 0 {
		buf = append(buf, ",U=1,q=2,i="...)
		buf = strconv.AppendUint(buf, uint64(k.id), 10)
		buf = append(buf, ",c="...)
		buf = strconv.AppendInt(buf, int64(k.cols), 10)
		buf = append(buf, ",r="...)
		buf = strconv.AppendInt(buf, int64(k.rows), 10)
	}
	buf = append(buf, ",m=1;"...)
	buf = append(buf, end...)
	for i := 0; i < len(data); i += kittyChunk {
		j, m := min(i+kittyChunk, len(data)), byte('1')
		if j == len(data) {
			m = '0'
		}
		buf = append(buf, start...)
		if k.id != 0 {
			buf = append(buf, "q=2,"...)
		}
		buf = append(buf, "m="...)
		buf = append(buf, m, ';')
		buf = base64.StdEncoding.AppendEncode(buf, data[i:j])
		buf = append(buf, end...)
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
