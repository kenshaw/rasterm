package rasterm

import (
	"bytes"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"testing"
)

func TestAvailable(t *testing.T) {
	for _, typ := range []TermType{Kitty, ITerm, Sixel, Default} {
		t.Logf("type: % 7s env: % 6s available: %t", typ, typ.EnvValue(), typ.Available())
	}
}

func TestEncode(t *testing.T) {
	out := os.Stdout
	for _, tt := range testfiles(t) {
		test := tt
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

func testfiles(t *testing.T) []testfile {
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
		f, err := dir.Open(name)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		buf, err := io.ReadAll(f)
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
		if err := f.Close(); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	}
	return files
}
