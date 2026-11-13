package rasterm

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/jpeg"
	"io"
	"strconv"

	"github.com/mattn/go-sixel"
)

// DefaultJPEGQuality is the default JPEG encode quality.
var DefaultJPEGQuality = 93

// Encoder encodes an image for a terminal.
type Encoder interface {
	Encode(w io.Writer, img image.Image) error
}

// KittyEncoder encodes an image with the Kitty graphics protocol. It
// satisfies the [Encoder] interface. [Terminal.Encoder] sets it up for the
// terminal.
//
// See: https://sw.kovidgoyal.net/kitty/graphics-protocol/
type KittyEncoder struct {
	// NoNewline writes no newline after the image.
	NoNewline bool
	// Zlib is how raw pixels are compressed. The zero value is [ZlibAuto].
	Zlib ZlibMode
	// Tmux wraps each escape sequence in the DCS passthrough of tmux (D20).
	Tmux bool
	// Cell is the size of a cell in pixels. In tmux, when it is known, the
	// image is drawn with Unicode placeholder cells (D22).
	Cell image.Point
}

// Encode satisfies the [Encoder] interface.
//
// If img is a [PNGImage], an [RGBImage] or an [RGBAImage], its bytes are
// sent as they are, in that order of preference. Otherwise the image is sent
// as a PNG. Raw pixels are compressed with zlib as [KittyEncoder.Zlib] says.
// The output is built in memory and written to w in one call (D9).
func (r KittyEncoder) Encode(w io.Writer, img image.Image) error {
	return r.encode(w, img, kittyImage{action: 'T'}, r.Tmux && r.Cell.X > 0 && r.Cell.Y > 0)
}

// encode writes img to w with the keys of k. When virtual is true, the image
// gets an id and a virtual placement, and is drawn with placeholder cells. k
// with an id keeps it.
func (r KittyEncoder) encode(w io.Writer, img image.Image, k kittyImage, virtual bool) error {
	src := getBuffer()
	defer putBuffer(src)
	format, data, err := kittyData(src, img)
	if err != nil {
		return err
	}
	if format != kittyPNG {
		// src holds no png here, so the zlib stream can use it
		if data, k.z, err = zlibData(src, data, r.Zlib); err != nil {
			return err
		}
	}
	out := getBuffer()
	defer putBuffer(out)
	b := img.Bounds()
	k.format, k.w, k.h, k.tmux = format, b.Dx(), b.Dy(), r.Tmux
	if virtual {
		// tmux does not know about an image, and draws over it, so the
		// image is drawn with placeholder cells, which tmux keeps as text
		if cols, rows, ok := kittyCells(k.w, k.h, r.Cell.X, r.Cell.Y); ok {
			if k.id == 0 {
				k.id = newKittyID()
			}
			k.virtual, k.cols, k.rows = true, cols, rows
		}
	}
	out.Grow(kittyLen(len(data), k.tmux) + kittyPlaceholderLen(k.cols, k.rows) + len(k.after) + 1)
	buf := appendKitty(out.AvailableBuffer(), k, data)
	buf = append(buf, k.after...)
	if k.virtual {
		buf = appendKittyPlaceholder(buf, k.id, k.cols, k.rows)
	}
	if !r.NoNewline && k.action == 'T' {
		buf = append(buf, '\n')
	}
	_, err = w.Write(buf)
	return err
}

// ITermEncoder encodes an image with the inline images protocol of iTerm2.
// It satisfies the [Encoder] interface.
//
// See: https://iterm2.com/documentation-images.html
type ITermEncoder struct {
	// NoNewline writes no newline after the image.
	NoNewline bool
	// Tmux wraps the escape sequence in the DCS passthrough of tmux (D20).
	Tmux bool
}

// Encode satisfies the [Encoder] interface.
//
// If img is a [PNGImage] or a [JPEGImage], its bytes are sent as they are.
// Otherwise a paletted image is sent as a PNG, and any other image as a JPEG
// (D5). The output is built in memory and written to w in one call (D9).
func (r ITermEncoder) Encode(w io.Writer, img image.Image) error {
	src := getBuffer()
	defer putBuffer(src)
	data, err := itermData(src, img)
	if err != nil {
		return err
	}
	out := getBuffer()
	defer putBuffer(out)
	start, end := itermSeq(r.Tmux)
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

// SixelEncoder encodes an image with Sixel. It satisfies the [Encoder]
// interface.
//
// See: https://saitoha.github.io/libsixel/
type SixelEncoder struct {
	// NoNewline writes no newline after the image.
	NoNewline bool
}

// Encode satisfies the [Encoder] interface.
//
// The output and the newline are built in memory and written to w in one
// call (D9). go-sixel before v0.0.14 made a call to Write for each few bytes
// when w was not an [os.File].
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

// kittyStartMax is the length of the longest start of a Kitty image or
// frame, with each number at its longest, in tmux.
const kittyStartMax = len(apcTmuxStart+"a=T,f=100,s=,v=,o=z,U=1,q=2,i=,c=,r=,x=,y=,X=1,Y=,z=,m=1;"+apcTmuxEnd) + 11*10

// kittyChunk is the number of bytes of PNG in each chunk. Its base64
// encoding is 4096 bytes, which is the largest chunk that the Kitty protocol
// allows.
const kittyChunk = 3072

// kittyLen returns the largest length of the output of [appendKitty] for n
// bytes of data, in tmux when tmux is true.
func kittyLen(n int, tmux bool) int {
	start, end := kittySeq(tmux)
	chunks := (n + kittyChunk - 1) / kittyChunk
	return kittyStartMax + chunks*len(start+"a=f,q=2,m=0;"+end) + base64.StdEncoding.EncodedLen(n)
}

// kittyImage holds the keys of the first escape sequence of a Kitty image or
// of a frame of an animation.
type kittyImage struct {
	// action is 'T' to send and show an image, or 'f' to add a frame. 0 is
	// 'T'.
	action byte
	// format is the format of the data, and w and h are the width and the
	// height of raw pixels.
	format, w, h int
	// z is true when the data is one zlib stream.
	z bool
	// tmux is true when each sequence goes in the DCS passthrough of tmux.
	tmux bool
	// id is the image id, or 0 for an image with no id. Each sequence of an
	// image with an id has q=2, so that the terminal writes no response
	// that tmux or the program reads as input.
	id uint32
	// virtual is true for a virtual placement of cols x rows cells, which
	// Unicode placeholder cells draw.
	virtual    bool
	cols, rows int
	// x and y are where a frame goes in the root frame. base is the frame
	// that gives the other pixels, or 0 for none. bg is the background
	// color, as 0xRRGGBBAA, when hasBg is true. replace replaces the pixels
	// under the frame. gap is the gap of the frame in ms.
	x, y    int
	base    int
	bg      uint32
	hasBg   bool
	replace bool
	gap     int
	// after is written after the image, before the placeholder cells.
	after []byte
}

// appendKitty appends the Kitty escape sequences that send data to buf. The
// first sequence holds the keys of k. Each chunk after it holds the base64
// encoding of [kittyChunk] bytes of data. In tmux, each sequence is wrapped
// in the DCS passthrough of tmux on its own, so that each is far below the
// input limit of tmux. Each chunk of a frame repeats a=f, as the protocol
// asks.
func appendKitty(buf []byte, k kittyImage, data []byte) []byte {
	if k.action == 0 {
		k.action = 'T'
	}
	start, end := kittySeq(k.tmux)
	buf = append(buf, start...)
	buf = append(buf, "a="...)
	buf = append(buf, k.action)
	buf = append(buf, ",f="...)
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
	switch {
	case k.virtual:
		buf = append(buf, ",U=1,q=2,i="...)
		buf = strconv.AppendUint(buf, uint64(k.id), 10)
		buf = append(buf, ",c="...)
		buf = strconv.AppendInt(buf, int64(k.cols), 10)
		buf = append(buf, ",r="...)
		buf = strconv.AppendInt(buf, int64(k.rows), 10)
	case k.id != 0:
		buf = append(buf, ",q=2,i="...)
		buf = strconv.AppendUint(buf, uint64(k.id), 10)
	}
	if k.action == 'f' {
		buf = appendKittyFrame(buf, k)
	}
	buf = append(buf, ",m=1;"...)
	buf = append(buf, end...)
	for i := 0; i < len(data); i += kittyChunk {
		j, m := min(i+kittyChunk, len(data)), byte('1')
		if j == len(data) {
			m = '0'
		}
		buf = append(buf, start...)
		if k.action == 'f' {
			buf = append(buf, "a=f,"...)
		}
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

// appendKittyFrame appends the keys of a frame in k to buf.
func appendKittyFrame(buf []byte, k kittyImage) []byte {
	if k.x != 0 {
		buf = append(buf, ",x="...)
		buf = strconv.AppendInt(buf, int64(k.x), 10)
	}
	if k.y != 0 {
		buf = append(buf, ",y="...)
		buf = strconv.AppendInt(buf, int64(k.y), 10)
	}
	if k.base != 0 {
		buf = append(buf, ",c="...)
		buf = strconv.AppendInt(buf, int64(k.base), 10)
	}
	if k.hasBg {
		buf = append(buf, ",Y="...)
		buf = strconv.AppendUint(buf, uint64(k.bg), 10)
	}
	if k.replace {
		buf = append(buf, ",X=1"...)
	}
	buf = append(buf, ",z="...)
	return strconv.AppendInt(buf, int64(k.gap), 10)
}
