# Draft: streaming animation for Kitty in rasterm

Status: the design record of D25, revision 2, after critiques by Gemini 3.1
Pro, GPT-6 Astra (OpenAI Pro) and DeepSeek V4 Pro on 2026-11-13. A
maintainer accepted the revised draft on 2026-11-14, and `animate.go`
follows it, with the choices of D28. Open questions 6, 7 and 8 in
[PLAN.md](PLAN.md) ask the maintainers the questions at the end of this
document. The sections before the revised draft are kept as they were
written, so they describe the code before the change.

## Context

`rasterm` (github.com/kenshaw/rasterm) encodes a Go `image.Image` as terminal
graphics. `KittyEncoder.Encode(w, img)` sends one image with `a=T`, as PNG
(`f=100`), or raw RGB/RGBA (`f=24`/`f=32`, zlib `o=z` when it pays). It writes
each image with exactly one `Write` call on a pooled buffer. In tmux it wraps
each escape sequence in the DCS passthrough, and places the image with Unicode
placeholder cells (`U=1`) and a random 32-bit image id.

The main user is `iv`, an image viewer. Today `iv` plays an animation (GIF,
APNG, WebP, video sampled by ffmpeg, Lottie) by drawing each frame as a new
image: cursor home, `a=d` to delete all images, `Encode` the frame, sleep for
the delay, repeat. A producer goroutine decodes and scales frames up to 8
ahead. For each loop, every frame is decoded, scaled and sent again. `a=d` is
not wrapped for tmux, so tmux drops it.

The Kitty graphics protocol has animation since kitty 0.20.0, and frame
composition since kitty 0.22.0. Ghostty has both since 1.4.0:

- `a=f,i=ID` adds a frame (or edits frame `r=N`). `x,y,s,v` give the
  rectangle the data covers. `c=N` names the base frame for the rest of the
  pixels, and `Y` a background color. `X=1` overwrites instead of blending.
  `z=ms` is the gap after the frame, 40 ms by default. A negative `z` makes a
  gapless frame that is not shown and can be a base.
- `a=a,i=ID`: `s=1` stop, `s=2` run in loading mode (at the last frame, wait
  for more frames instead of looping), `s=3` run and loop. `v=0` ignored,
  `v=1` loop forever, `v=n` loop n-1 times. `c=N` shows frame N. `r=N,z=ms`
  sets the gap of frame N. The root frame has gap 0 until it is set.
- `a=c` composes a rectangle of one frame onto another.
- `a=d,d=f|F` deletes frames (the spec does not say which).
- kitty stores extra frames on disk, with a quota of 5 x 320 MB, and replies
  `ENOSPC` when it is full.

## Goal

Send each frame once, as it is decoded, and let the terminal keep the time
and loop. `iv` gets smooth playback that does not depend on its own sleeps, no
re-encoding for each loop, and no full-screen delete between frames.

## Draft API

```go
// KittyAnimation streams the frames of one animation to a Kitty terminal.
// The terminal stores the frames under one image id, and plays them with
// the gap of each frame. It is not safe for concurrent use. Each method
// writes its output with one call to Write (D9).
type KittyAnimation struct {
	w      io.Writer
	enc    KittyEncoder    // Zlib and NoNewline of the encoder that made it
	id     uint32          // random, low 24 bits not 0, as for placeholders
	bounds image.Rectangle // bounds of the root frame, in pixels
	frames int             // frames sent, the root frame is 1
	tmux   bool
	ended  bool
}

// Animate sends first as the root frame of an animation, draws it at the
// cursor (or as placeholder cells in tmux), and starts the animation in
// loading mode: when the terminal reaches the last frame, it waits for
// the next one. delay is how long first is shown.
func (r KittyEncoder) Animate(w io.Writer, first image.Image, delay time.Duration) (*KittyAnimation, error)

// Frame adds img as the next frame, shown for delay.
//
// When img.Bounds() is the bounds of the root frame, img replaces the
// whole frame. When it is a smaller rectangle inside them, only that
// rectangle is sent, and the other pixels come from the frame before
// (c=previous). img in any other place is an error.
//
// A delay under 1 ms is 1 ms, because z=0 means the default of 40 ms. A
// negative delay makes a gapless frame, which is never shown, and which
// later frames can use as a base.
//
// PNGImage, RGBImage and RGBAImage are sent as they are, as Encode does.
func (a *KittyAnimation) Frame(img image.Image, delay time.Duration) error

// End tells the terminal that no more frames come. It plays the frames
// loops times, or loops forever when loops is 0 (s=3, v=loops+1).
func (a *KittyAnimation) End(loops int) error

// Stop stops the animation on the frame that it shows (s=1).
func (a *KittyAnimation) Stop() error

// Delete deletes the image and all of its frames from the terminal
// (a=d,d=I,i=id), in tmux too.
func (a *KittyAnimation) Delete() error

// ID returns the image id of the animation.
func (a *KittyAnimation) ID() uint32
```

The wire output, for a 3-frame stream outside tmux:

```
Animate:  ESC_Ga=T,i=ID,q=2,f=..,s=W,v=H[,o=z],m=1;<data>ESC\ ...chunks...
          ESC_Ga=a,i=ID,q=2,r=1,z=GAP1ESC\
          ESC_Ga=a,i=ID,q=2,s=2ESC\
Frame:    ESC_Ga=f,i=ID,q=2,f=..,z=GAP2,[x=,y=,s=,v=,c=1,]m=1;<data>ESC\ ...
Frame:    ESC_Ga=f,i=ID,q=2,f=..,z=GAP3,m=1;<data>ESC\ ...
End(0):   ESC_Ga=a,i=ID,q=2,s=3,v=1ESC\
Delete:   ESC_Ga=d,d=I,i=IDESC\
```

How `iv` would use it: on the first pass, the producer feeds frames to
`Frame` as it makes them; at the end of the pass it calls `End(0)` and stops
producing. The terminal loops. When the user quits, or `iv` reaches its time
limit, it calls `Stop` and leaves the last frame, or `Delete`.

## Open questions

1. Memory for long streams. A video sampled for minutes, or a stream with no
   end, grows without bound in the terminal (up to 1.6 GB in kitty, then
   ENOSPC, which `q=2` hides). Options: a ring of N frames that are edited in
   place with `r=N` (but the play head can reach a frame while it is being
   replaced); delete old frames with `a=d,d=f` (semantics unclear); or let
   the caller fall back to one image per frame for long streams.
2. Pacing. In loading mode the terminal holds the last frame until the next
   arrives, so a slow producer stalls the animation, and a fast producer
   fills terminal memory far ahead. Should `Frame` know where the play head
   is? The protocol gives no position report.
3. Detection. `KittyEncoder.Available` does not tell kitty 0.19 or Ghostty
   1.3 (no animation) from kitty 0.20+ or Ghostty 1.4. kitty 0.50.0 answers a
   feature query (`a=q,e=a`), but older ones and Ghostty may not. Reading a
   reply means the stdin query of D23/D24. Should rasterm query, or leave it
   to the caller (an env variable, a flag)?
4. Partial frames. Is "a sub-rectangle of the root bounds means a partial
   frame based on the previous frame" clear enough, or should the base
   frame, blend mode (X=1) and background be explicit? GIF disposal methods
   (restore to background, restore to previous) map to c= and Y=.
5. Shape. Methods on a type returned by `KittyEncoder.Animate`, versus an
   `Animator` interface that other encoders could satisfy (iTerm plays a GIF
   file; Sixel and half blocks would draw frame by frame with sleeps), versus
   a function that takes an `iter.Seq2[image.Image, time.Duration]`.
6. Errors. With `q=2` the terminal reports nothing. Without it, replies come
   on stdin, which the program also reads.
7. Writes. Each method writes once (D9). A frame of a 4K RGBA stream is
   33 MB raw. Is one Write per frame right, or should Frame stream chunks?
8. tmux. Placeholder cells point at the image id, so animation should work
   through them, but this is not tested. tmux drops a sequence over its
   1 MiB input buffer, so each chunk stays wrapped alone.

## Critiques, 2026-11-13

All three models answered at low reasoning effort, because the deep requests
timed out. Each claim about the protocol was checked against the spec at
https://sw.kovidgoyal.net/kitty/graphics-protocol/.

Points that two or more models made, and that hold:

- Do not use a stored animation for a stream with no end, such as video.
  The terminal keeps every frame, and nothing tells the program which frames
  were shown. Video stays one image for each frame, as `iv` does now.
  (Gemini, OpenAI. DeepSeek said to delete old frames.)
- Make partial frames explicit: the rectangle, the base frame, overwrite or
  blend, and the background. Bounds alone cannot say GIF "restore to
  previous". (All three.)
- Write a frame in chunks, not as one 44 MB buffer for a 4K frame. (All
  three.) tmux is not the reason, because each chunk is wrapped alone
  already. The reason is memory: the frame is held twice, raw and base64.
  This conflicts with D9, so the maintainers decide it.

Points that one model made, and that hold:

- The doc of Frame is wrong about the order: "under 1 ms is 1 ms" also
  catches a negative delay. Negative must be tested first. (OpenAI)
- An image from SubImage keeps its bounds, which do not start at 0,0, and
  the root frame may not start at 0,0 either. Offsets must be relative to
  the root bounds. (OpenAI)
- Check loops for a negative value and for overflow before `v=loops+1`.
  (OpenAI)
- Detection must not read standard input inside an encode call. A separate
  query, as for Sixel (D23, D24), or a choice of the caller. (OpenAI)
- Send the root frame with `a=t`, then `a=a,s=2`, then place it, so that
  the first frame does not show before its gap is set. (Gemini, flicker not
  verified)

Claims that are wrong, by the spec:

- DeepSeek: `a=f` is not an action, `v=0` is infinite, negative `z` is not
  gapless, `s=2` loads a frame. The spec says the opposite of each.
- Gemini: `d=I` deletes by cursor. `d=i` and `d=I` delete by id. `d=c`
  and `d=C` use the cursor.
- Gemini: `z=0` may be a gap of 0. The spec says `z=0` is ignored.

## Revised draft

```go
// KittyAnimation streams the frames of one finite animation, such as a GIF,
// to a Kitty terminal, which stores them under one image id and plays them.
// It is not for a stream with no end, such as video, because the terminal
// keeps every frame. It is not safe for concurrent use.
type KittyAnimation struct{ /* unexported */ }

// KittyFrame describes how a frame is drawn over the frame before it.
type KittyFrame struct {
	// Delay is how long the frame is shown. A Delay under 0 makes a frame
	// that is never shown, and that is a base for later frames. A Delay from
	// 0 to 1 ms is 1 ms.
	Delay time.Duration
	// At is where the image goes, relative to the top left of the root frame.
	// The image covers At to At plus its size.
	At image.Point
	// Base is the frame whose pixels fill the rest of the frame, numbered from
	// 1. 0 is the frame before. -1 is the Background color.
	Base int
	// Background is the color of the pixels that no frame gives, when Base is
	// -1.
	Background color.Color
	// Replace replaces the pixels under the image. By default, the image is
	// blended over them.
	Replace bool
}

// Animate sends first as the root frame, shown for delay, and places it at
// the cursor, or as placeholder cells in tmux. The terminal plays each frame
// as it arrives, and waits at the last one for the next one.
func (r KittyEncoder) Animate(w io.Writer, first image.Image, delay time.Duration) (*KittyAnimation, error)

// Frame adds img as the next frame, and returns its number.
func (a *KittyAnimation) Frame(img image.Image, f KittyFrame) (int, error)

// End tells the terminal that no more frames come, and plays them loops
// times. 0 plays them forever.
func (a *KittyAnimation) End(loops int) error

// Stop stops the animation on the frame that it shows.
func (a *KittyAnimation) Stop() error

// Delete deletes the image and all of its frames from the terminal.
func (a *KittyAnimation) Delete() error

// ID returns the image id.
func (a *KittyAnimation) ID() uint32
```

Questions for the maintainers, which are open questions 6, 7 and 8 in PLAN.md:

1. One Write for each frame (D9), or one Write for each chunk of a frame?
2. Does rasterm add a query for animation support (`a=q,e=a`, kitty 0.50.0
   and up), or does the caller choose?
3. Must rasterm also have an `Animator` that the other encoders satisfy,
   frame by frame, or does `iv` keep its own loop for them?
