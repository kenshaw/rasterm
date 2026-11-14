<div align="center">
  <a href="#about" title="About">About</a> |
  <a href="#status" title="Status">Status</a> |
  <a href="#installing" title="Installing">Installing</a> |
  <a href="#using" title="Using">Using</a> |
  <a href="#kitty-animation" title="Kitty Animation">Animation</a> |
  <a href="#kitty-stream" title="Kitty Stream">Stream</a> |
  <a href="#tmux" title="tmux">tmux</a> |
  <a href="#performance" title="Performance">Performance</a> |
  <a href="#related-projects" title="Related Projects">Related Projects</a> |
  <a href="#documents" title="Documents">Documents</a> |
  <a href="https://pkg.go.dev/github.com/kenshaw/rasterm" title="Go Reference">Reference</a>
</div>

<br/>

[![Unit Tests][rasterm-ci-status]][rasterm-ci]
[![Go Reference][goref-rasterm-status]][goref-rasterm]
[![Releases][release-status]][releases]
[![Discord Discussion][discord-status]][discord]

[rasterm-ci]: https://github.com/kenshaw/rasterm/actions/workflows/test.yml "Test CI"
[rasterm-ci-status]: https://github.com/kenshaw/rasterm/actions/workflows/test.yml/badge.svg "Test CI"
[goref-rasterm]: https://pkg.go.dev/github.com/kenshaw/rasterm "Go Reference"
[goref-rasterm-status]: https://pkg.go.dev/badge/github.com/kenshaw/rasterm.svg "Go Reference"
[release-status]: https://img.shields.io/github/v/release/kenshaw/rasterm?display_name=tag&sort=semver "Latest Release"
[releases]: https://github.com/kenshaw/rasterm/releases "Releases"
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"

# About

`rasterm` draws a Go `image.Image` in a terminal. It writes the image with
the graphics protocol that the terminal supports:

- The Kitty graphics protocol, which kitty, Ghostty, WezTerm and Konsole
  support.
- The inline images of iTerm2, which iTerm2 and WezTerm support.
- Sixel, which foot, xterm, mlterm, Windows Terminal and others support.

It can also draw an image as text, with half blocks and 24-bit color, in a
terminal that has no graphics.

`rasterm` asks the terminal which protocols it supports, so detection works
over SSH and in tmux. It sends the bytes of a PNG or a JPEG as they are when
the terminal can read them, and builds each image in memory and writes it in
one call.

# Status

The API changed after v0.1.23. `Protocol` replaced `TermType`, and `Detect`
and `Terminal.Encoder` replaced `Available`, `Encode`, `DefaultEncoder` and
`PendingInput` (D26 and D27 in [docs/PLAN.md](docs/PLAN.md)). The change is
not released yet.

# Installing

```sh
go get github.com/kenshaw/rasterm@latest
```

# Using

Call `Detect` once, when the program starts, and keep the `Terminal` that it
returns. Then get an encoder for the terminal, and encode each image with
it:

```go
term, pending, err := rasterm.Detect(ctx)
if err != nil {
	return err
}
enc := term.Encoder()
if enc == nil {
	return rasterm.ErrTermGraphicsNotAvailable
}
f, err := os.Open("photo.jpg")
if err != nil {
	return err
}
defer f.Close()
img, _, err := image.Decode(f)
if err != nil {
	return err
}
return enc.Encode(os.Stdout, img)
```

`Detect` finds the protocol in this order:

1. If `TERM_GRAPHICS` names a protocol, `Detect` uses it, and asks the
   terminal nothing. The values are `kitty`, `iterm`, `sixel`, `halfblock`
   and `none`. Case, hyphens and a final `s` do not matter, so
   `half-blocks` also selects half blocks.
2. If the program runs in tmux, `Detect` asks tmux for the terminal of its
   client. See [tmux](#tmux).
3. If standard input and standard output are terminals, `Detect` asks the
   terminal. It sends all its queries in one write, and waits for the
   answers up to the timeout, which is 250 ms by default.
4. If the terminal cannot be asked, or does not answer, `Detect` reads the
   other variables of the environment, such as `TERM` and `TERM_PROGRAM`.

These options change what `Detect` reads:

| Option | Default |
| --- | --- |
| `WithInput(f)` | `os.Stdin` |
| `WithOutput(f)` | `os.Stdout` |
| `WithTimeout(d)` | `DefaultDetectTimeout`, 250 ms |
| `WithGetenv(getenv)` | `os.Getenv`. A program with its own value for `TERM_GRAPHICS` gives it here |

The context stops the wait. A terminal that does not answer is not an
error.

When the terminal supports more than one protocol, `Detect` selects Kitty,
then iTerm, then Sixel. It selects half blocks only when `TERM_GRAPHICS`
asks for them.

The `Terminal` also tells what `Detect` found: the name and the version of
the terminal, the size of a cell in pixels, each protocol that the terminal
supports, and whether it reported Kitty animation.

## Keys typed during the query

While `Detect` waits for the answers, it reads standard input. A key that the
user types in that time is not part of an answer, so `Detect` returns it in
`pending`, in the order that the user typed it. A program that reads standard
input, such as a line editor, must use `pending` as its first input. Call
`Detect` before the line editor starts.

## Writing to a file

A program that writes to a file, or that knows the protocol, does not need
`Detect`. It sets the protocol itself:

```go
enc := rasterm.Terminal{Protocol: rasterm.Kitty}.Encoder(rasterm.WithNoNewline())
```

`WithNoNewline` writes no newline after each image. `WithZlib` sets how Kitty
compresses raw pixels.

## Sending the bytes of an image as is

An encoder sends the bytes that an image already holds, without decoding and
encoding it again, when the image has one of these methods:

| Interface | Method | Wrapper | Encoders that send it as is |
| --- | --- | --- | --- |
| `PNGImage` | `PNG() []byte` | `rasterm.PNG`, `rasterm.DecodePNG` | Kitty, iTerm |
| `JPEGImage` | `JPEG() []byte` | `rasterm.JPEG`, `rasterm.DecodeJPEG` | iTerm |
| `RGBImage` | `RGB() []byte` | `rasterm.RGB` | Kitty |
| `RGBAImage` | `RGBA() []byte` | `rasterm.RGBA` | Kitty |

```go
buf, err := os.ReadFile("diagram.png")
if err != nil {
	return err
}
img, err := rasterm.DecodePNG(buf)
if err != nil {
	return err
}
// Kitty and iTerm send buf, and Sixel encodes img.Image
return enc.Encode(os.Stdout, img)
```

RGB and RGBA are packed 8-bit pixels, row by row, with no padding. RGBA alpha
is not premultiplied. If the program scales or crops the image, it must wrap
the new image with no bytes, or with new ones.

Raw pixels take less time to encode than a PNG, and the terminal must read
more bytes. Kitty compresses raw pixels of 64 KiB or more with zlib while
they compress to less than a quarter of their size. It checks after each
64 KiB, and sends the pixels raw at the first check that fails. So a flat
image, such as a chart, sends few bytes, and a photo stays raw. Set
`WithZlib(rasterm.ZlibAlways)` or `WithZlib(rasterm.ZlibNever)` to force the choice.

## Kitty animation

A terminal with the Kitty protocol can play an animation. The program sends
each frame once, as it decodes it, and the terminal keeps the time and loops:

```go
k, ok := term.Encoder().(rasterm.KittyEncoder)
if !ok {
	return rasterm.ErrTermGraphicsNotAvailable
}
anim, err := k.Animate(os.Stdout, frames[0], delays[0])
if err != nil {
	return err
}
for i := 1; i < len(frames); i++ {
	if _, err := anim.Frame(frames[i], rasterm.KittyFrame{Delay: delays[i]}); err != nil {
		return err
	}
}
// 0 loops forever
return anim.End(0)
```

The terminal plays each frame as it arrives, and waits at the last frame for
the next one, until `End`. A `KittyFrame` can also cover a part of the root
frame, at `At`, over a base frame or a background color. `Stop` holds the
frame that shows, and `Delete` removes the image and its frames.

Use it for a finite animation, such as a GIF. The terminal keeps every
frame, so draw a stream with no end, such as video, as one image for each
frame. kitty 0.20.0 and later and Ghostty 1.4.0 play animations.
`Terminal.KittyAnimation` is true when the terminal reports it, which kitty
0.50.0 and later do, so the program decides.

## Kitty stream

A stream shows pictures of one size as one Kitty image, at the times that the
program chooses. The terminal stores two frames however many pictures come,
so a stream suits an animation of any length, such as video:

```go
s, err := k.Stream(os.Stdout, frames[0])
if err != nil {
	return err
}
for _, frame := range frames[1:] {
	// wait until the frame is due, then draw it
	if err := s.Draw(frame); err != nil {
		return err
	}
}
```

`Draw` writes the picture into the frame that is not shown, and then shows
that frame, so the picture changes at once. Each picture must have the size
of the first one. `Delete` removes the image.

## Half blocks

The half-block encoder draws an image as text, with the `▀` character and
24-bit color. Each cell shows two pixels, one above the other. It does not
scale the image, so scale it to the size of the terminal first: one column
for each pixel across, and one row for each two pixels down. `Detect`
selects it only when `TERM_GRAPHICS=halfblock` is set.

# tmux

In a tmux pane, the Kitty and iTerm encoders wrap each escape sequence in the
DCS passthrough of tmux, so that tmux sends it to the terminal. tmux sends it
only when its `allow-passthrough` option is on:

```
set -g allow-passthrough on
```

In tmux, `Detect` does not ask the terminal, because tmux answers the
questions itself. It asks tmux for the terminal of its client, and finds
kitty, Ghostty, iTerm2 and WezTerm by name. tmux learns that name from the
terminal when the client attaches, over SSH too. When `allow-passthrough`
is off, `Detect` selects no protocol.

tmux does not know about an image that goes through it, so it can draw over
the image. In tmux, Kitty draws each image with Unicode placeholder cells,
which kitty 0.28 and later and Ghostty read. tmux keeps the cells as text,
so each image keeps its rows, and the terminal draws it again when tmux
draws the pane again. An iTerm image in tmux does not keep its rows.

tmux drops a sequence over its `input-buffer-size`, which is 1 MiB by
default, so an iTerm image file over about 786 KB does not show. Kitty sends
each chunk in a sequence of its own, so it has no such limit. tmux built
with image support draws Sixel itself, and half blocks are text, so neither
is wrapped.

Ghostty 1.3.1 can close after a clear of the terminal in tmux, when it shows
images that use placeholder cells. This is a fault in Ghostty, related to
[Ghostty discussion 5453](https://github.com/ghostty-org/ghostty/discussions/5453).

# Performance

The values below were measured on 2026-11-14, with Go 1.27.4 and `go-sixel`
v0.0.14, on an AMD Ryzen 9 9950X. Each value is the median of 6 runs. The
time is the time to encode in memory. It does not include the time that the
terminal takes to show the image. [testdata/README.md](testdata/README.md)
describes each image.

An image that holds its own bytes skips the encoding:

| Image | Encoder | Encoded | Sent as is | Output |
| --- | --- | --- | --- | --- |
| `flag.png` as a `PNG` | Kitty | 5.82 ms | 0.013 ms | 63 KB, then 45 KB |
| `flag.png` as a `PNG` | iTerm | 14.0 ms | 0.014 ms | 137 KB, then 45 KB |
| `photo.jpg` as a `JPEG` | iTerm | 14.6 ms | 0.082 ms | 376 KB, then 274 KB |
| `photo.jpg` as an `RGB` | Kitty | 44.0 ms | 1.84 ms | 1860 KB, then 4900 KB |
| `photo.jpg` as an `RGBA` | Kitty | 44.0 ms | 2.29 ms | 1860 KB, then 6534 KB |

A PNG or a JPEG that is sent as is takes 170 to 1000 times less time. Raw
pixels take 19 to 24 times less time, and the terminal must read 3 to 4
times the bytes. iTerm sends a JPEG for an image that is not paletted (D5),
so it encodes `flag.png` as a JPEG.

Kitty compresses raw RGBA pixels with zlib when they compress well. The size
is what the terminal reads, after base64:

| Image | Raw | Always zlib | Automatic |
| --- | --- | --- | --- |
| a flat 1024 x 1024 image | 5474 KiB, 1.74 ms | 44 KiB, 1.40 ms | 44 KiB, 1.45 ms |
| `flag.png` | 4503 KiB, 1.54 ms | 51 KiB, 1.77 ms | 51 KiB, 1.78 ms |
| `alpha-gradient.png` | 342 KiB, 0.10 ms | 146 KiB, 0.99 ms | 342 KiB, 0.35 ms |
| random noise, 1024 x 1024 | 5474 KiB, 1.71 ms | 5474 KiB, 2.72 ms | 5474 KiB, 1.76 ms |
| `photo.jpg` | 6381 KiB, 2.00 ms | 2890 KiB, 19.3 ms | 6381 KiB, 2.29 ms |
| `earth.jpg` | 8559 KiB, 2.64 ms | 5029 KiB, 30.0 ms | 8559 KiB, 3.51 ms |

The black border at the top of `earth.jpg` compresses well, so the automatic
choice compresses the first rows before it finds that the photo does not,
and then sends it raw.

When each encoder started to write the image in one call, on 2026-10-15, it
changed as follows. The values are geometric means over the test images of
that time, which are no longer in the repository.

| Encoder | Time | Memory allocated | Allocations | Calls to Write |
| --- | --- | --- | --- | --- |
| Kitty | 53% less | 91% less | 98% less | about 16 to 1 |
| iTerm | 26% less | 92% less | 57% less | 2 to 1 |
| Sixel | no change | no change | no change | 2 to 1 |

To measure on your own machine, run the benchmarks:

```sh
go test -run '^$' -bench . -benchtime 10x -count 6 . > new.txt
go run golang.org/x/perf/cmd/benchstat@latest new.txt
```

# Related projects

| Project | About |
| --- | --- |
| [`iv`][iv] [![Releases][iv-status]][iv-releases] | A command line image viewer. It draws each image and each frame of an animation with `rasterm`. |
| [`usql`][usql] [![Releases][usql-status]][usql-releases] | A universal command line client for SQL databases. It draws charts and images in the terminal with `rasterm`. |
| [`zuzu`][zuzu] [![Releases][zuzu-status]][zuzu-releases] | Charts and diagrams in pure Go, from a Vega spec, DOT or Mermaid. |
| [`okage`][okage] [![Releases][okage-status]][okage-releases] | A pure Go port of resvg, which renders SVG to an image. |

[iv]: https://github.com/kenshaw/iv "iv"
[iv-status]: https://img.shields.io/github/v/release/kenshaw/iv?display_name=tag&sort=semver "Latest Release"
[iv-releases]: https://github.com/kenshaw/iv/releases "Releases"
[usql]: https://github.com/xo/usql "usql"
[usql-status]: https://img.shields.io/github/v/release/xo/usql?display_name=tag "Latest Release"
[usql-releases]: https://github.com/xo/usql/releases "Releases"
[zuzu]: https://github.com/xo/zuzu "zuzu"
[zuzu-status]: https://img.shields.io/github/v/release/xo/zuzu?display_name=tag "Latest Release"
[zuzu-releases]: https://github.com/xo/zuzu/releases "Releases"
[okage]: https://github.com/xo/okage "okage"
[okage-status]: https://img.shields.io/github/v/release/xo/okage?display_name=tag "Latest Release"
[okage-releases]: https://github.com/xo/okage/releases "Releases"

# Documents

| Document | About |
| --- | --- |
| [docs/PLAN.md](docs/PLAN.md) | Each decision, and the open questions |
| [docs/BACKLOG.md](docs/BACKLOG.md) | Work that is known and not done |
| [docs/DETECTION.md](docs/DETECTION.md) | The design of detection by query, and of the API that replaced `Available` |
| [docs/KITTY_ANIMATION.md](docs/KITTY_ANIMATION.md) | The design of Kitty animation |
| [testdata/README.md](testdata/README.md) | Where each test image comes from, and its license |
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to change `rasterm` |

# History

`rasterm` began as a fork of
[`BourgeoisBear/rasterm`](https://github.com/BourgeoisBear/rasterm), by Jason
Stewart, which was rewritten in 2023 (D1) and has changed much since. The
copyright notice of that project stays in [LICENSE](LICENSE).
