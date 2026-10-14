# rasterm

Package `rasterm` provides a simple way to encode images as terminal graphics,
supporting Kitty, iTerm, and Sixel.

[![Unit Tests][rasterm-ci-status]][rasterm-ci]
[![Go Reference][goref-rasterm-status]][goref-rasterm]
[![Releases][release-status]][Releases]
[![Discord Discussion][discord-status]][discord]

Fork/rewrite of [`rasterm`][original-rasterm].

![rasterm sample output](screenshot.png)

[rasterm-ci]: https://github.com/kenshaw/rasterm/actions/workflows/test.yml "Test CI"
[rasterm-ci-status]: https://github.com/kenshaw/rasterm/actions/workflows/test.yml/badge.svg "Test CI"
[goref-rasterm]: https://pkg.go.dev/github.com/kenshaw/rasterm "Go Reference"
[goref-rasterm-status]: https://pkg.go.dev/badge/github.com/kenshaw/rasterm.svg "Go Reference"
[release-status]: https://img.shields.io/github/v/release/kenshaw/rasterm?display_name=tag&sort=semver "Latest Release"
[discord]: https://discord.gg/WDWAgXwJqN "Discord Discussion"
[discord-status]: https://img.shields.io/discord/829150509658013727.svg?label=Discord&logo=Discord&colorB=7289da&style=flat-square "Discord Discussion"
[releases]: https://github.com/kenshaw/rasterm/releases "Releases"
[original-rasterm]: https://github.com/BourgeoisBear/rasterm

## Using

Install in the usual Go fashion:

```sh
$ go get -u github.com/kenshaw/rasterm@latest
```

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
buf, err := os.ReadFile("image.png")
if err != nil {
	return err
}
img, err := rasterm.DecodePNG(buf)
if err != nil {
	return err
}
// kitty and iterm send buf, and sixel encodes img.Image
if err := rasterm.Encode(os.Stdout, img); err != nil {
	return err
}
```

RGB and RGBA are packed 8-bit pixels, row by row, with no padding, and RGBA
alpha is not premultiplied. Raw pixels take less time to encode than a PNG,
and the terminal must read more bytes. If the image is scaled or cropped,
wrap the new image with no bytes, or with new ones.

## Half blocks

`rasterm.Halfblock` draws an image as text, with the `▀` character and
24-bit color. Each cell shows two pixels, one above the other. It does not
scale the image, so scale it to the size of the terminal first: one column
for each pixel across, and one row for each two pixels down. It is available
only when `TERM_GRAPHICS=halfblock` is set, and the default encoder then uses
it in place of any graphics protocol.

`TERM_GRAPHICS` selects one protocol: `kitty`, `iterm`, `sixel` or
`halfblock`. Each other protocol is then not available. `TERM_GRAPHICS=none`
turns off each of them. Case, hyphens and a final `s` do not matter, so
`half-blocks` and `HalfBlock` also select half blocks.

## Performance

Each encoder builds its output in memory and writes it in one call. Kitty
and iTerm also send the bytes that an image already holds, without encoding
it again. The tables below compare the code at `eacef4b` with this release.
Both ran with the same dependencies, Go 1.27.2 and `go-sixel` v0.0.14, on an
AMD Ryzen 9 9950X, on 2026-10-15. Each value is the median of 6 runs, and
`benchstat` compared the two. The time is the time to encode in memory. It
does not include the time that the terminal takes to show the image.

Over the 9 images in `testdata/`, each encoder changed as follows. Each
value is a geometric mean over the images.

| Encoder | Time | Memory allocated | Allocations | Calls to Write |
| --- | --- | --- | --- | --- |
| Kitty | 53% less | 91% less | 98% less | about 16 to 1 |
| iTerm | 26% less | 92% less | 57% less | 2 to 1 |
| Sixel | no change | no change | no change | 2 to 1 |

Kitty gains the most on a JPEG. `testdata/jpg1.jpg` took 62.9 ms with
839096 allocations, and now takes 25.7 ms with 9. Kitty sends a PNG, and the
encoder now converts the decoded JPEG to RGBA before PNG encoding. Sixel
spends its time in the color quantizer of `go-sixel`.

An image that holds its own bytes skips the encoding:

| Image | Encoder | Encoded | Sent as is | Output |
| --- | --- | --- | --- | --- |
| `orca.png` as a `PNG` | Kitty | 14.4 ms | 0.08 ms | 233 KB, then 247 KB |
| `png1.png` as a `PNG` | Kitty | 0.96 ms | 0.005 ms | 34 KB, then 14 KB |
| `jpg1.jpg` as a `JPEG` | iTerm | 8.26 ms | 0.04 ms | 171 KB, then 115 KB |
| `jpg1.jpg` as an `RGB` | Kitty | 25.7 ms | 1.10 ms | 992 KB, then 3362 KB |
| `jpg1.jpg` as an `RGBA` | Kitty | 25.7 ms | 1.51 ms | 992 KB, then 4483 KB |

A PNG or a JPEG that is sent as is takes 180 to 220 times less time. Raw
pixels take 17 to 23 times less time, and the terminal must read 3 to 5
times the bytes.

To measure on your own machine, run the benchmarks:

```sh
go test -run '^$' -bench . -benchtime 10x -count 6 . > new.txt
go run golang.org/x/perf/cmd/benchstat@latest new.txt
```
