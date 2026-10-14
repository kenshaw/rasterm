# Backlog

This document lists work that is known and not done. Each item names where
it came from.

A decision is not a backlog item. It goes in [PLAN.md](PLAN.md). When an
item here is done, delete it, and record in `PLAN.md` anything that was
decided on the way (D8).

On 2026-10-15, the code had no `TODO` or `FIXME` comment. The items below
come from the sources that each one names.

## Encoders

### Send a Kitty image through a file or shared memory

Source: the review by Gemini 3.1 Pro on 2026-10-15 (D10).

The Kitty protocol can read the image from a file (`t=f`), a temporary file
(`t=t`) or shared memory (`t=s`), and not from base64 in the escape
sequence. The encoder then sends a path, and the terminal reads the bytes
directly. This works only when the terminal runs on the same host as the
program, and not over SSH. The encoder must find that out before it uses it.

### Send an image.NRGBA to Kitty as raw pixels

Source: found while writing D14, on 2026-10-15.

An `*image.NRGBA` whose bounds start at the origin holds its pixels in the
form of `RGBAImage`. Kitty can send its `Pix` with `f=32`, with no wrapper
and no PNG. That takes less time and sends more bytes (D14), so it changes
what each caller gets. Ask Ken before the default changes.

### Find out what Kitty does with an animated PNG

Source: the review by Gemini 3.1 Pro on 2026-10-15 (D10).

The review said that Kitty plays an animated PNG that it gets with `f=100`.
This was not tested. If Kitty plays it, `iv` can send the file once and not
each frame.

### Make Sixel faster

Source: `BenchmarkEncode`, on 2026-10-15.

Sixel takes 68 ms for `testdata/jpg1.jpg`, and Kitty takes 26 ms. Most of
the time is in the median cut quantizer of `go-sixel`. The output is written
in one call already (D9).

## Users

### Send the PNG of an image in iv

Source: Ken, on 2026-10-15 (D10).

`iv` decodes a PNG file with `image.Decode`, and the bytes of the file are
lost. To use D10, `iv` must keep the bytes and wrap the image with
`rasterm.PNG` when it does not scale or change the image. The change is in
`iv`, not here.
