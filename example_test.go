package rasterm_test

import (
	"image"
	"os"

	_ "image/png"

	"github.com/kenshaw/rasterm"
)

func Example() {
	f, err := os.Open("/path/to/image.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		panic(err)
	}
	if err := rasterm.Encode(os.Stdout, img); err != nil {
		panic(err)
	}
}
