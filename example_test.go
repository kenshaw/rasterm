package rasterm_test

import (
	"context"
	"image"
	"os"

	_ "image/png"

	"github.com/kenshaw/rasterm"
)

func Example() {
	term, _, err := rasterm.Detect(context.Background())
	if err != nil {
		panic(err)
	}
	enc := term.Encoder()
	if enc == nil {
		panic(rasterm.ErrTermGraphicsNotAvailable)
	}
	f, err := os.Open("/path/to/image.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		panic(err)
	}
	if err := enc.Encode(os.Stdout, img); err != nil {
		panic(err)
	}
}
