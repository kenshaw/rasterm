package rasterm

import (
	"bytes"
	"fmt"
	"image"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestKittyCells(t *testing.T) {
	tests := []struct {
		w, h, cw, ch int
		cols, rows   int
		ok           bool
	}{
		{100, 100, 10, 20, 10, 5, true},
		{101, 101, 10, 20, 11, 6, true},
		{1, 1, 10, 20, 1, 1, true},
		// no cell size
		{100, 100, 0, 0, 0, 0, false},
		{100, 100, 10, 0, 0, 0, false},
		// more rows than the diacritics can number
		{100, 297 * 20, 10, 20, 10, 297, true},
		{100, 297*20 + 1, 10, 20, 10, 298, false},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%dx%d/%dx%d", test.w, test.h, test.cw, test.ch), func(t *testing.T) {
			cols, rows, ok := kittyCells(test.w, test.h, test.cw, test.ch)
			if ok != test.ok || ok && (cols != test.cols || rows != test.rows) {
				t.Errorf("expected %d x %d %t, got: %d x %d %t", test.cols, test.rows, test.ok, cols, rows, ok)
			}
		})
	}
}

func TestKittyPlaceholder(t *testing.T) {
	flat := flatImage(256, 100)
	inputs := []struct {
		name string
		img  image.Image
	}{
		{"rgba zlib", RGBA{Image: flat, Data: packRGBA(flat)}},
		{"png as is", PNG{Image: flat, Data: mustPNG(t, flat)}},
		{"encoded png", flat},
	}
	for _, in := range inputs {
		t.Run(in.name, func(t *testing.T) {
			withTmux(t, true)
			withTmuxClient(t, tmuxClient{protocol: "kitty", cellWidth: 10, cellHeight: 20})
			var buf bytes.Buffer
			if err := encoders[Kitty].Encode(&buf, in.img); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			seqs, rest := unwrapTmux(t, buf.String())
			keys, _ := decodeKitty(t, strings.Join(seqs, ""))
			for k, v := range map[string]string{"a": "T", "U": "1", "q": "2", "c": "26", "r": "5"} {
				if keys[k] != v {
					t.Errorf("expected %s=%s in the first sequence, got: %v", k, v, keys)
				}
			}
			for i, seq := range seqs[1:] {
				if !strings.HasPrefix(seq, "\x1b_Gq=2,m=") {
					t.Fatalf("chunk %d: expected q=2, so that the terminal writes no response, got: %.20q", i, seq)
				}
			}
			id, err := strconv.ParseUint(keys["i"], 10, 32)
			if err != nil || id&0xffffff == 0 {
				t.Fatalf("expected an image id with low bits that are not 0, got: %q", keys["i"])
			}
			// the placeholder cells are outside the envelopes, as text that
			// tmux keeps, and the newline of the encoder follows them
			rows := readPlaceholder(t, strings.TrimSuffix(rest, "\n"))
			if len(rows) != 5 {
				t.Fatalf("expected 5 rows of placeholder cells, got: %d", len(rows))
			}
			for y, row := range rows {
				if row.id != uint32(id) || row.row != y || row.col != 0 || row.cells != 26 {
					t.Errorf("row %d: expected id %d, row %d, column 0 and 26 cells, got: %+v", y, id, y, row)
				}
			}
		})
	}
}

func TestKittyPlaceholderIDs(t *testing.T) {
	withTmux(t, true)
	withTmuxClient(t, tmuxClient{protocol: "kitty", cellWidth: 10, cellHeight: 20})
	img := flatImage(40, 40)
	ids := make(map[string]bool)
	for range 20 {
		var buf bytes.Buffer
		if err := encoders[Kitty].Encode(&buf, img); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		seqs, _ := unwrapTmux(t, buf.String())
		keys, _ := decodeKitty(t, strings.Join(seqs, ""))
		ids[keys["i"]] = true
	}
	// each image gets its own id, so that a new image does not replace one
	// that is on the screen
	if len(ids) != 20 {
		t.Errorf("expected 20 ids for 20 images, got: %d", len(ids))
	}
}

func TestKittyPlaceholderFallsBack(t *testing.T) {
	tests := []struct {
		name string
		c    tmuxClient
		img  image.Image
	}{
		{"no cell size", tmuxClient{protocol: "kitty"}, flatImage(40, 40)},
		{"too many rows", tmuxClient{protocol: "kitty", cellWidth: 10, cellHeight: 1}, flatImage(40, 400)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withTmux(t, true)
			withTmuxClient(t, test.c)
			var buf bytes.Buffer
			if err := encoders[Kitty].Encode(&buf, test.img); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			seqs, rest := unwrapTmux(t, buf.String())
			keys, _ := decodeKitty(t, strings.Join(seqs, ""))
			if keys["U"] != "" || rest != "\n" {
				t.Errorf("expected the image to be drawn at once, with no placeholder, got keys %v and %q", keys, rest)
			}
		})
	}
}

func TestKittyPlaceholderWritesOnce(t *testing.T) {
	withTmux(t, true)
	withTmuxClient(t, tmuxClient{protocol: "kitty", cellWidth: 10, cellHeight: 20})
	var w countWriter
	if err := encoders[Kitty].Encode(&w, flatImage(300, 300)); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if w.writes != 1 {
		t.Errorf("expected 1 write, got: %d", w.writes)
	}
}

// placeholderRow is what the terminal reads from a row of placeholder cells.
type placeholderRow struct {
	id       uint32
	row, col int
	cells    int
}

// readPlaceholder reads rows of placeholder cells as the terminal does. Each
// row must set a 24-bit foreground color, then have a first cell with the
// diacritics of its row, its column and the high byte of the id, then cells
// with no diacritics, then reset the foreground color.
func readPlaceholder(t *testing.T, s string) []placeholderRow {
	t.Helper()
	index := make(map[rune]int)
	for i, r := range kittyDiacritics {
		index[r] = i
	}
	var rows []placeholderRow
	for i, line := range strings.Split(s, "\n") {
		var r, g, b, n int
		if _, err := fmt.Sscanf(line, "\x1b[38;2;%d;%d;%dm%n", &r, &g, &b, &n); err != nil {
			// Sscanf does not report %n with no error, so find the m
			n = strings.IndexByte(line, 'm') + 1
			if _, err := fmt.Sscanf(line[:n], "\x1b[38;2;%d;%d;%dm", &r, &g, &b); err != nil {
				t.Fatalf("row %d: expected a 24-bit foreground color, got: %.30q", i, line)
			}
		}
		line = line[n:]
		if !strings.HasSuffix(line, "\x1b[39m") {
			t.Fatalf("row %d: expected the foreground color to be reset at the end", i)
		}
		cells := []rune(strings.TrimSuffix(line, "\x1b[39m"))
		if len(cells) < 4 || cells[0] != kittyPlaceholder {
			t.Fatalf("row %d: expected a placeholder cell with three diacritics, got: %q", i, string(cells))
		}
		row := placeholderRow{
			id:  uint32(index[cells[3]])<<24 | uint32(r)<<16 | uint32(g)<<8 | uint32(b),
			row: index[cells[1]],
			col: index[cells[2]],
		}
		for _, c := range cells[4:] {
			if c != kittyPlaceholder {
				t.Fatalf("row %d: expected only placeholder cells after the first, got: %U", i, c)
			}
		}
		row.cells = 1 + len(cells) - 4
		rows = append(rows, row)
	}
	if !utf8.ValidString(s) {
		t.Errorf("expected valid UTF-8")
	}
	return rows
}
