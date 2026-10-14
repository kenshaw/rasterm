// Package rasterm provides a simple way to encode images as terminal graphics,
// supporting Kitty, iTerm, and Sixel.
package rasterm

import (
	"image"
	"io"
	"regexp"
	"strings"
)

// TermType is a terminal graphics type.
type TermType uint8

// Terminal graphics types.
const (
	None TermType = iota
	Kitty
	ITerm
	Sixel
	Halfblock
	Default TermType = ^TermType(0)
)

// Available returns true when the terminal graphics type is available.
func (typ TermType) Available() bool {
	if r, ok := encoders[typ]; ok {
		return r.Available()
	}
	return false
}

// Encode encodes the image to w.
func (typ TermType) Encode(w io.Writer, img image.Image) error {
	if r, ok := encoders[typ]; ok {
		return r.Encode(w, img)
	}
	return ErrTermGraphicsNotAvailable
}

// EnvValue returns the environment value name for the type.
func (typ TermType) EnvValue() string {
	if typ == Default {
		return ""
	}
	return typ.String()
}

// String satisfies the [fmt.Stringer] interface.
func (typ TermType) String() string {
	switch typ {
	case Kitty:
		return "kitty"
	case ITerm:
		return "iterm"
	case Sixel:
		return "sixel"
	case Halfblock:
		return "halfblock"
	case Default:
		return "default"
	}
	return "none"
}

// MarshalText satisfies the [encoding.TextMarshaler] interface.
func (typ TermType) MarshalText() ([]byte, error) {
	switch typ {
	case None, Kitty, ITerm, Sixel, Halfblock:
		return []byte(typ.EnvValue()), nil
	case Default:
		return nil, nil
	}
	return nil, ErrUnknownTermType
}

// UnmarshalText satisfies the [encoding.TextUnmarshaler] interface.
//
// The text is read as [normalize] returns it, so "Half-Blocks" is
// [Halfblock].
func (typ *TermType) UnmarshalText(buf []byte) error {
	switch normalize(string(buf)) {
	default:
		*typ = None
	case "kitty":
		*typ = Kitty
	case "iterm":
		*typ = ITerm
	case "sixel":
		*typ = Sixel
	case "halfblock":
		*typ = Halfblock
	case "":
		*typ = Default
	}
	return nil
}

// normalize returns the name of a terminal graphics type as the code
// compares it: in lower case, with each character that is not a letter
// removed, and without a final s. So "half-block", "half-blocks",
// "halfblocks" and "HalfBlock" are each "halfblock", and "iTerm2" is "iterm".
func normalize(s string) string {
	return strings.TrimSuffix(nonLetters.ReplaceAllString(strings.ToLower(s), ""), "s")
}

// nonLetters matches each run of characters that are not a lower case letter.
var nonLetters = regexp.MustCompile(`[^a-z]+`)

// encoders are the registered encoders.
var encoders map[TermType]Encoder

func init() {
	kitty := NewKittyEncoder()
	iterm := NewITermEncoder()
	sixel := NewSixelEncoder()
	halfblock := NewHalfblockEncoder()
	encoders = map[TermType]Encoder{
		Kitty:     kitty,
		ITerm:     iterm,
		Sixel:     sixel,
		Halfblock: halfblock,
		// half blocks are last, and are available only when TERM_GRAPHICS
		// is halfblock, so they are never a fallback
		Default: NewDefaultEncoder(kitty, iterm, sixel, halfblock),
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
	// ErrUnknownTermType is the unknown term type error.
	ErrUnknownTermType Error = "unknown term type"
)
