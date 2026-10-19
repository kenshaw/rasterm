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
	for _, typ := range []TermType{Kitty, ITerm} {
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
		for _, typ := range []TermType{Sixel, Halfblock} {
			if want, got := encodeIn(t, typ, f.img, false), encodeIn(t, typ, f.img, true); got != want {
				t.Errorf("%s/%s: expected the same output in tmux", typ, f.name)
			}
		}
	}
}

func TestTmuxWritesOnce(t *testing.T) {
	withTmux(t, true)
	for _, f := range testfiles(t) {
		for _, typ := range []TermType{Kitty, ITerm} {
			var w countWriter
			if err := encoders[typ].Encode(&w, f.img); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if w.writes != 1 {
				t.Errorf("%s/%s: expected 1 write, got: %d", typ, f.name, w.writes)
			}
		}
	}
}

// withTmux makes [inTmux] return v, and [tmuxClientOnce] return a client
// with no protocol and no cell size, until the test ends, so that no test
// asks the tmux of the machine.
func withTmux(t testing.TB, v bool) {
	t.Helper()
	withTmuxClient(t, tmuxClient{})
	prev := inTmux
	inTmux = func() bool { return v }
	t.Cleanup(func() { inTmux = prev })
}

// withTmuxClient makes [tmuxClientOnce] return c until the test ends.
func withTmuxClient(t testing.TB, c tmuxClient) {
	t.Helper()
	prev := tmuxClientOnce
	tmuxClientOnce = func() tmuxClient { return c }
	t.Cleanup(func() { tmuxClientOnce = prev })
}

// encodeIn returns the output of the encoder for typ, in tmux when tmux is
// true.
func encodeIn(t *testing.T, typ TermType, img image.Image, tmux bool) string {
	t.Helper()
	withTmux(t, tmux)
	var buf bytes.Buffer
	if err := encoders[typ].Encode(&buf, img); err != nil {
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
		tmux bool
		c    tmuxClient
	}{
		{"plain", false, tmuxClient{}},
		{"tmux", true, tmuxClient{}},
		// with a cell size, kitty draws with placeholder cells (D22)
		{"placeholder", true, tmuxClient{protocol: "kitty", cellWidth: 10, cellHeight: 20}},
	}
	for _, m := range modes {
		for _, typ := range []TermType{Kitty, ITerm} {
			for _, f := range files {
				b.Run(m.name+"/"+typ.String()+"/"+f.name, func(b *testing.B) {
					withTmux(b, m.tmux)
					withTmuxClient(b, m.c)
					benchEncode(b, encoders[typ], f.img)
				})
			}
		}
	}
}

func TestParseTmuxClient(t *testing.T) {
	tests := []struct {
		line string
		exp  tmuxClient
	}{
		{"ghostty 1.3.1-arch2|on|24|44", tmuxClient{"kitty", 24, 44}},
		{"kitty(0.49.2)|on|10|20", tmuxClient{"kitty", 10, 20}},
		{"kitty(0.49.2)|all|10|20", tmuxClient{"kitty", 10, 20}},
		{"iTerm2 3.5.14|on|9|18", tmuxClient{"iterm", 9, 18}},
		{"WezTerm 20240203-110809-5046fc22|on|8|16", tmuxClient{"iterm", 8, 16}},
		// tmux drops each sequence when passthrough is off
		{"ghostty 1.3.1-arch2|off|24|44", tmuxClient{"", 24, 44}},
		// a terminal that is not known, no client, or no cell size
		{"foot(1.21.0)|on|10|20", tmuxClient{"", 10, 20}},
		{"|on||", tmuxClient{}},
		{"kitty(0.49.2)|on", tmuxClient{}},
		{"", tmuxClient{}},
	}
	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			if got := parseTmuxClient(test.line); got != test.exp {
				t.Errorf("expected %+v, got: %+v", test.exp, got)
			}
		})
	}
}

func TestTmuxDetection(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		tg       string
		kitty    bool
		iterm    bool
	}{
		{"kitty client", "kitty", "", true, false},
		{"iterm client", "iterm", "", false, true},
		// passthrough off, or a client that is not known
		{"no protocol", "", "", false, false},
		// TERM_GRAPHICS names a protocol, and tmux is not asked
		{"forced kitty", "", "kitty", true, false},
		{"forced iterm", "kitty", "iterm", false, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withTmux(t, true)
			withTmuxClient(t, tmuxClient{protocol: test.protocol})
			prevTG := termGraphics
			termGraphics = func() string { return test.tg }
			t.Cleanup(func() { termGraphics = prevTG })
			if got := (KittyEncoder{}).Available(); got != test.kitty {
				t.Errorf("expected kitty available %t, got: %t", test.kitty, got)
			}
			if got := (ITermEncoder{}).Available(); got != test.iterm {
				t.Errorf("expected iterm available %t, got: %t", test.iterm, got)
			}
		})
	}
}
