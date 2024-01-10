// Package rasterm provides a simple way to encode images as terminal graphics,
// supporting Kitty, iTerm, and Sixel.
package rasterm

import (
	"fmt"
	"image"
	"io"
)

// TermType is a terminal graphics type.
type TermType uint8

// Terminal graphics types.
const (
	Default TermType = ^TermType(0)
	None    TermType = iota
	Kitty
	ITerm
	Sixel
)

// String satisfies the [fmt.Stringer] interface.
func (typ TermType) String() string {
	switch r, ok := encoders[typ]; {
	case ok:
		return r.String()
	case typ != None:
		return fmt.Sprintf("TermType(%d)", uint8(typ))
	}
	return ""
}

// Available returns true when the terminal graphics type is available.
func (typ TermType) Available() bool {
	if r, ok := encoders[typ]; ok {
		return r.Available()
	}
	return typ == None
}

// Encode encodes the image to w.
func (typ TermType) Encode(w io.Writer, img image.Image) error {
	switch r, ok := encoders[typ]; {
	case ok:
		return r.Encode(w, img)
	case typ != None:
		return ErrTermGraphicsNotAvailable
	}
	return nil
}

// encoders are the registered encoders.
var encoders map[TermType]Encoder

func init() {
	kitty := NewKittyEncoder()
	iterm := NewITermEncoder()
	sixel := NewSixelEncoder()
	encoders = map[TermType]Encoder{
		Kitty:   kitty,
		ITerm:   iterm,
		Sixel:   sixel,
		Default: NewDefaultEncoder(kitty, iterm, sixel),
	}
}

// Encode encodes the image to w using the [Default] encoder.
func Encode(w io.Writer, img image.Image) error {
	return Default.Encode(w, img)
}

// Available returns true the [Default] encoder is available.
func Available() bool {
	return Default.Available()
}

// Error is an error.
type Error string

// Error satisfies the [error] interface.
func (err Error) Error() string {
	return string(err)
}

const (
	// ErrTermGraphicsNotAvailable is the term graphics not available error.
	ErrTermGraphicsNotAvailable Error = "term graphics not available"
	// ErrNonTTY is the non tty error.
	ErrNonTTY Error = "non tty"
	// ErrTermResponseTimedOut is the term response timed out error.
	ErrTermResponseTimedOut Error = "term response timed out"
)
