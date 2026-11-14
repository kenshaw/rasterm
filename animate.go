package rasterm

import (
	"image"
	"image/color"
	"io"
	"math"
	"strconv"
	"time"
)

// KittyAnimation streams the frames of one finite animation, such as a GIF,
// to a terminal with the Kitty graphics protocol. The terminal stores the
// frames under one image id, and plays them with the delay of each frame
// (D25).
//
// It is not for a stream with no end, such as video, because the terminal
// keeps every frame, and nothing tells the program which frame it shows.
// Draw a stream as one image for each frame instead.
//
// Each method writes its output with one call to Write (D9). A
// KittyAnimation is not safe for concurrent use.
type KittyAnimation struct {
	w      io.Writer
	enc    KittyEncoder
	id     uint32
	size   image.Point
	frames int
	ended  bool
}

// KittyFrame describes how a frame of a [KittyAnimation] is drawn.
type KittyFrame struct {
	// Delay is how long the frame is shown. A Delay under 0 makes a frame
	// that is never shown, and that is a base for later frames. A Delay from
	// 0 to 1 ms is 1 ms, because the protocol reads 0 as its default of 40
	// ms.
	Delay time.Duration
	// At is where the top left of the image goes, relative to the top left
	// of the root frame. The image covers At to At plus its size, which
	// must be inside the root frame.
	At image.Point
	// Base is the frame that gives the pixels that the image does not cover,
	// numbered from 1, as [KittyAnimation.Frame] returns them. 0 is the frame
	// before. -1 is the Background color.
	Base int
	// Background is the color of the pixels that the image does not cover,
	// when Base is -1. nil is transparent.
	Background color.Color
	// Replace replaces the pixels under the image. By default, the image is
	// blended over them.
	Replace bool
}

// Animate sends first as the root frame of an animation, shown for delay,
// and places it at the cursor, or as placeholder cells in tmux. The terminal
// plays each frame as it arrives, and waits at the last frame for the next
// one, until [KittyAnimation.End].
//
// A terminal that does not support animation shows first, and ignores the
// frames. [Terminal.KittyAnimation] is true when the terminal reported that
// it supports animation, but kitty before 0.50.0 and Ghostty support it and
// do not report it, so the program decides.
func (r KittyEncoder) Animate(w io.Writer, first image.Image, delay time.Duration) (*KittyAnimation, error) {
	b := first.Bounds()
	a := &KittyAnimation{
		w:      w,
		enc:    r,
		id:     newKittyID(),
		size:   b.Size(),
		frames: 1,
	}
	// the root frame has a gap of 0 until it is set, and loading mode waits
	// at the last frame for the next one
	var after []byte
	after = appendKittyControl(after, r.Tmux, a.id, "r=1,z="+strconv.Itoa(kittyGap(delay)))
	after = appendKittyControl(after, r.Tmux, a.id, "s=2")
	k := kittyImage{action: 'T', id: a.id, after: after}
	if err := r.encode(w, first, k, r.Tmux && r.Cell.X > 0 && r.Cell.Y > 0); err != nil {
		return nil, err
	}
	return a, nil
}

// Frame adds img as the next frame, and returns its number. The root frame
// is 1.
//
// If img is a [PNGImage], an [RGBImage] or an [RGBAImage], its bytes are
// sent as they are, as [KittyEncoder.Encode] does.
func (a *KittyAnimation) Frame(img image.Image, f KittyFrame) (int, error) {
	if a.ended {
		return 0, ErrAnimationEnded
	}
	size := img.Bounds().Size()
	if r := (image.Rectangle{Min: f.At, Max: f.At.Add(size)}); !r.In(image.Rectangle{Max: a.size}) || r.Empty() {
		return 0, ErrFrameOutOfBounds
	}
	k := kittyImage{
		action:  'f',
		id:      a.id,
		x:       f.At.X,
		y:       f.At.Y,
		replace: f.Replace,
		gap:     kittyGap(f.Delay),
	}
	switch {
	case f.Base == 0:
		k.base = a.frames
	case f.Base == -1:
		if f.Background != nil {
			k.bg, k.hasBg = kittyColor(f.Background), true
		}
	case f.Base < -1 || f.Base > a.frames:
		return 0, ErrUnknownFrame
	default:
		k.base = f.Base
	}
	enc := a.enc
	enc.NoNewline = true
	if err := enc.encode(a.w, img, k, false); err != nil {
		return 0, err
	}
	a.frames++
	return a.frames, nil
}

// End tells the terminal that no more frames come, and plays the frames
// loops times. 0 plays them forever.
func (a *KittyAnimation) End(loops int) error {
	if loops < 0 || loops >= math.MaxInt32 {
		return ErrInvalidLoops
	}
	a.ended = true
	// v=1 loops forever, and v=n plays the frames n-1 times
	return a.write(appendKittyControl(nil, a.enc.Tmux, a.id, "s=3,v="+strconv.Itoa(loops+1)))
}

// Stop stops the animation on the frame that it shows. Frames can still be
// added, and [KittyAnimation.End] plays the animation again.
func (a *KittyAnimation) Stop() error {
	return a.write(appendKittyControl(nil, a.enc.Tmux, a.id, "s=1"))
}

// Delete deletes the image and all of its frames from the terminal. No frame
// can be added after it.
func (a *KittyAnimation) Delete() error {
	a.ended = true
	return a.write(appendKittyDelete(nil, a.enc.Tmux, a.id))
}

// ID returns the image id of the animation.
func (a *KittyAnimation) ID() uint32 {
	return a.id
}

// Frames returns the number of frames, with the root frame.
func (a *KittyAnimation) Frames() int {
	return a.frames
}

// appendKittyControl appends a control of the animation of the image id,
// a=a with keys, to buf, in the passthrough of tmux when tmux is true.
func appendKittyControl(buf []byte, tmux bool, id uint32, keys string) []byte {
	start, end := kittySeq(tmux)
	buf = append(buf, start...)
	buf = append(buf, "a=a,q=2,i="...)
	buf = strconv.AppendUint(buf, uint64(id), 10)
	buf = append(buf, ',')
	buf = append(buf, keys...)
	return append(buf, end...)
}

// appendKittyDelete appends the deletion of the image id and its frames to
// buf, in the passthrough of tmux when tmux is true.
func appendKittyDelete(buf []byte, tmux bool, id uint32) []byte {
	start, end := kittySeq(tmux)
	buf = append(buf, start...)
	buf = append(buf, "a=d,d=I,q=2,i="...)
	buf = strconv.AppendUint(buf, uint64(id), 10)
	return append(buf, end...)
}

// write writes buf to the writer of the animation in one call.
func (a *KittyAnimation) write(buf []byte) error {
	_, err := a.w.Write(buf)
	return err
}

// kittyGap returns the gap in ms for a frame shown for d. A negative d is a
// gapless frame, -1. Any other d is at least 1 ms, because the protocol reads
// 0 as its default.
func kittyGap(d time.Duration) int {
	if d < 0 {
		return -1
	}
	return int(min(max(d/time.Millisecond, 1), math.MaxInt32))
}

// kittyColor returns c as the protocol writes a color, 0xRRGGBBAA, not
// premultiplied.
func kittyColor(c color.Color) uint32 {
	n := color.NRGBAModel.Convert(c).(color.NRGBA)
	return uint32(n.R)<<24 | uint32(n.G)<<16 | uint32(n.B)<<8 | uint32(n.A)
}
