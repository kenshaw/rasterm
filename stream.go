package rasterm

import (
	"image"
	"io"
	"strconv"
)

// KittyStream shows a sequence of pictures of one size as one Kitty image,
// with the terminal storing two frames however many pictures come. The
// program keeps the time, and calls [KittyStream.Draw] when the next picture
// is due. It suits an animation of any length, such as video (D29).
//
// The animation of the image stays stopped, so the terminal never changes
// the picture by itself. Draw writes the picture into the frame that is not
// shown, and then shows that frame, so the picture changes at once and does
// not flicker.
//
// Each method writes its output with one call to Write (D9). A KittyStream
// is not safe for concurrent use.
type KittyStream struct {
	w      io.Writer
	enc    KittyEncoder
	id     uint32
	size   image.Point
	frames int
	shown  int
	ended  bool
}

// Stream sends first as frame 1 of a Kitty image, and places it at the
// cursor, or as placeholder cells in tmux. It stops the animation of the
// image, so that the terminal does not play it.
//
// The image of each later [KittyStream.Draw] must have the size of first.
// [PNGImage], [RGBImage] and [RGBAImage] are sent as they are, as
// [KittyEncoder.Encode] does.
func (r KittyEncoder) Stream(w io.Writer, first image.Image) (*KittyStream, error) {
	s := &KittyStream{
		w:      w,
		enc:    r,
		id:     newKittyID(),
		size:   first.Bounds().Size(),
		frames: 1,
		shown:  1,
	}
	k := kittyImage{action: 'T', id: s.id, after: appendKittyControl(nil, r.Tmux, s.id, "s=1")}
	if err := r.encode(w, first, k, r.Tmux && r.Cell.X > 0 && r.Cell.Y > 0); err != nil {
		return nil, err
	}
	return s, nil
}

// Draw shows img in place of the picture that shows. It writes img into the
// frame that is not shown, and then shows that frame, in one call to Write.
// It returns [ErrFrameSize] when img does not have the size of the first
// image.
func (s *KittyStream) Draw(img image.Image) error {
	if s.ended {
		return ErrAnimationEnded
	}
	if img.Bounds().Size() != s.size {
		return ErrFrameSize
	}
	// the first Draw adds frame 2, and each later one edits the frame that
	// is not shown
	next := 3 - s.shown
	k := kittyImage{action: 'f', id: s.id, replace: true}
	if next <= s.frames {
		k.edit = next
	}
	k.after = appendKittyControl(nil, s.enc.Tmux, s.id, "c="+strconv.Itoa(next))
	enc := s.enc
	enc.NoNewline = true
	if err := enc.encode(s.w, img, k, false); err != nil {
		return err
	}
	s.frames, s.shown = max(s.frames, next), next
	return nil
}

// Delete deletes the image and its frames from the terminal. No picture can
// be drawn after it.
func (s *KittyStream) Delete() error {
	s.ended = true
	_, err := s.w.Write(appendKittyDelete(nil, s.enc.Tmux, s.id))
	return err
}

// ID returns the image id of the stream.
func (s *KittyStream) ID() uint32 {
	return s.id
}
