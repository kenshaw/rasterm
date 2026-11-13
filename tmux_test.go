package rasterm

import (
	"bytes"
	"image"
	"strings"
	"testing"
)

func TestTmuxWrapsEachSequence(t *testing.T) {
	flat := flatImage(256, 256)
	inputs := []struct {
		name string
		img  image.Image
	}{
		{"rgba zlib", RGBA{Image: flat, Data: packRGBA(flat)}},
		{"png as is", PNG{Image: flat, Data: mustPNG(t, flat)}},
	}
	for _, f := range testfiles(t) {
		inputs = append(inputs, struct {
			name string
			img  image.Image
		}{f.name, f.img})
	}
	for _, typ := range []Protocol{Kitty, ITerm} {
		for _, in := range inputs {
			t.Run(typ.String()+"/"+in.name, func(t *testing.T) {
				want := encodeIn(t, typ, in.img, false)
				got := encodeIn(t, typ, in.img, true)
				seqs, rest := unwrapTmux(t, got)
				// tmux writes the inside of each envelope, and the newline
				// is outside them, so the terminal gets the same bytes
				if s := strings.Join(seqs, "") + rest; s != want {
					t.Fatalf("expected the unwrapped output to be the output outside tmux")
				}
				if rest != "\n" {
					t.Errorf("expected only the newline outside the envelopes, got: %q", rest)
				}
				for i, seq := range seqs {
					// each envelope holds one escape sequence
					if n := strings.Count(seq, "\x1b"); typ == Kitty && n != 2 || typ == ITerm && n != 1 {
						t.Fatalf("envelope %d: expected one escape sequence, got %d ESC", i, n)
					}
				}
				if typ == Kitty && len(seqs) < 2 {
					t.Errorf("expected an envelope for the start and each chunk, got %d", len(seqs))
				}
				if typ == ITerm && len(seqs) != 1 {
					t.Errorf("expected one envelope, got %d", len(seqs))
				}
			})
		}
	}
}

func TestTmuxKittyFitsTheInputLimit(t *testing.T) {
	// tmux drops a sequence longer than its input-buffer-size, which is 1 MiB
	// by default. Each kitty chunk is wrapped on its own, so a large image
	// fits
	img := noiseImage(1024, 1024)
	got := encodeIn(t, Kitty, RGBA{Image: img, Data: packRGBA(img)}, true)
	seqs, _ := unwrapTmux(t, got)
	for i, seq := range seqs {
		if n := len("tmux;") + len(seq) + strings.Count(seq, "\x1b"); n > 8<<10 {
			t.Fatalf("envelope %d: expected at most 8 KiB, got: %d", i, n)
		}
	}
}

func TestTmuxDoesNotWrapTextOrSixel(t *testing.T) {
	// tmux draws sixel itself when it is built with image support, and half
	// blocks are text
	for _, f := range testfiles(t) {
		for _, typ := range []Protocol{Sixel, Halfblock} {
			if want, got := encodeIn(t, typ, f.img, false), encodeIn(t, typ, f.img, true); got != want {
				t.Errorf("%s/%s: expected the same output in tmux", typ, f.name)
			}
		}
	}
}

func TestTmuxWritesOnce(t *testing.T) {
	for _, f := range testfiles(t) {
		for _, typ := range []Protocol{Kitty, ITerm} {
			var w countWriter
			if err := tmuxEncoder(typ, image.Point{}).Encode(&w, f.img); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if w.writes != 1 {
				t.Errorf("%s/%s: expected 1 write, got: %d", typ, f.name, w.writes)
			}
		}
	}
}

// encodeIn returns the output of the encoder for typ, in tmux when tmux is
// true.
func encodeIn(t *testing.T, typ Protocol, img image.Image, tmux bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := (Terminal{Protocol: typ, Tmux: tmux}).Encoder().Encode(&buf, img); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return buf.String()
}

// unwrapTmux returns what tmux writes to the outer terminal for each DCS
// passthrough envelope in s, and the bytes of s outside the envelopes. As
// tmux does, it changes each doubled ESC to one ESC, and ends an envelope at
// ESC \. It fails on an ESC inside an envelope that is not doubled and does
// not end it.
func unwrapTmux(t *testing.T, s string) ([]string, string) {
	t.Helper()
	var seqs []string
	var rest strings.Builder
	for len(s) != 0 {
		i := strings.Index(s, tmuxStart)
		if i < 0 {
			rest.WriteString(s)
			break
		}
		rest.WriteString(s[:i])
		s = s[i+len(tmuxStart):]
		var seq strings.Builder
		for {
			j := strings.IndexByte(s, 0x1b)
			if j < 0 || j+1 == len(s) {
				t.Fatalf("expected the envelope to end with ESC \\")
			}
			seq.WriteString(s[:j])
			c := s[j+1]
			s = s[j+2:]
			if c == '\\' {
				break
			}
			if c != 0x1b {
				t.Fatalf("expected each ESC in an envelope to be doubled, got ESC %q", c)
			}
			seq.WriteByte(0x1b)
		}
		seqs = append(seqs, seq.String())
	}
	return seqs, rest.String()
}

// BenchmarkTmux encodes each test file with Kitty and iTerm, out of tmux, in
// tmux, and in tmux with a cell size, where Kitty draws placeholder cells.
func BenchmarkTmux(b *testing.B) {
	files := testfiles(b)
	modes := []struct {
		name string
		t    Terminal
	}{
		{"plain", Terminal{}},
		{"tmux", Terminal{Tmux: true}},
		// with a cell size, kitty draws with placeholder cells (D22)
		{"placeholder", Terminal{Tmux: true, Cell: image.Pt(10, 20)}},
	}
	for _, m := range modes {
		for _, typ := range []Protocol{Kitty, ITerm} {
			for _, f := range files {
				b.Run(m.name+"/"+typ.String()+"/"+f.name, func(b *testing.B) {
					term := m.t
					term.Protocol = typ
					benchEncode(b, term.Encoder(), f.img)
				})
			}
		}
	}
}

func TestParseTmuxClient(t *testing.T) {
	tests := []struct {
		line string
		exp  tmuxClient
		ok   bool
	}{
		{"ghostty 1.3.1-arch2|on|24|44", tmuxClient{"ghostty", "1.3.1-arch2", true, image.Pt(24, 44)}, true},
		{"kitty(0.49.2)|on|10|20", tmuxClient{"kitty", "0.49.2", true, image.Pt(10, 20)}, true},
		{"kitty(0.49.2)|all|10|20", tmuxClient{"kitty", "0.49.2", true, image.Pt(10, 20)}, true},
		{"iTerm2 3.5.14|on|9|18", tmuxClient{"iTerm2", "3.5.14", true, image.Pt(9, 18)}, true},
		{"WezTerm 20240203-110809-5046fc22|on|8|16", tmuxClient{"WezTerm", "20240203-110809-5046fc22", true, image.Pt(8, 16)}, true},
		// tmux drops each sequence when passthrough is off
		{"ghostty 1.3.1-arch2|off|24|44", tmuxClient{"ghostty", "1.3.1-arch2", false, image.Pt(24, 44)}, true},
		// no client, or no cell size
		{"|on||", tmuxClient{passthrough: true}, true},
		{"kitty(0.49.2)|on", tmuxClient{}, false},
		{"", tmuxClient{}, false},
	}
	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			got, ok := parseTmuxClient(test.line)
			if got != test.exp || ok != test.ok {
				t.Errorf("expected %+v %t, got: %+v %t", test.exp, test.ok, got, ok)
			}
		})
	}
}
