# Test images

This document records where each image in this folder comes from, its
license, and its SHA-256 checksum. `TestEachTestImageHasAProvenance` fails
when an image here has no row, or when its checksum does not match the row.

The tests encode every image here with every encoder. Each image is
here because it reaches a different path in the code. A new image must
reach a path that no other image reaches.

## The images

| File | Size | What it tests | Source | License | SHA-256 |
| --- | --- | --- | --- | --- | --- |
| `photo.jpg` | 1280 x 955, baseline JPEG, Exif | A photo that does not compress. Kitty sends its raw pixels without zlib, and iTerm sends its bytes as they are | "Pavo Real Venezolano" by Wilfredor, [Wikimedia Commons][peacock], the 1280 px thumbnail of Commons | [CC0 1.0][cc0] | `a96f9c96ea1cb0996b05848e760ccbbbd1e74142369c2299d294b0b5e8d4208c` |
| `photo-progressive.jpg` | 1280 x 955, progressive JPEG | A progressive JPEG with the same pixels as `photo.jpg` | `photo.jpg`, made with `jpegtran -copy none -progressive` | [CC0 1.0][cc0] | `33852721790750bc7b6f4f0b577eec493449166d7d46cfec82348840f6f2ef42` |
| `earth.jpg` | 1280 x 1281, baseline JPEG | A photo with a black border. The first rows compress well and the rest do not, so `ZlibAuto` stops after the first check | "The Blue Marble", NASA, the Apollo 17 crew, 1972, [Wikimedia Commons][earth], the 1280 px thumbnail of Commons | Public domain, a work of NASA | `3f55578a3c58509029c6a1ec88c683199f585ea9ffe974efd4265aec314156fb` |
| `flag.png` | 1280 x 674, 8-bit RGBA PNG | A large flat image that compresses well, as a chart or a diagram does | "Flag of the United States", [Wikimedia Commons][flag], the 1280 px PNG that Commons renders from the SVG | Public domain | `56709ec59faf103b118b38cc673b0cb7bc38404052e0a8843c65a35885df39f8` |
| `cmyk.jpeg` | 150 x 103, CMYK JPEG | A JPEG with 4 components, which Go decodes to `*image.CMYK` | `src/image/testdata/video-001.cmyk.jpeg` of Go 1.27.4 | [BSD-3-Clause](#the-license-of-go) | `323ac0779011f0e5581af8622898d7300d65746061292718ccb6c00685e5e0b5` |
| `gray.jpeg` | 150 x 103, grayscale JPEG | A JPEG with 1 component, which Go decodes to `*image.Gray` | `src/image/testdata/video-005.gray.jpeg` of Go 1.27.4 | [BSD-3-Clause](#the-license-of-go) | `29e745eabbc435987127ae43f2e4af059f844d017224bf54f02d750073cd912e` |
| `paletted.gif` | 150 x 103, GIF | A GIF, which Go decodes to `*image.Paletted` | `src/image/testdata/video-001.gif` of Go 1.27.4 | [BSD-3-Clause](#the-license-of-go) | `13c7f6698a4e4f38b60da55c8cad135d431b369ff0bc0a99df295012d70a9429` |
| `alpha-gradient.png` | 256 x 256, 8-bit RGBA PNG | Alpha that changes across the image | `src/image/png/testdata/benchNRGBA-gradient.png` of Go 1.27.4 | [BSD-3-Clause](#the-license-of-go) | `0cd1dd8c81f134eab1a520e6f63810184e00733c2ffad098bcb231e3e7b61513` |
| `interlaced.png` | 256 x 256, 8-bit RGB PNG, interlaced | An interlaced PNG | `src/image/png/testdata/benchRGB-interlace.png` of Go 1.27.4 | [BSD-3-Clause](#the-license-of-go) | `5f48775450a0242ff66d880ec5646d3d628e0d29d75354159ef3bd9c8748524e` |
| `paletted.png` | 256 x 256, 8-bit paletted PNG | A paletted PNG, which iTerm sends as a PNG (D5) | `src/image/png/testdata/benchPaletted.png` of Go 1.27.4 | [BSD-3-Clause](#the-license-of-go) | `646534562b08ca76094d55fd0cfb4a57fc4ec0d565e85e40b8e13c4edd91b331` |
| `gray.png` | 256 x 256, 8-bit grayscale PNG | A grayscale PNG | `src/image/png/testdata/benchGray.png` of Go 1.27.4 | [BSD-3-Clause](#the-license-of-go) | `3370d2f3a41337b5dd393b3a1c5bce53f8532309011076c6e7508b1d5b177bfc` |
| `rgba16.png` | 32 x 32, 16-bit RGBA PNG | 16 bits for each channel, which Go decodes to `*image.NRGBA64` | `basn6a16.png` of PngSuite, from `src/image/png/testdata/pngsuite/` of Go 1.27.4 | [PngSuite](#the-license-of-pngsuite) | `569040d3237a5552935a44b8bbe165cf02afe0d71caf30fba81955922ac9373f` |
| `gray16.png` | 32 x 32, 16-bit grayscale PNG | 16-bit gray, which Go decodes to `*image.Gray16` | `basn0g16.png` of PngSuite, from the same folder | [PngSuite](#the-license-of-pngsuite) | `f26eb27e99300829c42c0832acd26fa499e0f317cb4d6f165e6663024b16bf6c` |
| `paletted-alpha.png` | 32 x 32, 8-bit paletted PNG with tRNS | A palette with transparent colors | `basn3p08-trns.png` from the same folder. The Go authors made it from `basn3p08.png` of PngSuite, in Go commit `737e96c7f3` (2011) | [PngSuite](#the-license-of-pngsuite) and [BSD-3-Clause](#the-license-of-go) | `fdeb6ef7e80ebebd2b92d2fdb6855073dce06b7d5e1d27012532dd738cfaa595` |
| `odd-size.png` | 31 x 31, 4-bit paletted PNG, interlaced | An odd width and height. Half blocks draw the last row with one pixel | `basn3p04-31i.png` from the same folder. The Go authors made it from `basn3p04.png` of PngSuite, in Go commit `5c2f01f392` (2014) | [PngSuite](#the-license-of-pngsuite) and [BSD-3-Clause](#the-license-of-go) | `172c656eb846f86beed905aca19b12777733aa5bb80f78db55ffd465ed0350d4` |

[peacock]: https://commons.wikimedia.org/wiki/File:Pavo_Real_Venezolano.jpg
[earth]: https://commons.wikimedia.org/wiki/File:The_Earth_seen_from_Apollo_17.jpg
[flag]: https://commons.wikimedia.org/wiki/File:Flag_of_the_United_States.svg
[cc0]: https://creativecommons.org/publicdomain/zero/1.0/

The three images from Wikimedia Commons were downloaded on 2026-11-14. The
license of each was read from the API of Commons on the same day.

## The license of Go

The images from the Go source tree have the license of Go:

```
Copyright 2009 The Go Authors.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

## The license of PngSuite

PngSuite is by Willem van Schaik. Go copies it from libpng 1.6.26, and
`src/image/png/testdata/pngsuite/README` of Go gives its license:

```
Permission to use, copy, and distribute these images for any purpose
and without fee is hereby granted.
```
