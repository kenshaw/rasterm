package rasterm

import (
	"image/color"
	"math/rand/v2"
	"unicode/utf8"
)

// kittyPlaceholder is the character of a Kitty Unicode placeholder cell. The
// terminal draws a part of an image in each cell that holds it (D22).
const kittyPlaceholder = 0x10EEEE

// kittyCells returns the number of columns and rows of cells that an image of
// w x h pixels covers, in cells of cw x ch pixels. It returns false when the
// cell size is not known, or when the image needs more rows than the
// diacritics can number.
func kittyCells(w, h, cw, ch int) (int, int, bool) {
	if w <= 0 || h <= 0 || cw <= 0 || ch <= 0 {
		return 0, 0, false
	}
	cols, rows := (w+cw-1)/cw, (h+ch-1)/ch
	return cols, rows, rows <= len(kittyDiacritics)
}

// newKittyID returns a random image id for an image drawn with placeholders.
// Image ids are shared by each program that writes to the terminal, so a
// random id seldom replaces the image of another program. The low 24 bits,
// which the foreground color carries, are not 0.
func newKittyID() uint32 {
	for {
		if id := rand.Uint32(); id&0xffffff != 0 {
			return id
		}
	}
}

// appendKittyPlaceholder appends rows of cols placeholder cells for the image
// id to buf. The foreground color holds the low 24 bits of the id. The first
// cell of each row has the diacritics of its row, of column 0 and of the high
// byte of the id, and each other cell takes them from the cell on its left.
// Each row but the last ends with a newline.
func appendKittyPlaceholder(buf []byte, id uint32, cols, rows int) []byte {
	fg := color.NRGBA{R: uint8(id >> 16), G: uint8(id >> 8), B: uint8(id)}
	high := kittyDiacritics[id>>24]
	for y := range rows {
		if y != 0 {
			buf = append(buf, '\n')
		}
		buf = appendRGB(append(buf, fgColor...), fg)
		buf = utf8.AppendRune(buf, kittyPlaceholder)
		buf = utf8.AppendRune(buf, kittyDiacritics[y])
		buf = utf8.AppendRune(buf, kittyDiacritics[0])
		buf = utf8.AppendRune(buf, high)
		for range cols - 1 {
			buf = utf8.AppendRune(buf, kittyPlaceholder)
		}
		buf = append(buf, "\x1b[39m"...)
	}
	return buf
}

// kittyPlaceholderLen returns the length of the output of
// [appendKittyPlaceholder] for rows of cols cells, at most.
func kittyPlaceholderLen(cols, rows int) int {
	return rows * (len(fgColor+"255;255;255m\x1b[39m\n") + 3*4 + cols*utf8.RuneLen(kittyPlaceholder))
}
