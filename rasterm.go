// Package rasterm draws images in a terminal, with the Kitty graphics
// protocol, the inline images of iTerm2, Sixel, or half blocks of text.
//
// [Detect] asks the terminal which protocols it supports. [Terminal.Encoder]
// returns the encoder for the protocol that Detect selected.
package rasterm

import (
	"image"
	"regexp"
	"slices"
	"strings"
)

// Protocol is a terminal graphics protocol.
type Protocol uint8

// Terminal graphics protocols.
const (
	// None is no graphics.
	None Protocol = iota
	// Kitty is the Kitty graphics protocol.
	Kitty
	// ITerm is the inline images protocol of iTerm2.
	ITerm
	// Sixel is the Sixel protocol.
	Sixel
	// Halfblock draws an image as text, with half blocks and 24-bit color. It
	// is not a protocol of the terminal, so [Detect] selects it only when
	// TERM_GRAPHICS asks for it (D12).
	Halfblock
)

// ParseProtocol returns the protocol that s names, as [Detect] reads
// TERM_GRAPHICS. Case, the characters that are not letters, and a final s do
// not matter, so "Half-Blocks" is [Halfblock] and "iTerm2" is [ITerm]. ok is
// false when s names no protocol, as the empty string does.
func ParseProtocol(s string) (p Protocol, ok bool) {
	switch normalize(s) {
	case "none":
		return None, true
	case "kitty":
		return Kitty, true
	case "iterm":
		return ITerm, true
	case "sixel":
		return Sixel, true
	case "halfblock":
		return Halfblock, true
	}
	return None, false
}

// String satisfies the [fmt.Stringer] interface.
func (p Protocol) String() string {
	switch p {
	case None:
		return "none"
	case Kitty:
		return "kitty"
	case ITerm:
		return "iterm"
	case Sixel:
		return "sixel"
	case Halfblock:
		return "halfblock"
	}
	return "unknown"
}

// MarshalText satisfies the [encoding.TextMarshaler] interface.
func (p Protocol) MarshalText() ([]byte, error) {
	if p > Halfblock {
		return nil, ErrUnknownProtocol
	}
	return []byte(p.String()), nil
}

// UnmarshalText satisfies the [encoding.TextUnmarshaler] interface. It reads
// the text as [ParseProtocol] does. Empty text is [None].
func (p *Protocol) UnmarshalText(buf []byte) error {
	if len(buf) == 0 {
		*p = None
		return nil
	}
	v, ok := ParseProtocol(string(buf))
	if !ok {
		return ErrUnknownProtocol
	}
	*p = v
	return nil
}

// normalize returns the name of a protocol as the code compares it: in lower
// case, with each character that is not a letter removed, and without a
// final s. So "half-block", "half-blocks", "halfblocks" and "HalfBlock" are
// each "halfblock", and "iTerm2" is "iterm".
func normalize(s string) string {
	return strings.TrimSuffix(nonLetters.ReplaceAllString(strings.ToLower(s), ""), "s")
}

// nonLetters matches each run of characters that are not a lower case letter.
var nonLetters = regexp.MustCompile(`[^a-z]+`)

// Source is where the protocol of a [Terminal] comes from.
type Source uint8

// Sources.
const (
	// SourceNone is no source. Nothing was found.
	SourceNone Source = iota
	// SourceForced is TERM_GRAPHICS.
	SourceForced
	// SourceTmux is the answer of tmux about the terminal of its client.
	SourceTmux
	// SourceQuery is the answers of the terminal to the queries of [Detect].
	SourceQuery
	// SourceEnv is the variables of the environment other than
	// TERM_GRAPHICS.
	SourceEnv
)

// String satisfies the [fmt.Stringer] interface.
func (s Source) String() string {
	switch s {
	case SourceNone:
		return "none"
	case SourceForced:
		return "forced"
	case SourceTmux:
		return "tmux"
	case SourceQuery:
		return "query"
	case SourceEnv:
		return "env"
	}
	return "unknown"
}

// Terminal is what [Detect] found about a terminal. A Terminal with only
// Protocol set is valid, for a program that knows the protocol, as when it
// writes to a file.
type Terminal struct {
	// Protocol is the protocol that the encoder uses. [None] is no graphics.
	Protocol Protocol
	// Source is where Protocol comes from.
	Source Source
	// Protocols is each protocol that the terminal reported, or that its name
	// tells, in the order of preference: Kitty, iTerm, Sixel. It is empty
	// when TERM_GRAPHICS selects the protocol.
	Protocols []Protocol
	// Name and Version are the name and the version that the terminal
	// reports, from XTVERSION, or from tmux.
	Name, Version string
	// Cell is the size of a cell in pixels. It is zero when it is not known.
	Cell image.Point
	// SixelColors is the number of color registers of Sixel, or 0 when it is
	// not known.
	SixelColors int
	// KittyAnimation is true when the terminal reported that it can play a
	// Kitty animation. False means that it did not report it: kitty before
	// 0.50.0 and Ghostty play animations and do not report it.
	KittyAnimation bool
	// Tmux is true when the program runs in tmux.
	Tmux bool
	// Passthrough is true when the allow-passthrough option of tmux is on.
	Passthrough bool
}

// Has returns true when p is in t.Protocols.
func (t Terminal) Has(p Protocol) bool {
	return slices.Contains(t.Protocols, p)
}

// Encoder returns the encoder for t.Protocol, set up for t. In tmux, Kitty
// and iTerm wrap their output in the passthrough of tmux (D20), and Kitty
// draws placeholder cells when t.Cell is known (D22). It returns nil when
// t.Protocol is [None].
func (t Terminal) Encoder(opts ...EncoderOption) Encoder {
	var o encoderOptions
	for _, opt := range opts {
		opt(&o)
	}
	switch t.Protocol {
	case Kitty:
		return KittyEncoder{NoNewline: o.noNewline, Zlib: o.zlib, Tmux: t.Tmux, Cell: t.Cell}
	case ITerm:
		return ITermEncoder{NoNewline: o.noNewline, Tmux: t.Tmux}
	case Sixel:
		return SixelEncoder{NoNewline: o.noNewline}
	case Halfblock:
		return HalfblockEncoder{NoNewline: o.noNewline}
	}
	return nil
}

// EncoderOption is an option of [Terminal.Encoder].
type EncoderOption func(*encoderOptions)

// encoderOptions are the options of each encoder.
type encoderOptions struct {
	noNewline bool
	zlib      ZlibMode
}

// WithNoNewline is an encoder option that writes no newline after the
// image.
func WithNoNewline() EncoderOption {
	return func(o *encoderOptions) {
		o.noNewline = true
	}
}

// WithZlib is an encoder option that sets how Kitty compresses raw pixels.
// The default is [ZlibAuto].
func WithZlib(mode ZlibMode) EncoderOption {
	return func(o *encoderOptions) {
		o.zlib = mode
	}
}

// Error is an error.
type Error string

// Error satisfies the [error] interface.
func (err Error) Error() string {
	return string(err)
}

// Errors.
const (
	// ErrTermGraphicsNotAvailable is the error when the terminal has no
	// graphics.
	ErrTermGraphicsNotAvailable Error = "term graphics not available"
	// ErrNonTTY is the error when standard input or standard output is not a
	// terminal.
	ErrNonTTY Error = "non tty"
	// ErrTermResponseTimedOut is the error when the terminal does not answer
	// in time.
	ErrTermResponseTimedOut Error = "term response timed out"
	// ErrUnknownProtocol is the error when text names no protocol.
	ErrUnknownProtocol Error = "unknown protocol"
	// ErrFrameOutOfBounds is the error when a frame of a Kitty animation is
	// not inside the root frame.
	ErrFrameOutOfBounds Error = "frame out of bounds"
	// ErrUnknownFrame is the error when a frame of a Kitty animation names a
	// base frame that does not exist.
	ErrUnknownFrame Error = "unknown frame"
	// ErrInvalidLoops is the error when the number of loops of a Kitty
	// animation is negative or too large.
	ErrInvalidLoops Error = "invalid loops"
	// ErrAnimationEnded is the error when a frame is added to a Kitty
	// animation after End, Stop or Delete.
	ErrAnimationEnded Error = "animation ended"
)
