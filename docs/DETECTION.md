# Draft: detection by query, and a new API

Status: the design record of D26, revision 2, 2026-11-14, after critiques
by Gemini 3.1 Pro, GPT-6 Astra (OpenAI Pro) and DeepSeek V4 Pro. A
maintainer accepted the revised draft on 2026-11-14, and the code follows
it. Backward compatibility did not matter (a maintainer, 2026-11-14).

The code differs from the revised draft at the end in two ways. `Detect`
takes a context and functional options, and `Terminal.Encoder` takes
functional options (D27). The implementation also made the choices of D28.
The README shows the API as it is. Revision 1 is kept below as it was
written.

## What was asked for

- Detect each protocol by a query to the terminal, not from the
  environment.
- `TERM_GRAPHICS` still forces a protocol, before any query.
- The other environment variables are checked last, when no query is
  possible or the terminal does not answer.
- Remove the `Available` API, and replace it with something more general.
- Decide if `TermType` stays.

## The API today

```go
type TermType uint8 // None, Kitty, ITerm, Sixel, Halfblock, Default
func (TermType) Available() bool
func (TermType) Encode(io.Writer, image.Image) error
func (TermType) EnvValue() string
func (TermType) String() string
func (TermType) MarshalText() ([]byte, error)
func (*TermType) UnmarshalText([]byte) error

func Encode(w io.Writer, img image.Image) error // Default
func Available() bool                          // Default
func PendingInput() []byte

type Encoder interface {
	Available() bool
	Encode(io.Writer, image.Image) error
}
type KittyEncoder struct{ NoNewline bool; Zlib ZlibMode }
type ITermEncoder struct{ NoNewline bool }
type SixelEncoder struct{ NoNewline bool }
type HalfblockEncoder struct{ NoNewline bool }
type DefaultEncoder struct{ /* first available of v */ }
func NewDefaultEncoder(v ...Encoder) *DefaultEncoder
```

Each `Available` reads package state that is set once: `TERM_GRAPHICS`,
the other environment variables, the tmux client (D21) and the Sixel query
(D23). Each `Encode` of Kitty and iTerm also reads whether the program runs
in tmux, and Kitty reads the cell size from tmux (D22). So an encoder is not
a pure value. A test cannot give an encoder a terminal.

The programs that use the API:

| Program | Uses |
| --- | --- |
| `iv` | `Kitty.Available`, `ITerm.Available`, `Encode`, `TermType` to pick the form of the image |
| `usql` | `TermType` from `TERM_GRAPHICS`, `PendingInput` |
| `chartfmt` | `TermType` as a parameter, `None` |
| `chromedp/termcast` | `NewDefaultEncoder` with `NoNewline` encoders, `Encoder` |
| `fv`, chromedp examples | `Available`, `Encode` |

## The queries

All the queries go out in one write. The reply to DA1 comes last, because
every terminal answers DA1 and answers in order. The package reads until the
DA1 reply, or until the 250 ms of D23 pass. It sets keys aside as D24 does.

| Query | Sequence | Reply | Gives |
| --- | --- | --- | --- |
| Kitty graphics | `ESC _G i=31,s=1,v=1,a=q,t=d,f=24;AAAA ESC \` | `ESC _G i=31;OK ESC \` | Kitty |
| Kitty animation | `ESC _G a=q,i=32,e=a ESC \` (kitty 0.50.0 and later) | `ESC _G i=32,e=a;OK ESC \`. A reply without `e`, or no reply, means not reported | animation, see open question 7 of PLAN.md |
| XTVERSION | `CSI > 0 q` | `DCS > \| name version ST` | the name and version, for iTerm and for the list of quirks |
| Sixel colors | `CSI ? 1 ; 1 ; 0 S` | `CSI ? 1 ; 0 ; n S` | the color registers of Sixel |
| Cell size | `CSI 16 t` | `CSI 6 ; h ; w t` | the size of a cell in pixels |
| DA1 | `CSI c` | `CSI ? Ps ; ... c` | Sixel (attribute 4), and the end of the replies |

No query asks for the inline images of iTerm. The name from XTVERSION is
compared with a list: iTerm2 and WezTerm. iTerm2 3.5 has a query for its
features, but its form is not checked yet.

In tmux, the package does not send these queries, because tmux answers DA1
and XTVERSION itself and drops the Kitty query. It asks tmux, as D21 does,
for `client_termtype`, `allow-passthrough` and the cell size. tmux got
`client_termtype` from the XTVERSION reply of the outer terminal, over SSH
too. tmux does not tell if the outer terminal has Kitty graphics, so the
name is compared with a list: kitty and Ghostty.

## The order

1. If `TERM_GRAPHICS` names a protocol, or `none`, use it. Send no query.
2. If the program runs in tmux, ask tmux.
3. If standard input and standard output are terminals, and the terminal is
   not dumb or Emacs, send the queries.
4. If no query was possible, or the terminal did not answer, read the other
   environment variables, as D3 does now: `KITTY_WINDOW_ID`, `TERM`,
   `TERM_PROGRAM`, `LC_TERMINAL`.

When the terminal has more than one protocol, the order is Kitty, then iTerm,
then Sixel. Half blocks are used only when `TERM_GRAPHICS` asks for them
(D12).

## Draft API

```go
// Protocol is a terminal graphics protocol.
type Protocol uint8

const (
	None Protocol = iota
	Kitty
	ITerm
	Sixel
	Halfblock
)

func (Protocol) String() string
func (Protocol) MarshalText() ([]byte, error)
func (*Protocol) UnmarshalText([]byte) error // as normalize reads it

// ParseProtocol returns the protocol that s names, as TERM_GRAPHICS does.
// ok is false when s names no protocol, as the empty string does.
func ParseProtocol(s string) (p Protocol, ok bool)

// Terminal is what the package knows about a terminal.
type Terminal struct {
	// Protocol is the protocol to use. It is None when the terminal has no
	// graphics.
	Protocol Protocol
	// Protocols is each protocol that the terminal has.
	Protocols []Protocol
	// Source is where Protocol comes from.
	Source Source
	// Name and Version are from XTVERSION, or from tmux.
	Name, Version string
	// Cell is the size of a cell in pixels. It is zero when not known.
	Cell image.Point
	// SixelColors is the count of the color registers of Sixel.
	SixelColors int
	// KittyAnimation is true when the terminal answers the query for animation.
	KittyAnimation bool
	// Tmux is true when the program runs in tmux, and Passthrough when
	// allow-passthrough is on.
	Tmux, Passthrough bool
}

// Source is where the protocol of a Terminal comes from.
type Source uint8

const (
	SourceForced Source = iota // TERM_GRAPHICS
	SourceTmux                 // tmux display-message
	SourceQuery                // the reply of the terminal
	SourceEnv                  // the other variables of the environment
)

// Detect finds the terminal of in and out, in the order above. It returns
// the keys that it read and that are not part of a reply (D24).
func Detect(ctx context.Context, in, out *os.File) (term Terminal, pending []byte, err error)

// Encoder returns the encoder for the protocol of t, set up for t: in tmux,
// Kitty and iTerm wrap their output in the passthrough (D20), and Kitty
// draws placeholder cells of the size of t.Cell (D22). It returns nil when
// t.Protocol is None.
func (t Terminal) Encoder() Encoder

// Encoder encodes an image.
type Encoder interface {
	Encode(w io.Writer, img image.Image) error
}

type KittyEncoder struct {
	NoNewline bool
	Zlib      ZlibMode
	Tmux      bool        // wrap in the passthrough, draw placeholder cells
	Cell      image.Point // the size of a cell, for placeholder cells
}
type ITermEncoder struct {
	NoNewline bool
	Tmux      bool
}
type SixelEncoder struct{ NoNewline bool }
type HalfblockEncoder struct{ NoNewline bool }

// Encode encodes img to w for the terminal of standard input and standard
// output. The first call runs Detect, and keeps its result. It returns
// ErrTermGraphicsNotAvailable when the terminal has no graphics.
func Encode(w io.Writer, img image.Image) error

// Default returns the Terminal that Encode uses. The first call runs Detect.
func Default() Terminal

// PendingInput returns the keys that Default read (D24).
func PendingInput() []byte
```

What goes away:

- `TermType` becomes `Protocol`. `Default` as a value of the type goes,
  because "the protocol that the terminal has" is now `Terminal.Protocol`.
- `Available` on each encoder, on `TermType`, and in the package. A program
  asks `Default().Protocol != None`, or `Default().Protocol == Kitty`.
- `DefaultEncoder` and `NewDefaultEncoder`. `Terminal.Encoder` picks the
  encoder. A program that wants `NoNewline` sets the field on the encoder
  that it gets.
- `TermType.Encode` and `EnvValue`.

How the programs change:

- `iv`: `rasterm.Default().Protocol` replaces `Kitty.Available` and
  `ITerm.Available`. `ivimg.Raw` takes a `Protocol`.
- `usql`: `ParseProtocol(os.Getenv("TERM_GRAPHICS"))`.
- `chartfmt`: `Protocol` replaces `TermType`.
- `termcast`: `rasterm.Default().Encoder()`, then sets `NoNewline`.

## Open points of revision 1

1. Is `Detect` with `context.Context` right, or a timeout parameter, or
   both?
2. Is `Terminal` a struct of fields, or does it need methods, such as
   `Has(Protocol) bool`?
3. Must `Encode` of the package run `Detect` itself, which reads standard
   input the first time, or must a program call `Detect` first?
4. `Terminal.Encoder` returns an interface, so a program cannot set
   `NoNewline` without a type switch. Is an options struct better?
5. Must a SSH session with no tmux, where the query works, still read the
   environment last?

## Critiques, 2026-11-14

All three models answered at low reasoning effort. Each claim about a
protocol was checked against the Kitty spec at
https://sw.kovidgoyal.net/kitty/graphics-protocol/, the man page of tmux
3.8, and D21.

Points that hold:

- Detection must be explicit. A package-level `Encode` that reads standard
  input the first time takes keys from a program such as `usql` at a time
  that the program does not choose. It also queries the terminal of
  standard input when `w` goes somewhere else. (All three.)
- An options struct, not a type switch, sets `NoNewline` and `Zlib`.
  (Gemini, OpenAI, DeepSeek.)
- A timeout, not a context. The query waits with `poll` (D23), and a context
  that is canceled cannot stop `poll` without a second file descriptor.
  (Gemini.)
- A protocol that `TERM_GRAPHICS` forces is a choice, not a fact about the
  terminal. `Protocols` must hold only what the terminal reported, or what
  its name tells. (OpenAI.)
- A `KittyAnimation` that is false can mean "not supported" or "not
  reported". Ghostty 1.4.0 has animation, but it can answer no feature
  query. The field must say "reported". (OpenAI.)
- The environment is read last over SSH too, because SSH often drops
  `KITTY_WINDOW_ID` and `TERM_PROGRAM`, and the query is then the best
  source. (Gemini, OpenAI.)

Points that are open:

- Query through the tmux passthrough, and not only ask tmux. (Gemini.) It
  is not tested whether tmux gives the reply of the outer terminal to the
  pane. It stays an open question until it is tested.
- `Halfblock` is not a protocol of the terminal. (DeepSeek.) It stays in
  `Protocol`, because `TERM_GRAPHICS=halfblock` selects it, but the query
  never puts it in `Protocols`.

Claims that are wrong:

- Gemini and OpenAI: `client_termtype` is the `TERM` of the client. The man
  page says "Terminal type of client, if available", and the tmux server of a maintainer
  returned `ghostty 1.3.1-arch2` (D21). That is the XTVERSION reply of
  Ghostty. Its `TERM` is `xterm-ghostty`.
- Gemini and OpenAI: `e=a` is not a query for animation. The Kitty spec
  added it in kitty 0.50.0, with `e=f` and `e=c`. The reply repeats `e`
  when the terminal supports the action.
- OpenAI and DeepSeek: DA1 is not a reliable end of the replies. The Kitty
  spec says that a terminal answers a query at once, in order, and
  recommends DA1 after the query for this reason.

## Revised draft

```go
// Protocol is a terminal graphics protocol.
type Protocol uint8

const (
	None Protocol = iota
	Kitty
	ITerm
	Sixel
	Halfblock // only when TERM_GRAPHICS selects it (D12)
)

func (Protocol) String() string
func (Protocol) MarshalText() ([]byte, error)
func (*Protocol) UnmarshalText([]byte) error

// ParseProtocol returns the protocol that s names, as TERM_GRAPHICS is read.
// ok is false when s names no protocol, as the empty string does.
func ParseProtocol(s string) (p Protocol, ok bool)

// Terminal is what the package found about a terminal. The zero value with
// only Protocol set is a valid Terminal, for a program that knows the
// protocol, as when it writes to a file.
type Terminal struct {
	// Protocol is the protocol that the encoder uses. None is no graphics.
	Protocol Protocol
	// Source is where Protocol comes from.
	Source Source
	// Protocols is each protocol that the terminal reported, or that its
	// name tells. It is empty when TERM_GRAPHICS forces the protocol.
	Protocols []Protocol
	// Name and Version are from XTVERSION, or from client_termtype in tmux.
	Name, Version string
	// Cell is the size of a cell in pixels. It is zero when not known.
	Cell image.Point
	// SixelColors is the count of the color registers of Sixel, or 0.
	SixelColors int
	// KittyAnimation is true when the terminal reported animation (e=a).
	// False means that it was not reported.
	KittyAnimation bool
	// Tmux is true in tmux. Passthrough is true when allow-passthrough is on.
	Tmux, Passthrough bool
}

// Source is where the protocol of a Terminal comes from.
type Source uint8

const (
	SourceNone   Source = iota // nothing found
	SourceForced               // TERM_GRAPHICS
	SourceTmux                 // tmux display-message
	SourceQuery                // the replies of the terminal
	SourceEnv                  // the other variables of the environment
)

// Has returns true when p is in t.Protocols.
func (t Terminal) Has(p Protocol) bool

// Detect finds the terminal of in and out, in the order of this document. It
// waits up to timeout for the replies. It returns the keys that it read and
// that are not part of a reply (D24). A terminal that does not answer is not
// an error: Detect then reads the environment. An error is a failure to read
// or write.
func Detect(in, out *os.File, timeout time.Duration) (t Terminal, pending []byte, err error)

// EncoderOptions are the options of each encoder.
type EncoderOptions struct {
	NoNewline bool
	Zlib      ZlibMode // Kitty only
}

// Encoder returns the encoder for t.Protocol, set up for t: in tmux, Kitty
// and iTerm wrap their output in the passthrough (D20), and Kitty draws
// placeholder cells of the size of t.Cell (D22). It returns nil when
// t.Protocol is None.
func (t Terminal) Encoder(opts EncoderOptions) Encoder

// Encoder encodes an image.
type Encoder interface {
	Encode(w io.Writer, img image.Image) error
}

// The encoders stay exported, for a test and for a program that sets up an
// encoder itself. Each field that was a package global is now a field.
type KittyEncoder struct {
	EncoderOptions
	Tmux bool        // wrap in the passthrough, draw placeholder cells
	Cell image.Point // the size of a cell, for placeholder cells
}
type ITermEncoder struct {
	EncoderOptions
	Tmux bool
}
type SixelEncoder struct{ EncoderOptions }
type HalfblockEncoder struct{ EncoderOptions }
```

What goes away, in addition to revision 1: the package-level `Encode`,
`Default` and `PendingInput`. A program calls `Detect` once, before it reads
standard input, and keeps the `Terminal`. A program that only writes images
does this:

```go
t, _, err := rasterm.Detect(os.Stdin, os.Stdout, 250*time.Millisecond)
if err != nil {
	return err
}
enc := t.Encoder(rasterm.EncoderOptions{})
if enc == nil {
	return rasterm.ErrTermGraphicsNotAvailable
}
return enc.Encode(os.Stdout, img)
```

## Questions for the maintainers

D26 in [PLAN.md](PLAN.md) holds the revised draft. A maintainer accepted it
on 2026-11-14, and with it the answers of the revised draft to questions
1, 2 and 4: the package-level `Encode` goes, `TermType` becomes `Protocol`,
and `Detect` reads `TERM_GRAPHICS`. Question 3 is open question 9 in
PLAN.md.

1. Does the package-level `Encode` go, so that each program calls `Detect`
   itself? The models agree that it must. `fv` and the chromedp examples
   then need four more lines each.
2. Is `TermType` renamed to `Protocol`, or does the name stay?
3. In tmux, does rasterm also send the queries through the passthrough, if
   a test shows that tmux gives the replies to the pane? Today it only asks
   tmux.
4. Is `TERM_GRAPHICS` still read by `Detect`, or does the program read it
   and give it to `Detect`? `usql` reads it itself today.
