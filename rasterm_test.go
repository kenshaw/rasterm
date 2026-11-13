package rasterm

import (
	"bytes"
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"testing"
)

// TestEncode writes each test image to the terminal that runs the test, with
// the protocol that Detect selects. It fails when Detect selects none, so set
// TERM_GRAPHICS, as CI does.
func TestEncode(t *testing.T) {
	term, _, err := Detect(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	t.Logf("protocol: %s source: %s name: %q version: %q", term.Protocol, term.Source, term.Name, term.Version)
	enc := term.Encoder()
	if enc == nil {
		t.Fatalf("expected an encoder, got: %v", ErrTermGraphicsNotAvailable)
	}
	for _, test := range testfiles(t) {
		t.Run(test.name, func(t *testing.T) {
			if err := enc.Encode(os.Stdout, test.img); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

// testEncoder returns the encoder for p, out of tmux.
func testEncoder(p Protocol) Encoder {
	return Terminal{Protocol: p}.Encoder()
}

// tmuxEncoder returns the encoder for p in tmux, with cells of the size cell.
func tmuxEncoder(p Protocol, cell image.Point) Encoder {
	return Terminal{Protocol: p, Tmux: true, Cell: cell}.Encoder()
}

// testdataReadme is the file in testdata/ that records where each image
// comes from. It is the only file there that is not an image.
const testdataReadme = "README.md"

type testfile struct {
	name string
	typ  string
	buf  []byte
	img  image.Image
}

func testfiles(t testing.TB) []testfile {
	t.Helper()
	dir := os.DirFS("testdata")
	var names []string
	err := fs.WalkDir(dir, ".", func(name string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir(), name == testdataReadme:
			return nil
		}
		names = append(names, name)
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	var files []testfile
	for _, name := range names {
		buf, err := fs.ReadFile(dir, name)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		img, typ, err := image.Decode(bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		files = append(files, testfile{
			name: name,
			typ:  typ,
			buf:  buf,
			img:  img,
		})
	}
	return files
}

func TestParseProtocol(t *testing.T) {
	tests := []struct {
		s   string
		exp Protocol
		ok  bool
	}{
		{"halfblock", Halfblock, true},
		{"halfblocks", Halfblock, true},
		{"half-block", Halfblock, true},
		{"half-blocks", Halfblock, true},
		{"Half_Blocks", Halfblock, true},
		{" half block ", Halfblock, true},
		{"kitty", Kitty, true},
		{"KITTY", Kitty, true},
		{"iterm", ITerm, true},
		{"iTerm2", ITerm, true},
		{"sixel", Sixel, true},
		{"sixels", Sixel, true},
		{"none", None, true},
		// no value, or a value that names no protocol, selects nothing
		{"", None, false},
		{"-", None, false},
		{"unknown", None, false},
	}
	for _, test := range tests {
		t.Run(test.s, func(t *testing.T) {
			p, ok := ParseProtocol(test.s)
			if p != test.exp || ok != test.ok {
				t.Errorf("expected %v %t, got: %v %t", test.exp, test.ok, p, ok)
			}
		})
	}
}

func TestProtocolText(t *testing.T) {
	for _, p := range []Protocol{None, Kitty, ITerm, Sixel, Halfblock} {
		buf, err := p.MarshalText()
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		var got Protocol
		if err := got.UnmarshalText(buf); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if got != p {
			t.Errorf("expected %v, got: %v", p, got)
		}
	}
	var p Protocol
	if err := p.UnmarshalText([]byte("unknown")); err != ErrUnknownProtocol {
		t.Errorf("expected %v, got: %v", ErrUnknownProtocol, err)
	}
	if _, err := Protocol(99).MarshalText(); err != ErrUnknownProtocol {
		t.Errorf("expected %v, got: %v", ErrUnknownProtocol, err)
	}
}

func TestTerminalEncoder(t *testing.T) {
	cell := image.Pt(10, 20)
	tests := []struct {
		t   Terminal
		exp Encoder
	}{
		{Terminal{}, nil},
		{Terminal{Protocol: Kitty}, KittyEncoder{}},
		{Terminal{Protocol: Kitty, Tmux: true, Cell: cell}, KittyEncoder{Tmux: true, Cell: cell}},
		{Terminal{Protocol: ITerm, Tmux: true, Cell: cell}, ITermEncoder{Tmux: true}},
		{Terminal{Protocol: Sixel}, SixelEncoder{}},
		{Terminal{Protocol: Halfblock}, HalfblockEncoder{}},
	}
	for _, test := range tests {
		if got := test.t.Encoder(); got != test.exp {
			t.Errorf("%v: expected %#v, got: %#v", test.t.Protocol, test.exp, got)
		}
	}
	got := Terminal{Protocol: Kitty}.Encoder(WithNoNewline(), WithZlib(ZlibNever))
	if exp := (KittyEncoder{NoNewline: true, Zlib: ZlibNever}); got != exp {
		t.Errorf("expected %#v, got: %#v", exp, got)
	}
}
