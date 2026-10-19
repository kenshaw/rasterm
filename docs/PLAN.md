# Plan

This document holds the plan for `rasterm`, then each decision that shapes
it, then the open questions for Ken. `rasterm` is a small library, so its
decisions stay in this document and do not get one file each (dbmeta D111).

## Purpose

`rasterm` encodes a Go `image.Image` as terminal graphics. A terminal that
reads the graphics draws the image in its cells. The package writes three
protocols: Kitty, iTerm and Sixel. It can also draw an image as text, with
half blocks, when the environment asks for it (D12). It finds from the environment which
protocol the terminal reads (D3), and its default encoder takes the first
one that is available (D4).

The main user is `github.com/kenshaw/iv`, a command-line image viewer. `iv`
draws each image and each frame of an animation through `rasterm.Encode`.

## Current state

The state below is at `eacef4b` (2026-06-23), with the work of 2026-10-15.

- `KittyEncoder`, `ITermEncoder` and `SixelEncoder` each satisfy `Encoder`.
  `DefaultEncoder` wraps all three. `Kitty`, `ITerm`, `Sixel` and `Default`
  are `TermType` values that find the encoder for each.
- Each encoder builds its output in memory and writes it in one call (D9).
- Kitty and iTerm send the bytes of a `PNGImage` as they are, and do not
  encode the image again (D10).
- iTerm sends the bytes of a `JPEGImage` as they are, and Kitty sends the
  pixels of an `RGBImage` or an `RGBAImage` as they are (D13, D14).
- `HalfblockEncoder` draws an image as text when `TERM_GRAPHICS` is
  `halfblock` (D12).
- `go test ./...` writes each image in `testdata/` to the terminal that runs
  it, through the default encoder. CI sets `TERM_GRAPHICS=sixel`, and runs
  on Linux, macOS and Windows (D16).
- The module declares `go 1.26.0` (D15). The code uses the idioms of that
  release, and `go fix -diff ./...` reports no change.

## Direction

No document states a direction beyond the work in
[BACKLOG.md](BACKLOG.md). Ask Ken before you start a new feature that is not
in that list.

## Decisions

Each decision that shapes `rasterm`, in the order that it was made. The
entries are append only. A decision is never edited to change its
conclusion. When a later decision changes one, the older entry keeps its
text, and its heading names the decision that changed it.

Each heading carries a status after the title. `Decided` means that the
history shows that Ken chose it. `Proposed` means that the code follows it
and nothing shows that Ken chose it, so ask him. After the status,
`Amends Dn` or `Amended by Dn` names a decision that this one changes or
that changes it.

D1 to D8 were written on 2026-10-15 from the history of the repository, from
`a297887` (2021-03-13) to `eacef4b` (2026-06-23). Most commit messages are
short. If no commit message, comment or document gives a reason, the entry
says "Reason not recorded".

A reference to a decision of another repository names that repository, as in
dbmeta D110. A bare number, as in D3, names a decision in this document.

| Decision | Title | Status |
| --- | --- | --- |
| [D1](#d1-rasterm-is-a-rewrite-of-bourgeoisbearrasterm-decided) | rasterm is a rewrite of BourgeoisBear/rasterm | Decided |
| [D2](#d2-rasterm-writes-kitty-iterm-and-sixel-and-go-sixel-writes-the-sixel-decided) | rasterm writes Kitty, iTerm and Sixel, and go-sixel writes the Sixel | Decided |
| [D3](#d3-the-environment-selects-the-protocol-and-term_graphics-overrides-it-decided-amended-by-d17-amended-by-d21) | The environment selects the protocol, and TERM_GRAPHICS overrides it | Decided. Amended by D17. Amended by D21 |
| [D4](#d4-the-default-encoder-takes-the-first-available-of-kitty-iterm-and-sixel-decided-amended-by-d12) | The default encoder takes the first available of Kitty, iTerm and Sixel | Decided. Amended by D12 |
| [D5](#d5-iterm-gets-a-png-for-a-paletted-image-and-a-jpeg-for-any-other-decided) | iTerm gets a PNG for a paletted image, and a JPEG for any other | Decided |
| [D6](#d6-a-sentinel-error-is-a-constant-of-type-error-decided) | A sentinel error is a constant of type Error | Decided |
| [D7](#d7-rasterm-makes-no-promise-of-backward-compatibility-proposed) | rasterm makes no promise of backward compatibility | Proposed |
| [D8](#d8-rasterm-is-set-up-for-coding-agents-as-every-xo-repository-is-decided) | rasterm is set up for coding agents as every xo repository is | Decided |
| [D9](#d9-each-encoder-writes-an-image-in-one-call-to-write-decided) | Each encoder writes an image in one call to Write | Decided |
| [D10](#d10-kitty-and-iterm-send-the-bytes-of-a-pngimage-as-they-are-decided) | Kitty and iTerm send the bytes of a PNGImage as they are | Decided |
| [D11](#d11-png-encoding-uses-the-fastest-compression-level-proposed) | PNG encoding uses the fastest compression level | Proposed |
| [D12](#d12-a-half-block-encoder-draws-the-image-as-text-only-when-term_graphics-asks-for-it-decided-amends-d4-amended-by-d17) | A half-block encoder draws the image as text, only when TERM_GRAPHICS asks for it | Decided. Amends D4. Amended by D17 |
| [D13](#d13-each-format-that-an-encoder-can-send-as-is-has-its-own-interface-decided) | Each format that an encoder can send as is has its own interface | Decided |
| [D14](#d14-kitty-sends-the-pixels-of-an-rgbimage-or-an-rgbaimage-as-they-are-decided-amended-by-d18) | Kitty sends the pixels of an RGBImage or an RGBAImage as they are | Decided. Amended by D18 |
| [D15](#d15-the-module-requires-the-newest-golangorgx-modules-and-the-go-release-that-they-need-decided) | The module requires the newest golang.org/x modules, and the Go release that they need | Decided |
| [D16](#d16-ci-tests-on-linux-macos-and-windows-on-the-go-release-of-gomod-and-the-latest-decided) | CI tests on Linux, macOS and Windows, on the Go release of go.mod and the latest | Decided |
| [D17](#d17-a-protocol-that-term_graphics-names-turns-off-each-other-protocol-decided-amends-d3-amends-d12) | A protocol that TERM_GRAPHICS names turns off each other protocol | Decided. Amends D3. Amends D12 |
| [D18](#d18-kitty-compresses-raw-pixels-with-zlib-when-they-compress-well-decided-amends-d14-amended-by-d19) | Kitty compresses raw pixels with zlib when they compress well | Decided. Amends D14. Amended by D19 |
| [D19](#d19-the-zlib-stream-is-never-flushed-decided-amends-d18) | The zlib stream is never flushed | Decided. Amends D18 |
| [D20](#d20-in-tmux-kitty-and-iterm-wrap-each-escape-sequence-in-the-dcs-passthrough-decided-amended-by-d21-amended-by-d22) | In tmux, Kitty and iTerm wrap each escape sequence in the DCS passthrough | Decided. Amended by D21. Amended by D22 |
| [D21](#d21-in-tmux-kitty-and-iterm-ask-tmux-for-the-terminal-and-for-allow-passthrough-proposed-amends-d3-amends-d20-amended-by-d22) | In tmux, Kitty and iTerm ask tmux for the terminal and for allow-passthrough | Proposed. Amends D3. Amends D20. Amended by D22 |
| [D22](#d22-in-tmux-kitty-draws-an-image-with-unicode-placeholder-cells-decided-amends-d20-amends-d21) | In tmux, Kitty draws an image with Unicode placeholder cells | Decided. Amends D20. Amends D21 |

### D1. rasterm is a rewrite of BourgeoisBear/rasterm. Decided.

Date: 2023-11-19. Commit: `49c2256`.

Jason Stewart wrote `github.com/BourgeoisBear/rasterm` in 2021. Ken forked
it and rewrote it in one commit, "Rewrite". The rewrite put the three
encoders behind one `Encoder` interface, added `TermType`, and moved the test
images to `testdata/`. The README says "Fork/rewrite of `rasterm`" and links
the original. The commit message gives no other reason.

### D2. rasterm writes Kitty, iTerm and Sixel, and go-sixel writes the Sixel. Decided.

Date: 2023-11-19. Commit: `49c2256`.

The original package had its own Sixel encoder in `sixel.go`. The rewrite
deleted it and calls `github.com/mattn/go-sixel` instead. Reason not
recorded. The Kitty and iTerm encoders stay in this package.

### D3. The environment selects the protocol, and TERM_GRAPHICS overrides it. Decided. Amended by D17. Amended by D21.

Date: 2024-01-11. Commits: `8fbd4fc`, `640e270`, `5466e05`, `dc6c752`,
`6c01d0a`.

`TERM_GRAPHICS` names one protocol: `kitty`, `iterm` or `sixel`. Its value
makes that encoder available. The value `none` makes each encoder
unavailable. Without it, each encoder reads other variables:

- Kitty: `TERM` is `xterm-kitty`, `TERM_PROGRAM` is `ghostty` (`dc6c752`,
  2025-01-02), or both `KITTY_PID` and `KITTY_WINDOW_ID` are set (`6c01d0a`,
  2025-05-21).
- iTerm: `TERM` is `mintty`, `LC_TERMINAL` is `iterm2`, or `TERM_PROGRAM`
  is `wezterm`.
- Sixel: the terminal answers a request for its device attributes with
  attribute 4. This needs a terminal on standard input and output, and it
  works on Unix only.

### D4. The default encoder takes the first available of Kitty, iTerm and Sixel. Decided. Amended by D12.

Date: 2023-11-19. Commits: `49c2256`, `48a9f45`.

`init` in `rasterm.go` builds `DefaultEncoder` from the Kitty, iTerm and
Sixel encoders, in that order. It selects one the first time that it is used,
and keeps it. `48a9f45` added the package-level `Available` for it. `iv`
depends on the order: it sends a Kitty delete command before each frame of an
animation when Kitty is available, because Kitty is the encoder that the
default takes.

### D5. iTerm gets a PNG for a paletted image, and a JPEG for any other. Decided.

Date: 2021-03-16. Commit: `707d360`. Kept in `49c2256`.

Jason Stewart made the iTerm encoder send a PNG only for an
`*image.Paletted`, and a JPEG for every other image. The commit message
names "wez perf issue", which is a performance problem in WezTerm. Ken kept
the rule in the rewrite. `DefaultJPEGQuality` sets the quality, and is 93.
D10 adds an exception: a `PNGImage` is sent as its PNG.

### D6. A sentinel error is a constant of type Error. Decided.

Date: 2023-11-19. Commit: `49c2256`.

`rasterm.go` declares `type Error string` and the constants
`ErrTermGraphicsNotAvailable`, `ErrNonTTY`, `ErrTermResponseTimedOut` and
`ErrUnknownTermType`. A constant cannot be changed by another package, and
it compares with `==` and `errors.Is`. The `xo` repositories use the same
form, as in terminfo D11 and dbmeta D32.

### D7. rasterm makes no promise of backward compatibility. Proposed.

Date: 2026-10-15. Source: tblfmt D37.

tblfmt D37 records that no `xo` project makes a promise about its API. A
change can rename or remove an exported identifier when that makes the code
better. `rasterm` is under `github.com/kenshaw`, and nothing shows that Ken
applied this rule here. The tags are all `v0`, which makes no promise under
the rules of Go modules.

### D8. rasterm is set up for coding agents as every xo repository is. Decided.

Date: 2026-10-15. Source: dbmeta D110, dbmeta D111 and iv D21.

Ken asked on 2026-10-15 for the agent setup of the `xo` projects, with
`xo/dbmeta` as a template and not as text to copy. `iv` uses the same setup
(iv D21). The setup is this:

- The root holds four documents: `README.md`, `AGENTS.md`, `CLAUDE.md` and
  `CONTRIBUTING.md`. Every other document goes in `docs/`.
- `AGENTS.md` holds the rules for a coding agent, and opens with the three
  standing rules of dbmeta D110. `CLAUDE.md` is an ordinary file that holds
  the one line `@AGENTS.md`.
- `simple-english` and `go-pedantry` are ordinary folders in
  `.agents/skills` and in `.claude/skills`, installed with `--copy`.
  `skills-lock.json` names the source of each.
- `.gitignore` ignores `.claude/settings.local.json`, and `.gitattributes`
  holds `* text=auto eol=lf`. `testdata/` holds images, and git does not
  convert them.
- This document holds the plan, the decisions and the open questions.
  [BACKLOG.md](BACKLOG.md) holds the work that is known and not done.

`docs_test.go` holds the checks. `TestSkillsAreCopies` fails on a symbolic
link, on a missing copy, on two copies that differ, and on a skill that
`skills-lock.json` does not name. `TestClaudeImportsAgents` fails unless
`CLAUDE.md` holds only `@AGENTS.md`. `TestTheRootHoldsFourDocuments` fails
on any other document in the root. `TestTheDecisionIndexIsComplete` checks
the index above against the headings. `TestAnAmendmentPointsBothWays` and
`TestEveryDecisionReferenceExists` check the references between decisions.

### D9. Each encoder writes an image in one call to Write. Decided.

Date: 2026-10-15.

Ken asked on 2026-10-15 for faster output, by building the output in memory
before it goes to the terminal. Each encoder now builds the escape sequences
and the newline in a buffer from a `sync.Pool`, and then calls `Write` once.
A buffer of more than 64 MiB does not go back to the pool.

Before, the Kitty encoder made one call to `fmt.Fprintf` for each chunk of
4096 bytes, which is 398 calls for a 1.6 MB image. `go-sixel` v0.0.8 made a
call for each few bytes when the writer was not an `*os.File`, which is
474250 calls for one test image. Each call on a terminal is a system call.
One call also gives the terminal the whole image at once.

The Kitty encoder now base64 encodes the PNG in slices of 3072 bytes. Each
slice gives a chunk of 4096 bytes, which is the largest chunk that the
protocol allows. The bytes are the same as before, and
`TestAppendKittyMatchesChunkEncode` compares the two. PNG encoding keeps its
zlib writer in a pool, and converts an `*image.YCbCr` or `*image.CMYK` to
`*image.RGBA` first, because `image/png` reads any type other than four
through `At`, one pixel at a time. `BenchmarkEncode` measures each encoder,
and `TestEncodeWritesOnce` fails on a second call.

On 2026-10-15, `benchstat` compared 6 runs of `BenchmarkEncode` before and
after, both with `go-sixel` v0.0.14, on an AMD Ryzen 9 9950X:

- Kitty takes 15% to 82% less time. A JPEG takes 58% less, and its
  allocations fall from 839096 to about 12 for each image.
- Each encoder makes one call to `Write`. Kitty made up to 398, and iTerm
  and Sixel made 2.
- Memory allocated falls by 80% over all the test files, because the
  `png.Encoder` keeps its zlib writer in the pool.
- Sixel and the iTerm JPEG are the same within 2% to 5%. A CPU profile of
  the iTerm JPEG shows only `image/jpeg`. `go-sixel` v0.0.14 builds its
  output in memory and writes it in one call, so this package adds only the
  newline to that write.

### D10. Kitty and iTerm send the bytes of a PNGImage as they are. Decided.

Date: 2026-10-15.

Ken asked on 2026-10-15 that the Kitty encoder send the bytes of a PNG that
it already has, and not decode and encode the image again. The use is `iv`,
which reads a PNG file and often draws it without a change.

`PNGImage` is an `image.Image` with a `PNG() []byte` method. An encoder finds
it with a type assertion, as `io.Copy` finds an `io.WriterTo`. `PNG` is a
struct that satisfies it, and `DecodePNG` makes one from the bytes of a file.
Kitty sends a PNG as it is, with `f=100`. iTerm sends any image file, so it
sends the PNG too, in place of the JPEG of D5. Sixel cannot, and encodes the
image that `PNG.Unwrap` returns, so that the fast path of `go-sixel` for an
`*image.Paletted` still works.

Before an encoder sends the bytes, it checks the first 24 bytes: the PNG
signature, the `IHDR` chunk, and a width and a height equal to the bounds of
the image. If a check fails, it encodes the image. The check finds an image
that was scaled or cropped after the PNG was made. It does not find other
changes to the pixels. `BenchmarkEncodePNG` measures it. On 2026-10-15,
Kitty sent `testdata/orca.png` in 80 µs as it is, and in 15.2 ms when it
encoded the image again. For `testdata/png1.png`, the times were 4.8 µs and
996 µs.

Gemini 3.1 Pro reviewed the design on 2026-10-15. It proposed the `Unwrap`
method and the signature check, and both are in. Open question 4 holds the
choices that it questioned.

### D11. PNG encoding uses the fastest compression level. Proposed.

Date: 2026-10-15.

`DefaultPNGCompression` is `png.BestSpeed`. `image/png` used
`png.DefaultCompression` before. The Kitty encoder and the iTerm encoder of a
paletted image use it. The variable has the form of `DefaultJPEGQuality`.
Ken did not choose the level.

`BenchmarkPNGCompression` measures the Kitty encoder at three levels. On
2026-10-15 it gave these times and sizes:

| File | DefaultCompression | BestSpeed | NoCompression |
| --- | --- | --- | --- |
| `jpg1.jpg` | 33.7 ms, 928 KB | 26.3 ms, 992 KB | 4.0 ms, 3.36 MB |
| `orca.png` | 18.9 ms, 224 KB | 14.8 ms, 233 KB | 3.8 ms, 5.33 MB |
| `png1.png` | 1.15 ms, 26 KB | 0.96 ms, 34 KB | 0.10 ms, 197 KB |

`BestSpeed` takes 15% to 25% less time than the default, and sends 4% to 32%
more bytes. `NoCompression` takes 80% to 90% less time, and sends 4 to 27
times the bytes. The terminal must read and decode each byte, and no
benchmark here measures that. The best level depends on the terminal and
on the connection to it.

### D12. A half-block encoder draws the image as text, only when TERM_GRAPHICS asks for it. Decided. Amends D4. Amended by D17.

Date: 2026-10-15.

Ken asked on 2026-10-15 for a half-block encoder, with the features of the
half-block renderer of `github.com/blacktop/go-termimg` v0.1.26, and only if
it needs few new dependencies. That renderer calls
`github.com/charmbracelet/x/mosaic`. It scales the image to a size in cells,
or to the size of the terminal, with four scale modes, and it can dither.
`mosaic` adds a threshold, inverted colors and quarter blocks.
`github.com/kenshaw/blocked` cannot draw the colors, because its `Bitmap`
holds one bit for each pixel.

Ken chose these rules on 2026-10-15:

- The encoder does not scale. The caller, such as `iv`, scales the image
  first. Each cell is the `▀` character, and shows the upper pixel in the
  foreground color and the lower pixel in the background color. So the
  encoder needs only the standard library.
- It is not a fallback. `HalfblockEncoder.Available` is true only when
  `TERM_GRAPHICS` is `halfblock`. `Default` tries it after Kitty, iTerm and
  Sixel, so it draws only when a person asks for it, and when no other
  protocol is detected. iv D1 says that `iv` never draws an image as text
  without that request.
- It has none of the other features of `go-termimg` or `mosaic`. There is
  no dithering, because each cell has 24-bit color and there is no palette
  to dither to.

A pixel with an alpha of less than half shows the terminal background. The
encoder writes a color sequence only when the color changes, ends each row
with a reset, and writes the image in one call (D9).

On 2026-10-15, `mosaic` and this encoder drew each test image to the same
number of cells. This encoder took 7 to 39 times less time. For
`testdata/orca.png`, it took 15 ms and 47 allocations, and `mosaic` took 232
ms and 11971359 allocations. `mosaic` also scales the image, which this
encoder does not do. The comparison ran in a separate module, so that
`rasterm` does not require `mosaic`.

### D13. Each format that an encoder can send as is has its own interface. Decided.

Date: 2026-10-15.

Ken chose on 2026-10-15 to add `JPEGImage`, with the method `JPEG() []byte`,
in the form of `PNGImage` (D10), and then `RGBImage` and `RGBAImage`. He gave
the reason: a type switch is much faster in any calling package. So the
package has one small interface for each format, and not one general method
such as `Encoded() (mime string, data []byte)`, which a caller must call and
then compare a string.

Each interface has a struct that satisfies it and an `Unwrap` method: `PNG`,
`JPEG`, `RGB` and `RGBA`. `DecodeJPEG` makes a `JPEG` from the bytes of a
file, as `DecodePNG` does. An encoder that cannot send the format encodes the
image that `Unwrap` returns.

Only iTerm takes a JPEG. Kitty takes a PNG and raw pixels only, and Sixel and
half blocks need the pixels. iTerm sends a PNG before a JPEG when an image
has both, and both are exceptions to D5. Before iTerm sends a JPEG, it reads
the markers of the file up to the first frame header, and checks the width
and height in it against the bounds of the image. `TestJPEGMatches` and
`FuzzJPEGMatches` test that scan on short and broken files. On 2026-10-15,
13 million fuzz inputs did not make it panic.

`BenchmarkEncodeJPEG` measures iTerm. On 2026-10-15, iTerm sent
`testdata/jpg1.jpg` in 59 µs as it is, and in 8.31 ms when it encoded the
image again. The output was 115 KB and not 171 KB, because the file is
smaller than the JPEG of quality 93 that the encoder makes.

A JPEG file can hold an orientation in its EXIF data. Go does not apply it,
and a terminal can. A rotated JPEG can then show in a different orientation
when it is sent as is than when it is encoded again.

### D14. Kitty sends the pixels of an RGBImage or an RGBAImage as they are. Decided. Amended by D18.

Date: 2026-10-15.

Ken asked on 2026-10-15 for `RGB` and `RGBA` forms of D13, for the encoders
that take raw pixels. Only Kitty does. It sends 24-bit pixels with `f=24` and
32-bit pixels with `f=32`, and gives the width and the height with `s` and
`v`. The bytes are packed with no padding at the end of a row, and the alpha
is not premultiplied, as in `image.NRGBA`. Kitty takes a PNG first, then RGB,
then RGBA. Before it sends raw pixels, it checks that their length is 3 or 4
times the number of pixels in the bounds. The data is not compressed, so
Kitty does not get `o=z`.

`BenchmarkEncodeRaw` measures both. On 2026-10-15, Kitty sent
`testdata/jpg1.jpg` as RGB in 1.1 ms and 3.36 MB, as RGBA in 1.5 ms and
4.48 MB, and as a PNG in 26 ms and 0.99 MB. For `testdata/orca.png`, RGB
took 1.8 ms and 5.33 MB, and the PNG took 14.5 ms and 0.23 MB. Raw pixels
take less time to encode, and the terminal must read 3 to 23 times the bytes.
A caller that has the pixels, such as a decoder in `iv` that returns them,
chooses which to send.

### D15. The module requires the newest golang.org/x modules, and the Go release that they need. Decided.

Date: 2026-10-15.

Ken first asked on 2026-10-15 that the module build with Go 1.25, with the
code in the style of Go 1.27. `golang.org/x/term` v0.46.0 and later, and
`golang.org/x/sys` v0.48.0 and later, declare `go 1.26.0`. The newest
releases that build with Go 1.25 were `x/term` v0.45.0 and `x/sys` v0.47.0.
Ken then chose the newest `golang.org/x` modules, and said that `go 1.26` is
fine. So `go.mod` declares `go 1.26.0`, with `x/term` v0.47.0 and `x/sys`
v0.49.0.

The code uses the idioms that Go 1.26 allows, such as `sync.OnceValue`,
`slices.Contains`, `WaitGroup.Go`, `range` over an integer, and a loop
variable that is not copied. The modernizers of `go fix` in Go 1.27 report
no change. They suggest only what the go directive allows.

### D16. CI tests on Linux, macOS and Windows, on the Go release of go.mod and the latest. Decided.

Date: 2026-10-15.

Ken asked on 2026-10-15 for CI on `ubuntu-latest`, `macos-latest` and
`windows-latest`, with the latest version of each action. On that date,
these were `actions/checkout@v7` (v7.0.1) and `actions/setup-go@v7`
(v7.0.0). Each platform runs on Go 1.26, which `go.mod` declares (D15), and
on the latest stable release. A job builds and tests, with
`TERM_GRAPHICS=sixel` so that `TestEncode` has an encoder. One job, on Linux
with the latest release, also checks the formatting and runs `go vet`.
`term_unix.go` builds on Linux and macOS, and `term_other.go` on Windows, so
each platform builds different code.

### D17. A protocol that TERM_GRAPHICS names turns off each other protocol. Decided. Amends D3. Amends D12.

Date: 2026-10-15.

On 2026-10-15, Ken set `TERM_GRAPHICS=halfblock` in Ghostty and ran `iv`, and
`iv` drew with Kitty. Each encoder checked only for the value `none`. The
Kitty encoder found `TERM_PROGRAM=ghostty`, and `Default` takes Kitty first
(D4). So the variable added a protocol, and did not override the others, as
the title of D3 says. `TERM_GRAPHICS=sixel` in Kitty or Ghostty had the same
fault, before D12.

Now, when `TERM_GRAPHICS` is `kitty`, `iterm`, `sixel`, `halfblock` or
`none`, each encoder for another protocol is not available, whatever the
other variables say. The Sixel encoder then also does not ask the terminal
for its device attributes. A value that names no protocol, such as the
empty string, leaves each encoder to detect its protocol from the
environment, as before. `TestOtherProtocol` checks each value.

Ken asked on 2026-10-15 that the value forgive small errors.
`normalize` reads it in lower case, removes each character that is not a
letter, and removes a final `s`. So `half-block`, `half-blocks`,
`halfblocks` and `HalfBlock` each select half blocks, `sixels` selects Sixel
and `iTerm2` selects iTerm. `TermType.UnmarshalText` reads a name the same
way. `TestNormalize` checks each form.

D12 said that `Default` draws half blocks only when no other protocol is
detected. Now `TERM_GRAPHICS=halfblock` selects half blocks in any terminal.
The `iv` code that sends a Kitty delete command before each frame of an
animation reads `rasterm.Kitty.Available`, so it also stops when another
protocol is selected.

### D18. Kitty compresses raw pixels with zlib when they compress well. Decided. Amends D14. Amended by D19.

Date: 2026-10-15.

Ken asked through the `iv` session on 2026-10-15 for zlib compression (`o=z`)
of raw pixels, and he chose the threshold. `iv` now sends raw RGB and RGBA
through `RGBImage` and `RGBAImage`. D14 sent them uncompressed.

`KittyEncoder.Zlib` sets the mode. `ZlibAuto` is the zero value and the
default, and `ZlibAlways` and `ZlibNever` force each choice. A PNG is never
compressed again, because it is compressed already. In `ZlibAuto`:

1. Raw pixels under 64 KiB are sent raw.
2. For more, the encoder starts one zlib stream at level 1 (RFC 1950).
   After each 64 KiB of input, it flushes the stream and compares all the
   output so far with all the input so far.
3. At the first check where the output is not less than 0.25 of the input,
   the encoder drops the stream and sends the pixels raw. The work before
   that check is the only waste. If each check passes, it sends the whole
   stream with `o=z`. A last piece under 64 KiB is not checked.

Kitty reads one zlib stream across all the chunks: zlib, then base64, then
chunks of 4096 bytes. The writer is closed before the last chunk. `s` and
`v` are the same as for raw pixels. The zlib writers are kept in a
`sync.Pool`. `TestKittyZlib` decodes the escape sequences back to the pixels
in each mode.

`iv` measured the choice in its own images. With level 1, flat and rendered
images compressed to 0.02 to 0.21 of the raw size, and two photos to 0.39
and 0.59. Gemini reviewed the design. It replaced a sample of the image with
the check on the live stream, and it advised against reading `SSH_CONNECTION`
or `SSH_TTY` to find a remote terminal, because tmux and containers keep
stale values.

The first design checked once, after the first 64 KiB. That reads only the
top of the image. For `testdata/jpg1.jpg`, a photo, the top compressed to
less than 0.25, so `ZlibAuto` compressed the whole photo, in 11 ms in place
of 1.35 ms. Ken chose on 2026-10-15 to check after each 64 KiB. The ratio is
of all the output to all the input, so that a flat page with a photo in it
still compresses. `TestKittyZlib` has a flat top over noise, which the first
design compressed and this one sends raw.

`BenchmarkKittyZlib` measures each mode. On 2026-10-15, with the RGBA of each
image:

| Image | Raw | `ZlibAuto`, one check | `ZlibAuto`, each 64 KiB |
| --- | --- | --- | --- |
| `orca.png` | 6938 KiB, 2.12 ms | 309 KiB, 4.65 ms | 329 KiB, 5.00 ms |
| `jpg1.jpg` | 4378 KiB, 1.35 ms | 1542 KiB, 10.99 ms | 4378 KiB, 3.69 ms |
| `jpg2.jpg` | 3055 KiB, 0.97 ms | 3055 KiB, 1.15 ms | 3055 KiB, 1.16 ms |
| noise, 1024 x 1024 | 5473 KiB, 1.68 ms | 5473 KiB, 1.68 ms | 5473 KiB, 1.71 ms |

The flush at each check costs a flat image about 6% more bytes and 7% more
time. A photo with a flat top still costs the time to compress the top, which
is 2.3 ms for `jpg1.jpg`.

### D19. The zlib stream is never flushed. Decided. Amends D18.

Date: 2026-10-15.

On 2026-10-15, the output of `ZlibAuto` in v0.1.19 closed Ghostty 1.3.1 for
Ken. The image was `testdata/gxps/example.xps` from `iv`, at 1390 x 1966
RGBA. The same pixels in one zlib stream with no flush worked, and so did
raw pixels and a PNG. Python and Zig 0.16 inflated every stream to the
right size, so the fault is in the inflater of Ghostty. The `iv` session
found the cause and proposed the fix.

D18 flushed the stream at each check, so that the output held each byte of
the input so far. At `BestSpeed`, each flush adds a stored block of one byte
and an empty stored block, which is the sync flush marker. The stream of the
XPS page had 166 of each.

Now the stream is never flushed. At each check, the encoder compares the
output so far with the input so far, as D18 says, and the output lags the
input by at most one block. So the ratio reads low, and a photo goes raw one
or two checks later. A stream that is sent is the same as one stream of all
the pixels. `TestKittyZlibIsOneStream` checks that the stream is equal to one
stream with no flush, and that it holds no sync flush marker. It fails on
v0.1.19.

`compress/flate` of Go 1.26 ends each stream with an empty final stored
block, which every zlib reader takes. Go 1.27 ends the stream with no stored
block, and `iv` builds with Go 1.27. Nobody tested a stream from Go 1.26 in
Ghostty, so the test does not read the last 8 bytes for the marker.

`BenchmarkKittyZlib` on 2026-10-15, for `ZlibAuto` with the RGBA of each
image:

| Image | v0.1.19, a flush at each check | No flush |
| --- | --- | --- |
| `orca.png` | 329 KiB, 5.00 ms | 309 KiB, 4.74 ms |
| `jpg1.jpg` | 4378 KiB, 3.69 ms, raw | 4378 KiB, 3.67 ms, raw |
| `jpg2.jpg` | 3055 KiB, 1.16 ms, raw | 3055 KiB, 1.13 ms, raw |
| a flat 1024 x 1024 image | 46 KiB, 1.46 ms | 44 KiB, 1.46 ms |

Without the flush, a flat image is as small as with `ZlibAlways`, and a
photo still goes raw.

### D20. In tmux, Kitty and iTerm wrap each escape sequence in the DCS passthrough. Decided. Amended by D21. Amended by D22.

Date: 2026-10-20. Source: xo/usql issue 594.

On 2026-10-19, kcoyner reported that the logo and `\chart` of `usql` show
in Ghostty and iTerm2, and show nothing in tmux 3.8 with
`set -g allow-passthrough on`. tmux does not send the Kitty (APC) or the
iTerm (OSC 1337) sequences to the outer terminal. It sends a sequence only
in its DCS passthrough: `ESC P tmux;`, then the sequence with each ESC
doubled, then `ESC \`. Ken asked on 2026-10-20 for a clean implementation
in this package.

The source of tmux 3.8 (`input.c`) gives these rules:

- tmux writes the inside of the envelope to the outer terminal as it is,
  after it changes each doubled ESC to one ESC.
- tmux does this only when `allow-passthrough` is `on` or `all`. Its
  default is `off`.
- tmux drops a sequence longer than `input-buffer-size`, which is 1 MiB by
  default.
- tmux built with image support reads Sixel itself, and draws it in the
  pane.

The package follows them:

- `inTmux` is true when `$TMUX` is set. tmux sets it in each pane.
- In tmux, Kitty wraps the start sequence and each chunk in an envelope of
  its own. Each envelope holds about 4 KiB, so a large image fits under the
  limit of tmux.
- In tmux, iTerm wraps its one sequence. A file of more than about 786 KB
  gives a sequence over 1 MiB after base64, and tmux drops it unless
  `input-buffer-size` is larger.
- Sixel and half blocks are not wrapped. tmux draws Sixel itself, and
  answers the request for device attributes that finds Sixel (D3). Half
  blocks are text.
- The data in each sequence is base64 or digits, and holds no ESC. So only
  the start and the end of each sequence change, and the encoders build
  them with no extra copy. Each image is still one call to `Write` (D9).

`TestTmuxWrapsEachSequence` unwraps the output as tmux does, and compares it
with the output outside tmux. On 2026-10-20, tmux 3.8 ran a program that
wrote 4 Kitty sequences and 1 iTerm sequence. With `allow-passthrough on`,
all 5 reached the outer terminal unchanged and in order. With it off, none
did. An iTerm sequence of 937402 bytes reached the terminal, and one of
1071598 bytes did not. `BenchmarkTmux` measures the cost. The time was the
same within 2%, and Kitty sent 0.3% to 1.5% more bytes.

This does not find the outer terminal from tmux. In tmux, `TERM_PROGRAM` is
`tmux`, so the detection of Ghostty fails, and `TERM_GRAPHICS` must name the
protocol. Nested tmux needs an envelope for each level, and this writes one.
[BACKLOG.md](BACKLOG.md) holds the work that is left.

### D21. In tmux, Kitty and iTerm ask tmux for the terminal and for allow-passthrough. Proposed. Amends D3. Amends D20. Amended by D22.

Date: 2026-10-20.

On 2026-10-20, Ken ran `iv` with D20 in tmux, in Ghostty and in Kitty. It
printed "term graphics not available". In a tmux pane, `TERM_PROGRAM` is
`tmux` and `TERM` is `screen-256color` or `tmux-256color`, so no encoder
found its terminal (D3). With `TERM_GRAPHICS=kitty`, `iv` printed only the
file names, because `allow-passthrough` was off, and tmux dropped each
sequence. The variables of the environment do not help. Ken's tmux server
started in Ghostty, so each pane had the `GHOSTTY_` variables, also the
panes of the client in Kitty. A pane that Ken pasted also had `TMUX_PANE`
and no `TMUX`.

Ken did not choose an answer, so this decision is Proposed:

- The package is in tmux when `$TMUX` or `$TMUX_PANE` is set. D20 read
  `$TMUX` only.
- In tmux, the Kitty and the iTerm encoders run
  `tmux display-message -p -t $TMUX_PANE '#{client_termtype}|#{allow-passthrough}'`
  once, with a limit of one second. `client_termtype` is the name that the
  terminal gives tmux, such as `ghostty 1.3.1-arch2` or `kitty(0.49.2)`.
- Kitty is available for a client whose name starts with `kitty` or
  `ghostty`, and iTerm for `iterm2` or `wezterm`. Neither is available when
  `allow-passthrough` is off. The caller then gets
  `ErrTermGraphicsNotAvailable`, not an empty screen.
- In tmux, the encoders do not read the other variables, because they can
  belong to another terminal.
- `TERM_GRAPHICS` still selects a protocol, and then tmux is not asked.

This is the one place where the package starts a process. Outside tmux it
starts none.

`TestTmuxProtocol` and `TestTmuxDetection` check the rules. On 2026-10-20,
the query on Ken's tmux server returned `ghostty 1.3.1-arch2|off`. A program
in a tmux 3.8 pane, with an outer terminal that gave tmux the name
`kitty(0.49.2)`, found Kitty with `allow-passthrough on` and no protocol with
it off, with no `TERM_GRAPHICS`. `iv`, built with this package through a
`go.work`, ran in a tmux 3.8 pane. With `allow-passthrough on`, all 10 of its
Kitty sequences reached the outer terminal. With it off, none did.

### D22. In tmux, Kitty draws an image with Unicode placeholder cells. Decided. Amends D20. Amends D21.

Date: 2026-10-20.

On 2026-10-20, with D21 and `allow-passthrough on`, `iv` drew each image of
`testdata/png` in tmux in Ghostty, and each image was on top of the one
before it. Kitty draws an image at the cursor of the terminal and moves the
cursor below it. tmux does not know about the image, so its own cursor
stays on the line after the name, and it moves the cursor of the terminal
back before it writes the next name. tmux can also draw text over an image
when it draws the pane again.

Ken chose on 2026-10-20 to keep the rows of each image, with the Unicode
placeholders of the Kitty protocol, which kitty 0.28 and later and Ghostty
read. `kitten icat` and yazi draw images in tmux the same way. In tmux,
when tmux reports the cell size of its client:

1. The encoder sends the image once, with a virtual placement:
   `a=T,U=1,q=2,i=<id>,c=<columns>,r=<rows>`. The number of columns and rows
   is the size of the image divided by the size of a cell, rounded up. `q=2`
   is on each sequence, so that the terminal writes no response for tmux to
   read as input.
2. The encoder then writes a row of placeholder cells for each row of the
   image. A cell is `U+10EEEE`. The foreground color holds the low 24 bits
   of the id. The first cell of each row has three diacritics, for its row,
   for column 0 and for the high byte of the id. The other cells take them
   from the cell on their left.
3. tmux keeps the placeholder cells as text, so the rows of the image are
   kept, and the terminal draws the image in them again each time that tmux
   draws them.

The id is random, and its low 24 bits are not 0. Image ids are shared by
each program that writes to the terminal, so a new image seldom replaces one
that is on the screen. `kittyDiacritics` is
`gen/rowcolumn-diacritics.txt` of kitty v0.49.2, which has 297 characters.
An image of more than 297 rows of cells, or a client with no cell size, is
drawn at once as in D20. The query of D21 now also reads
`#{client_cell_width}` and `#{client_cell_height}`, in the same call.

`TestKittyPlaceholder` reads the output back as the terminal does: the keys
of the first sequence, `q=2` on each chunk, the pixels, and the id, row and
column of each row of cells. On 2026-10-20, `iv` drew `tux.png` and
`rose.png` in a tmux 3.8 pane, with an outer terminal that gave tmux the name
`kitty(0.49.2)` and cells of 10 x 20 pixels. Each image went out with its own
id and `c=10,r=5` and `c=10,r=4`, and tmux drew the placeholder cells.
`BenchmarkTmux` measures the cost. The time was the same within 5%, and the
placeholder cells added 2% to 13% to the bytes.

Ken drew each case of placeholder cells in Ghostty 1.3.1 in tmux on
2026-10-20: a small id, a 32-bit id, and zlib RGBA. Each was correct.

Ghostty 1.3.1 has a known fault with them. In tmux, after a clear of the
terminal, Ghostty can close. Ken found it with these steps in one Ghostty
window: `iv ./testdata/vips`, then `clear`, then `iv ./testdata/vips` again.
Just before Ghostty closed, it logged `missing image for virtual placement`
for two image ids of the package, once for each placeholder cell and each
frame, and it then stopped answering the Wayland compositor. Outside tmux,
`iv` does not make Ghostty close.

The fault is related to Ghostty discussion 5453, which Ken reported on
2025-01-31. In that report, Ghostty 1.1.0 closed on a clear or a
`tput reset` after it evicted Kitty images to make space. That fault is
fixed for images that are drawn at once. Placeholder cells use virtual
placements, which take a different path. The cause is not known yet, and no
report is filed. [BACKLOG.md](BACKLOG.md) holds the work.

## Open questions for Ken

An open question is not a decision. When Ken answers one, it becomes a
decision above, and the question goes.

1. D7 and D11 are Proposed. Is each one a rule?
2. dbmeta D110 names the `xo` repositories, and `rasterm` is
   `github.com/kenshaw/rasterm`. D8 records that you asked for the same
   setup here. Is that right?
3. `.golangci.yml` dates from 2024 and has the form of golangci-lint v1. It
   disables linters that v2 removed, such as `execinquery`, `exportloopref`
   and `gomnd`, and CI does not run it. Do you want the v2 configuration that
   the `xo` repositories use, and a lint step in CI?
4. D13 settles the shape of the interfaces. Must `rasterm` also export the
   wrapper structs `PNG`, `JPEG`, `RGB` and `RGBA`, or only the interfaces
   and `DecodePNG` and `DecodeJPEG`?
5. `TestEncode` writes each test image to the standard output of the test,
   which is the terminal that runs it. Do you want to keep that, or write to
   a buffer and keep the visual check behind a flag?
6. `go-pedantry` and the code disagree in two places. The skill wants a
   receiver named for its type, and the encoders use `r`. The skill wants
   sentinel errors from `errors.New`, and D6 uses constants. D6 wins for
   the errors. Which wins for the receivers?
