package rasterm

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"sync"
)

// ZlibMode is how the Kitty encoder compresses the raw pixels of an
// [RGBImage] or an [RGBAImage]. It never compresses a PNG, which is
// compressed already.
type ZlibMode uint8

// Zlib modes.
const (
	// ZlibAuto compresses raw pixels when they compress well. Pixels under
	// [zlibMin] bytes are sent raw. For more, the encoder starts one zlib
	// stream, and after each [zlibProbe] bytes of input, it compares all the
	// output so far with all the input so far. At the first check where the
	// output is not less than [zlibRatio] of the input, it drops the stream
	// and sends the pixels raw. The stream is never flushed, so a stream that
	// is sent is the same as one stream of all the pixels. Flat and rendered
	// images compress, and photos stay raw.
	ZlibAuto ZlibMode = iota
	// ZlibAlways compresses raw pixels of any size.
	ZlibAlways
	// ZlibNever sends raw pixels as they are.
	ZlibNever
)

// The limits of [ZlibAuto]. Ken chose them on 2026-10-15, from the
// measurements in iv (D18).
const (
	// zlibMin is the size of raw pixels below which they are not compressed.
	zlibMin = 64 << 10
	// zlibProbe is the number of bytes of input between two checks of the
	// ratio of the output to the input.
	zlibProbe = 64 << 10
	// zlibRatio is the ratio of the output to the input below which the
	// encoder keeps the stream.
	zlibRatio = 0.25
)

// zlibData returns data compressed with zlib into buf, and true, when mode
// asks for it and, for [ZlibAuto], when data compresses well. Otherwise it
// returns data and false.
func zlibData(buf *bytes.Buffer, data []byte, mode ZlibMode) ([]byte, bool, error) {
	switch {
	case mode == ZlibNever, mode == ZlibAuto && len(data) < zlibMin:
		return data, false, nil
	}
	zw := getZlibWriter(buf)
	defer zlibWriters.Put(zw)
	rest := data
	if mode == ZlibAuto {
		// check after each full probe. A shorter end is written with the
		// close, and is not checked
		for n := zlibProbe; n <= len(data); n += zlibProbe {
			if _, err := zw.Write(data[n-zlibProbe : n]); err != nil {
				return nil, false, err
			}
			// the writer is not flushed, because a flush adds stored blocks
			// that crash the inflater of Ghostty 1.3.1 (D18). So buf holds
			// the output up to the last block, and the ratio reads low by at
			// most one block, and a photo goes raw a check or two later
			if float64(buf.Len()) >= zlibRatio*float64(n) {
				buf.Reset()
				return data, false, nil
			}
			rest = data[n:]
		}
	}
	if _, err := zw.Write(rest); err != nil {
		return nil, false, err
	}
	if err := zw.Close(); err != nil {
		return nil, false, err
	}
	return buf.Bytes(), true, nil
}

// zlibWriters holds the zlib writers of [zlibData]. A writer keeps the large
// state of its compressor for the next stream.
var zlibWriters sync.Pool

// getZlibWriter returns a zlib writer at [flate.BestSpeed] that writes to
// buf.
func getZlibWriter(buf *bytes.Buffer) *zlib.Writer {
	if zw, ok := zlibWriters.Get().(*zlib.Writer); ok {
		zw.Reset(buf)
		return zw
	}
	// the level is valid, so there is no error
	zw, _ := zlib.NewWriterLevel(buf, flate.BestSpeed)
	return zw
}
