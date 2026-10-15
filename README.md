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
alpha is not premultiplied. Raw pixels take less time to encode than a PNG.
Kitty compresses raw pixels of 64 KiB or more with zlib while they compress
to less than a quarter of their size. It checks after each 64 KiB, and sends
the pixels raw at the first check that fails. So a flat or rendered image
sends few bytes, and a photo stays raw. Set `KittyEncoder.Zlib` to
`ZlibAlways` or `ZlibNever` to force the choice. If the image is scaled or cropped,
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

Kitty compresses raw pixels with zlib when they compress well. The size is
what the terminal reads, after base64:

| Image | Raw | Always zlib | Automatic |
| --- | --- | --- | --- |
| a flat 1024 x 1024 image | 5473 KiB, 1.66 ms | 44 KiB, 1.44 ms | 46 KiB, 1.46 ms |
| `orca.png` | 6938 KiB, 2.12 ms | 309 KiB, 4.69 ms | 329 KiB, 5.00 ms |
| `png3.png` | 1336 KiB, 0.46 ms | 78 KiB, 1.13 ms | 80 KiB, 1.08 ms |
| random noise, 1024 x 1024 | 5473 KiB, 1.68 ms | 5474 KiB, 2.68 ms | 5473 KiB, 1.71 ms |
| `jpg2.jpg` | 3055 KiB, 0.97 ms | 1371 KiB, 9.64 ms | 3055 KiB, 1.16 ms |
| `jpg1.jpg` | 4378 KiB, 1.35 ms | 1543 KiB, 10.99 ms | 4378 KiB, 3.69 ms |

The top of `jpg1.jpg` compresses well, so the automatic choice compresses
the first rows before it finds that the photo does not, and then sends it
raw.

To measure on your own machine, run the benchmarks:

```sh
go test -run '^$' -bench . -benchtime 10x -count 6 . > new.txt
go run golang.org/x/perf/cmd/benchstat@latest new.txt
```
