package rasterm_test

import (
	"image"
	"os"

	_ "image/png"

	"github.com/kenshaw/rasterm"
)

func Example() {
	f, err := os.OpenFile("/path/to/image.png", os.O_RDONLY, 0)
	if err != nil {
		panic(err)
	}
	img, _, err := image.Decode(f)
	if err != nil {
		panic(err)
	}
	if err := rasterm.Encode(os.Stdout, img); err != nil {
		panic(err)
	}
}
