package rasterm

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"os"
	"slices"
	"testing"
)

func TestAvailable(t *testing.T) {
	for _, typ := range []TermType{None, Kitty, ITerm, Sixel, Halfblock, Default} {
		t.Logf("type: % 7s env: % 6s available: %t", typ, typ.EnvValue(), typ.Available())
	}
}

func TestEncode(t *testing.T) {
	out := os.Stdout
	for _, test := range testfiles(t) {
		t.Run(test.name, func(t *testing.T) {
			if err := Default.Encode(out, test.img); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

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
		case d.IsDir():
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

func TestOtherProtocol(t *testing.T) {
	protocols := []string{"kitty", "iterm", "sixel", "halfblock"}
	tests := []struct {
		v   string
		exp []string
	}{
		// a named protocol turns off each of the others
		{"kitty", []string{"iterm", "sixel", "halfblock"}},
		{"iterm", []string{"kitty", "sixel", "halfblock"}},
		{"sixel", []string{"kitty", "iterm", "halfblock"}},
		{"halfblock", []string{"kitty", "iterm", "sixel"}},
		// none turns off each of them
		{"none", protocols},
		// no value, or a value that names no protocol, turns off none
		{"", nil},
		{"unknown", nil},
	}
	for _, test := range tests {
		t.Run(test.v, func(t *testing.T) {
			for _, typ := range protocols {
				exp := slices.Contains(test.exp, typ)
				if got := otherProtocol(test.v, typ); got != exp {
					t.Errorf("TERM_GRAPHICS=%s: expected otherProtocol(%q) to be %t, got: %t", test.v, typ, exp, got)
				}
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		s   string
		exp TermType
	}{
		{"halfblock", Halfblock},
		{"halfblocks", Halfblock},
		{"half-block", Halfblock},
		{"half-blocks", Halfblock},
		{"Half_Blocks", Halfblock},
		{" half block ", Halfblock},
		{"kitty", Kitty},
		{"KITTY", Kitty},
		{"iterm", ITerm},
		{"iTerm2", ITerm},
		{"sixel", Sixel},
		{"sixels", Sixel},
		{"none", None},
		{"", Default},
		{"-", Default},
		{"unknown", None},
	}
	for _, test := range tests {
		t.Run(test.s, func(t *testing.T) {
			var typ TermType
			if err := typ.UnmarshalText([]byte(test.s)); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if typ != test.exp {
				t.Errorf("expected %v, got: %v", test.exp, typ)
			}
			// the environment is read the same way
			if v, exp := normalize(test.s), test.exp.EnvValue(); test.exp != None && v != exp {
				t.Errorf("expected normalize to return %q, got: %q", exp, v)
			}
		})
	}
}
