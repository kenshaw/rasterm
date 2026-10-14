package rasterm

import (
	"image"
)

// RGBImage is the interface for an image that holds its own pixels as packed
// 8-bit red, green and blue, 3 bytes for each pixel, row by row from the top
// left, with no padding at the end of a row.
//
// The Kitty encoder sends the bytes that RGB returns, and does not encode the
// image. It checks that their length is 3 times the number of pixels in the
// bounds of the image. If the check fails, it encodes the image. iTerm, Sixel
// and half blocks cannot send raw pixels, and encode the image.
type RGBImage interface {
	image.Image
	RGB() []byte
}

// RGBAImage is the interface for an image that holds its own pixels as packed
// 8-bit red, green, blue and alpha, 4 bytes for each pixel, row by row from
// the top left, with no padding at the end of a row. The alpha is not
// premultiplied, as in [image.NRGBA].
//
// The Kitty encoder sends the bytes that RGBA returns, and does not encode the
// image. It checks that their length is 4 times the number of pixels in the
// bounds of the image. If the check fails, it encodes the image. iTerm, Sixel
// and half blocks cannot send raw pixels, and encode the image.
type RGBAImage interface {
	image.Image
	RGBA() []byte
}

// RGB is an image and its pixels as packed red, green and blue. It satisfies
// the [RGBImage] interface.
//
// Wrap an image only while it is not changed. A scaled or cropped image needs
// new pixels, or no wrapper at all.
type RGB struct {
	image.Image

	// Data is the pixels of Image, as [RGBImage] describes them.
	Data []byte
}

// RGB satisfies the [RGBImage] interface.
func (r RGB) RGB() []byte {
	return r.Data
}

// Unwrap returns the image that r wraps.
func (r RGB) Unwrap() image.Image {
	return r.Image
}

// RGBA is an image and its pixels as packed red, green, blue and alpha. It
// satisfies the [RGBAImage] interface.
//
// Wrap an image only while it is not changed. A scaled or cropped image needs
// new pixels, or no wrapper at all.
type RGBA struct {
	image.Image

	// Data is the pixels of Image, as [RGBAImage] describes them.
	Data []byte
}

// RGBA satisfies the [RGBAImage] interface.
func (r RGBA) RGBA() []byte {
	return r.Data
}

// Unwrap returns the image that r wraps.
func (r RGBA) Unwrap() image.Image {
	return r.Image
}

// rgbOf returns the bytes of img when it is an [RGBImage] whose bytes have the
// length of its bounds.
func rgbOf(img image.Image) ([]byte, bool) {
	r, ok := img.(RGBImage)
	if !ok {
		return nil, false
	}
	data := r.RGB()
	return data, rawMatches(data, img.Bounds(), 3)
}

// rgbaOf returns the bytes of img when it is an [RGBAImage] whose bytes have
// the length of its bounds.
func rgbaOf(img image.Image) ([]byte, bool) {
	r, ok := img.(RGBAImage)
	if !ok {
		return nil, false
	}
	data := r.RGBA()
	return data, rawMatches(data, img.Bounds(), 4)
}

// rawMatches returns true when bounds is not empty and data holds n bytes for
// each of its pixels.
func rawMatches(data []byte, bounds image.Rectangle, n int) bool {
	return !bounds.Empty() && len(data) == bounds.Dx()*bounds.Dy()*n
}
