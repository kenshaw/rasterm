package rasterm

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/jpeg"
)

// JPEGImage is the interface for an image that holds its own encoding as a
// JPEG.
//
// The iTerm encoder sends the bytes that JPEG returns, and does not encode the
// image again. It checks that the bytes start with the JPEG start of image
// marker, and that the width and height in the frame header match the bounds
// of the image. If either check fails, it encodes the image. Kitty and Sixel
// cannot send a JPEG, and encode the image.
type JPEGImage interface {
	image.Image
	JPEG() []byte
}

// JPEG is an image and its encoding as a JPEG. It satisfies the [JPEGImage]
// interface.
//
// Wrap an image only while it is not changed. A scaled or cropped image needs
// a new JPEG, or no wrapper at all.
type JPEG struct {
	image.Image

	// Data is the encoding of Image as a JPEG.
	Data []byte
}

// DecodeJPEG decodes buf as a JPEG, and keeps buf as the encoding of the
// image.
func DecodeJPEG(buf []byte) (JPEG, error) {
	img, err := jpeg.Decode(bytes.NewReader(buf))
	if err != nil {
		return JPEG{}, err
	}
	return JPEG{
		Image: img,
		Data:  buf,
	}, nil
}

// JPEG satisfies the [JPEGImage] interface.
func (j JPEG) JPEG() []byte {
	return j.Data
}

// Unwrap returns the image that j wraps.
func (j JPEG) Unwrap() image.Image {
	return j.Image
}

// jpegOf returns the bytes of img when it is a [JPEGImage] whose bytes pass
// the checks of [jpegMatches].
func jpegOf(img image.Image) ([]byte, bool) {
	j, ok := img.(JPEGImage)
	if !ok {
		return nil, false
	}
	data := j.JPEG()
	return data, jpegMatches(data, img.Bounds())
}

// jpegMatches returns true when data starts with the JPEG start of image
// marker, and its first frame header holds the width and height of bounds.
// It reads the markers up to the frame header, and skips the data of each.
func jpegMatches(data []byte, bounds image.Rectangle) bool {
	if len(data) < 2 || data[0] != 0xff || data[1] != 0xd8 {
		return false
	}
	for i := 2; i+2 <= len(data); {
		if data[i] != 0xff {
			return false
		}
		m := data[i+1]
		switch {
		case m == 0xff: // fill byte
			i++
			continue
		case m == 0x01, 0xd0 <= m && m <= 0xd8: // a marker with no length
			i += 2
			continue
		case m == 0xd9, m == 0xda: // end of image or start of scan, before a frame header
			return false
		}
		i += 2
		if i+2 > len(data) {
			return false
		}
		n := int(binary.BigEndian.Uint16(data[i:]))
		if n < 2 || i+n > len(data) {
			return false
		}
		if jpegFrame(m) {
			// length (2), precision (1), height (2), width (2)
			if n < 7 {
				return false
			}
			h, w := binary.BigEndian.Uint16(data[i+3:]), binary.BigEndian.Uint16(data[i+5:])
			return int(w) == bounds.Dx() && int(h) == bounds.Dy()
		}
		i += n
	}
	return false
}

// jpegFrame returns true when m is a start of frame marker. The markers from
// 0xc0 to 0xcf start a frame, except 0xc4, 0xc8 and 0xcc.
func jpegFrame(m byte) bool {
	return 0xc0 <= m && m <= 0xcf && m != 0xc4 && m != 0xc8 && m != 0xcc
}
