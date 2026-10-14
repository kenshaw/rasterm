package rasterm

import (
	"image"
	"image/color"
	"image/draw"
	"io"
	"strconv"
)

// HalfblockEncoder is a terminal graphics encoder that draws an image as text,
// with the Unicode half block characters and 24-bit color.
//
// Each cell shows two pixels, one above the other. The character `▀` shows
// the upper pixel in the foreground color and the lower pixel in the
// background color. A pixel whose alpha is less than half is transparent, and
// shows the background of the terminal.
//
// The encoder does not scale. Each column of pixels is one column of cells,
// and each two rows of pixels are one row of cells, so the caller scales the
// image to the size of the terminal first. The encoder is available only when
// TERM_GRAPHICS is halfblock. [Default] tries it last, after Kitty, iTerm and
// Sixel, so it is never a fallback.
type HalfblockEncoder struct {
	NoNewline bool
}

// NewHalfblockEncoder creates a half block terminal graphics encoder.
func NewHalfblockEncoder() Encoder {
	return HalfblockEncoder{}
}

// Available satisfies the [Encoder] interface. It is true only when
// TERM_GRAPHICS is halfblock, because the encoder needs nothing from the
// terminal except 24-bit color, which no variable reliably reports.
func (HalfblockEncoder) Available() bool {
	return hasTermGraphics("halfblock")
}

// Encode satisfies the [Encoder] interface.
//
// The output is built in memory and written to w in one call. A color
// sequence is written only when the color changes from the cell before it,
// and each row ends with a reset of the colors.
func (r HalfblockEncoder) Encode(w io.Writer, img image.Image) error {
	out := getBuffer()
	defer putBuffer(out)
	buf := appendHalfblock(out.AvailableBuffer(), halfblockSource(unwrap(img)))
	if !r.NoNewline {
		buf = append(buf, '\n')
	}
	_, err := w.Write(buf)
	return err
}

// halfblockSource returns img as an *image.NRGBA whose bounds start at the
// origin. [draw.Draw] has a fast path to *image.NRGBA from each image type in
// the standard library.
func halfblockSource(img image.Image) *image.NRGBA {
	b := img.Bounds()
	if m, ok := img.(*image.NRGBA); ok && b.Min == (image.Point{}) {
		return m
	}
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

// The half block characters and the color sequences of the half block
// encoder.
const (
	upperHalf = "▀"
	lowerHalf = "▄"
	fgColor   = "\x1b[38;2;"
	bgColor   = "\x1b[48;2;"
	bgDefault = "\x1b[49m"
	resetSGR  = "\x1b[0m"
)

// halfblockState is the foreground and background color that the terminal
// uses for the next cell. A color with ok false is the default color of the
// terminal.
type halfblockState struct {
	fg, bg halfblockColor
}

// halfblockColor is an opaque color, or the default color when ok is false.
type halfblockColor struct {
	c  color.NRGBA
	ok bool
}

// appendHalfblock appends the half block cells of img to buf. Each row of
// cells is two rows of pixels. An odd last row of pixels has a transparent row
// under it.
func appendHalfblock(buf []byte, img *image.NRGBA) []byte {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	for y := 0; y < h; y += 2 {
		if y != 0 {
			buf = append(buf, '\n')
		}
		var st halfblockState
		for x := range w {
			top, bottom := halfblockPixel(img, x, y), halfblockPixel(img, x, y+1)
			switch {
			case top.ok:
				buf = st.setFg(buf, top)
				buf = st.setBg(buf, bottom)
				buf = append(buf, upperHalf...)
			case bottom.ok:
				buf = st.setFg(buf, bottom)
				buf = st.setBg(buf, top)
				buf = append(buf, lowerHalf...)
			default:
				buf = st.setBg(buf, top)
				buf = append(buf, ' ')
			}
		}
		buf = append(buf, resetSGR...)
	}
	return buf
}

// halfblockPixel returns the color of the pixel at x, y. It returns the
// default color when the pixel is outside img or its alpha is less than half.
func halfblockPixel(img *image.NRGBA, x, y int) halfblockColor {
	if y >= img.Rect.Dy() {
		return halfblockColor{}
	}
	i := y*img.Stride + x*4
	s := img.Pix[i : i+4 : i+4]
	if s[3] < 0x80 {
		return halfblockColor{}
	}
	return halfblockColor{c: color.NRGBA{R: s[0], G: s[1], B: s[2], A: 0xff}, ok: true}
}

// setFg appends the sequence that sets the foreground color to c, unless it
// is set already. c is never the default color.
func (st *halfblockState) setFg(buf []byte, c halfblockColor) []byte {
	if st.fg == c {
		return buf
	}
	st.fg = c
	return appendRGB(append(buf, fgColor...), c.c)
}

// setBg appends the sequence that sets the background color to c, unless it
// is set already. The state starts at the default background, because each
// row starts after a reset.
func (st *halfblockState) setBg(buf []byte, c halfblockColor) []byte {
	switch {
	case st.bg == c:
		return buf
	case !c.ok:
		st.bg = c
		return append(buf, bgDefault...)
	}
	st.bg = c
	return appendRGB(append(buf, bgColor...), c.c)
}

// appendRGB appends the red, green and blue of c, separated with a
// semicolon, and the final m of a color sequence.
func appendRGB(buf []byte, c color.NRGBA) []byte {
	buf = strconv.AppendUint(buf, uint64(c.R), 10)
	buf = append(buf, ';')
	buf = strconv.AppendUint(buf, uint64(c.G), 10)
	buf = append(buf, ';')
	buf = strconv.AppendUint(buf, uint64(c.B), 10)
	return append(buf, 'm')
}
