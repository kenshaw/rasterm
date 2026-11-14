package rasterm

import (
	"bytes"
	"errors"
	"image"
	"strconv"
	"strings"
	"testing"
)

func TestStream(t *testing.T) {
	var buf bytes.Buffer
	s, err := KittyEncoder{}.Stream(&buf, flatImage(40, 30))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	id := strconv.FormatUint(uint64(s.ID()), 10)
	keys, _ := kittySeqs(buf.String())
	if keys[0]["a"] != "T" || keys[0]["i"] != id || keys[0]["q"] != "2" {
		t.Fatalf("expected frame 1 with a=T, i=%s and q=2, got: %v", id, keys[0])
	}
	// the animation is stopped, so that the terminal does not play it
	if c := keys[len(keys)-1]; c["a"] != "a" || c["s"] != "1" || c["i"] != id {
		t.Errorf("expected the animation to stop, got: %v", c)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("expected a newline after the image")
	}
	// the first draw adds frame 2, and each later draw edits the frame that
	// is not shown, and then shows it
	for i, exp := range []struct{ edit, show string }{{"", "2"}, {"1", "1"}, {"2", "2"}, {"1", "1"}} {
		buf.Reset()
		w := countWriter{w: &buf}
		s.w = &w
		img := RGBA{Image: flatImage(40, 30), Data: packRGBA(flatImage(40, 30))}
		if err := s.Draw(img); err != nil {
			t.Fatalf("draw %d: expected no error, got: %v", i, err)
		}
		if w.writes != 1 {
			t.Errorf("draw %d: expected 1 write, got: %d", i, w.writes)
		}
		keys, _ := kittySeqs(buf.String())
		f := keys[0]
		if f["a"] != "f" || f["i"] != id || f["X"] != "1" || f["r"] != exp.edit || f["f"] != "32" || f["s"] != "40" || f["v"] != "30" {
			t.Errorf("draw %d: expected a whole frame that replaces frame %q, got: %v", i, exp.edit, f)
		}
		for _, k := range []string{"c", "x", "y", "z"} {
			if _, ok := f[k]; ok {
				t.Errorf("draw %d: expected no %s, got: %v", i, k, f)
			}
		}
		if c := keys[len(keys)-1]; c["a"] != "a" || c["c"] != exp.show || c["i"] != id {
			t.Errorf("draw %d: expected to show frame %s, got: %v", i, exp.show, c)
		}
		if strings.Contains(buf.String(), "\n") {
			t.Errorf("draw %d: expected no newline", i)
		}
	}
	if err := s.Draw(flatImage(40, 31)); !errors.Is(err, ErrFrameSize) {
		t.Errorf("expected %v, got: %v", ErrFrameSize, err)
	}
	buf.Reset()
	s.w = &buf
	if err := s.Delete(); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if keys, _ := kittySeqs(buf.String()); len(keys) != 1 || keys[0]["a"] != "d" || keys[0]["d"] != "I" || keys[0]["i"] != id {
		t.Errorf("expected the deletion of the image, got: %v", keys)
	}
	if err := s.Draw(flatImage(40, 30)); !errors.Is(err, ErrAnimationEnded) {
		t.Errorf("expected %v, got: %v", ErrAnimationEnded, err)
	}
}

func TestStreamTmux(t *testing.T) {
	var buf bytes.Buffer
	s, err := KittyEncoder{Tmux: true, Cell: image.Pt(10, 20)}.Stream(&buf, flatImage(40, 40))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	seqs, rest := unwrapTmux(t, buf.String())
	keys, _ := kittySeqs(strings.Join(seqs, ""))
	if keys[0]["U"] != "1" || keys[0]["i"] != strconv.FormatUint(uint64(s.ID()), 10) {
		t.Errorf("expected a virtual placement with the id of the stream, got: %v", keys[0])
	}
	if rows := readPlaceholder(t, strings.TrimSuffix(rest, "\n")); len(rows) != 2 {
		t.Errorf("expected 2 rows of placeholder cells, got: %d", len(rows))
	}
	buf.Reset()
	if err := s.Draw(flatImage(40, 40)); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if _, rest := unwrapTmux(t, buf.String()); rest != "" {
		t.Errorf("expected each sequence of a draw in the passthrough, got: %q", rest)
	}
}

func BenchmarkStreamDraw(b *testing.B) {
	for _, f := range testfiles(b) {
		b.Run(f.name, func(b *testing.B) {
			var w countWriter
			s, err := KittyEncoder{}.Stream(&w, f.img)
			if err != nil {
				b.Fatalf("expected no error, got: %v", err)
			}
			w = countWriter{}
			b.ReportAllocs()
			for b.Loop() {
				if err := s.Draw(f.img); err != nil {
					b.Fatalf("expected no error, got: %v", err)
				}
			}
			b.ReportMetric(float64(w.writes)/float64(b.N), "writes/op")
			b.ReportMetric(float64(w.n)/float64(b.N), "bytes/op")
		})
	}
}
