package rasterm

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/draw"
	"image/png"
	"io"
	"sync"
)

// DefaultPNGCompression is the default PNG compression level.
//
// [png.BestSpeed] encodes in less time than [png.DefaultCompression], and
// gives a larger PNG. BenchmarkPNGCompression measures both.
var DefaultPNGCompression = png.BestSpeed

// PNGImage is the interface for an image that holds its own encoding as a
// PNG.
//
// The Kitty and iTerm encoders send the bytes that PNG returns, and do not
// encode the image again. They check that the bytes start with the PNG
// signature and that the width and height in the header match the bounds of
// the image. If either check fails, they encode the image. Sixel cannot send
// a PNG, and encodes the image.
type PNGImage interface {
	image.Image
	PNG() []byte
}

// PNG is an image and its encoding as a PNG. It satisfies the [PNGImage]
// interface.
//
// Wrap an image only while it is not changed. A scaled or cropped image needs
// a new PNG, or no wrapper at all.
type PNG struct {
	image.Image

	// Data is the encoding of Image as a PNG.
	Data []byte
}

// DecodePNG decodes buf as a PNG, and keeps buf as the encoding of the image.
func DecodePNG(buf []byte) (PNG, error) {
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		return PNG{}, err
	}
	return PNG{
		Image: img,
		Data:  buf,
	}, nil
}

// PNG satisfies the [PNGImage] interface.
func (p PNG) PNG() []byte {
	return p.Data
}

// Unwrap returns the image that p wraps.
func (p PNG) Unwrap() image.Image {
	return p.Image
}

// unwrap returns the image under each Unwrap method of img, so that a type
// switch on the result finds the concrete type.
func unwrap(img image.Image) image.Image {
	for {
		u, ok := img.(interface{ Unwrap() image.Image })
		if !ok {
			return img
		}
		img = u.Unwrap()
	}
}

// pngSignature is the first eight bytes of every PNG.
const pngSignature = "\x89PNG\r\n\x1a\n"

// pngData returns the encoding of img as a PNG. It returns the bytes from
// [pngOf] when there are any, and otherwise encodes img into buf and returns
// the bytes of buf.
func pngData(buf *bytes.Buffer, img image.Image) ([]byte, error) {
	if data, ok := pngOf(img); ok {
		return data, nil
	}
	if err := encodePNG(buf, unwrap(img)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// pngOf returns the bytes of img when it is a [PNGImage] whose bytes pass the
// checks of [pngMatches].
func pngOf(img image.Image) ([]byte, bool) {
	p, ok := img.(PNGImage)
	if !ok {
		return nil, false
	}
	data := p.PNG()
	return data, pngMatches(data, img.Bounds())
}

// pngMatches returns true when data starts with the PNG signature and an IHDR
// chunk that holds the width and height of bounds. It reads only the first 24
// bytes.
func pngMatches(data []byte, bounds image.Rectangle) bool {
	// signature (8), chunk length (4), chunk type (4), width (4), height (4)
	if len(data) < 24 || string(data[:8]) != pngSignature || string(data[12:16]) != "IHDR" {
		return false
	}
	w, h := binary.BigEndian.Uint32(data[16:20]), binary.BigEndian.Uint32(data[20:24])
	return int64(w) == int64(bounds.Dx()) && int64(h) == int64(bounds.Dy())
}

// encodePNG encodes img to w as a PNG with [DefaultPNGCompression].
func encodePNG(w io.Writer, img image.Image) error {
	enc := png.Encoder{
		CompressionLevel: DefaultPNGCompression,
		BufferPool:       pngBuffers{},
	}
	return enc.Encode(w, pngFast(img))
}

// pngFast returns img as a type that [png.Encoder] encodes without a call to
// At for each pixel. The encoder has a fast path for *image.Gray,
// *image.RGBA, *image.Paletted and *image.NRGBA only. A JPEG decodes to
// *image.YCbCr, or to *image.CMYK, and [draw.Draw] has a fast path from each
// of these to *image.RGBA. Both are opaque, so no alpha is lost. Any other
// type is returned as is.
func pngFast(img image.Image) image.Image {
	switch img.(type) {
	case *image.YCbCr, *image.CMYK:
		b := img.Bounds()
		dst := image.NewRGBA(b)
		draw.Draw(dst, b, img, b.Min, draw.Src)
		return dst
	}
	return img
}

// pngBuffers is the buffer pool that each [png.Encoder] shares. It keeps the
// zlib writer and the row buffers of an encoder for the next encode.
type pngBuffers struct{}

// Get satisfies the [png.EncoderBufferPool] interface.
func (pngBuffers) Get() *png.EncoderBuffer {
	b, _ := pngBufferPool.Get().(*png.EncoderBuffer)
	return b
}

// Put satisfies the [png.EncoderBufferPool] interface.
func (pngBuffers) Put(b *png.EncoderBuffer) {
	pngBufferPool.Put(b)
}

var pngBufferPool sync.Pool

// maxPooledBuffer is the capacity above which a buffer is not returned to
// [bufferPool], so that one large image does not hold its memory after the
// encode.
const maxPooledBuffer = 64 << 20

// bufferPool holds the buffers that the encoders build their output in.
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// getBuffer returns an empty buffer from [bufferPool].
func getBuffer() *bytes.Buffer {
	buf, _ := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	return buf
}

// putBuffer returns buf to [bufferPool], unless its capacity is over
// [maxPooledBuffer].
func putBuffer(buf *bytes.Buffer) {
	if buf.Cap() <= maxPooledBuffer {
		bufferPool.Put(buf)
	}
}
