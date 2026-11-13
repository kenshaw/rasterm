package rasterm

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

// kittySeqRE matches a Kitty escape sequence, with its keys and its data.
var kittySeqRE = regexp.MustCompile("\x1b_G([^;\x1b]*)(?:;([^\x1b]*))?\x1b\\\\")

// kittySeqs returns the keys of each Kitty escape sequence in s, and the
// data of each sequence that has data, in order.
func kittySeqs(s string) ([]map[string]string, []string) {
	var keys []map[string]string
	var data []string
	for _, m := range kittySeqRE.FindAllStringSubmatch(s, -1) {
		k := make(map[string]string)
		for _, kv := range kittyKeyRE.FindAllStringSubmatch(m[1], -1) {
			k[kv[1]] = kv[2]
		}
		keys, data = append(keys, k), append(data, m[2])
	}
	return keys, data
}

func TestAnimate(t *testing.T) {
	var w countWriter
	var buf bytes.Buffer
	enc := KittyEncoder{Zlib: ZlibNever}
	root := flatImage(40, 30)
	a, err := enc.Animate(&buf, root, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	id := strconv.FormatUint(uint64(a.ID()), 10)
	keys, _ := kittySeqs(buf.String())
	if keys[0]["a"] != "T" || keys[0]["i"] != id || keys[0]["q"] != "2" {
		t.Fatalf("expected the root frame with a=T, i=%s and q=2, got: %v", id, keys[0])
	}
	n := len(keys)
	if c := keys[n-2]; c["a"] != "a" || c["i"] != id || c["r"] != "1" || c["z"] != "100" {
		t.Errorf("expected the gap of the root frame, got: %v", c)
	}
	if c := keys[n-1]; c["a"] != "a" || c["s"] != "2" {
		t.Errorf("expected loading mode, got: %v", c)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("expected a newline after the root frame")
	}
	tests := []struct {
		name string
		img  image.Image
		f    KittyFrame
		exp  map[string]string
		no   []string
	}{
		{
			"whole frame", flatImage(40, 30), KittyFrame{Delay: 50 * time.Millisecond},
			map[string]string{"a": "f", "i": id, "q": "2", "c": "1", "z": "50", "f": "100"},
			[]string{"x", "y", "X", "Y"},
		},
		{
			"partial frame", flatImage(10, 5), KittyFrame{Delay: time.Second, At: image.Pt(3, 4), Base: 1, Replace: true},
			map[string]string{"a": "f", "x": "3", "y": "4", "c": "1", "X": "1", "z": "1000"},
			[]string{"Y"},
		},
		{
			"background", flatImage(40, 30), KittyFrame{Base: -1, Background: color.NRGBA{R: 1, G: 2, B: 3, A: 4}},
			map[string]string{"a": "f", "Y": strconv.Itoa(0x01020304), "z": "1"},
			[]string{"c"},
		},
		{
			"gapless", flatImage(40, 30), KittyFrame{Delay: -1},
			map[string]string{"a": "f", "z": "-1", "c": "4"},
			nil,
		},
		{
			"raw frame", RGBA{Image: flatImage(8, 6), Data: packRGBA(flatImage(8, 6))}, KittyFrame{At: image.Pt(1, 2)},
			map[string]string{"a": "f", "f": "32", "s": "8", "v": "6", "x": "1", "y": "2", "c": "5"},
			nil,
		},
	}
	for i, test := range tests {
		buf.Reset()
		w = countWriter{w: &buf}
		a.w = &w
		num, err := a.Frame(test.img, test.f)
		if err != nil {
			t.Fatalf("%s: expected no error, got: %v", test.name, err)
		}
		if num != i+2 {
			t.Errorf("%s: expected frame %d, got: %d", test.name, i+2, num)
		}
		if w.writes != 1 {
			t.Errorf("%s: expected 1 write, got: %d", test.name, w.writes)
		}
		keys, _ := kittySeqs(buf.String())
		for k, v := range test.exp {
			if keys[0][k] != v {
				t.Errorf("%s: expected %s=%s, got: %v", test.name, k, v, keys[0])
			}
		}
		for _, k := range test.no {
			if _, ok := keys[0][k]; ok {
				t.Errorf("%s: expected no %s, got: %v", test.name, k, keys[0])
			}
		}
		// each chunk of a frame repeats a=f
		for j, c := range keys[1:] {
			if c["a"] != "f" {
				t.Errorf("%s: chunk %d: expected a=f, got: %v", test.name, j, c)
			}
		}
		if strings.Contains(buf.String(), "\n") {
			t.Errorf("%s: expected no newline after a frame", test.name)
		}
	}
	for _, test := range []struct {
		name string
		call func() error
		exp  map[string]string
	}{
		{"stop", a.Stop, map[string]string{"a": "a", "s": "1", "i": id}},
		{"end", func() error { return a.End(3) }, map[string]string{"a": "a", "s": "3", "v": "4", "i": id}},
		{"delete", a.Delete, map[string]string{"a": "d", "d": "I", "i": id, "q": "2"}},
	} {
		buf.Reset()
		w = countWriter{w: &buf}
		a.w = &w
		if err := test.call(); err != nil {
			t.Fatalf("%s: expected no error, got: %v", test.name, err)
		}
		keys, _ := kittySeqs(buf.String())
		if len(keys) != 1 || w.writes != 1 {
			t.Fatalf("%s: expected one sequence in one write, got %d in %d", test.name, len(keys), w.writes)
		}
		for k, v := range test.exp {
			if keys[0][k] != v {
				t.Errorf("%s: expected %s=%s, got: %v", test.name, k, v, keys[0])
			}
		}
	}
	if _, err := a.Frame(root, KittyFrame{}); !errors.Is(err, ErrAnimationEnded) {
		t.Errorf("expected %v, got: %v", ErrAnimationEnded, err)
	}
}

func TestAnimateErrors(t *testing.T) {
	var buf bytes.Buffer
	a, err := KittyEncoder{}.Animate(&buf, flatImage(40, 30), 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	tests := []struct {
		name string
		img  image.Image
		f    KittyFrame
		exp  error
	}{
		{"too wide", flatImage(41, 30), KittyFrame{}, ErrFrameOutOfBounds},
		{"outside", flatImage(10, 10), KittyFrame{At: image.Pt(35, 0)}, ErrFrameOutOfBounds},
		{"negative", flatImage(10, 10), KittyFrame{At: image.Pt(-1, 0)}, ErrFrameOutOfBounds},
		{"no base frame", flatImage(10, 10), KittyFrame{Base: 2}, ErrUnknownFrame},
		{"bad base", flatImage(10, 10), KittyFrame{Base: -2}, ErrUnknownFrame},
	}
	for _, test := range tests {
		if _, err := a.Frame(test.img, test.f); !errors.Is(err, test.exp) {
			t.Errorf("%s: expected %v, got: %v", test.name, test.exp, err)
		}
	}
	for _, loops := range []int{-1, 1 << 40} {
		if err := a.End(loops); !errors.Is(err, ErrInvalidLoops) {
			t.Errorf("loops %d: expected %v, got: %v", loops, ErrInvalidLoops, err)
		}
	}
	if a.Frames() != 1 {
		t.Errorf("expected 1 frame, got: %d", a.Frames())
	}
}

func TestAnimateSubImage(t *testing.T) {
	// a frame from SubImage has bounds that do not start at 0,0, and At
	// still places it
	var buf bytes.Buffer
	a, err := KittyEncoder{}.Animate(&buf, flatImage(40, 30), 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	sub := flatImage(40, 30).SubImage(image.Rect(20, 20, 30, 30))
	buf.Reset()
	if _, err := a.Frame(sub, KittyFrame{At: image.Pt(30, 20)}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	keys, _ := kittySeqs(buf.String())
	if keys[0]["x"] != "30" || keys[0]["y"] != "20" {
		t.Errorf("expected x=30 and y=20, got: %v", keys[0])
	}
}

func TestAnimateTmux(t *testing.T) {
	var buf bytes.Buffer
	enc := KittyEncoder{Tmux: true, Cell: image.Pt(10, 20)}
	a, err := enc.Animate(&buf, flatImage(40, 40), time.Second)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	seqs, rest := unwrapTmux(t, buf.String())
	keys, _ := kittySeqs(strings.Join(seqs, ""))
	if keys[0]["U"] != "1" || keys[0]["i"] != strconv.FormatUint(uint64(a.ID()), 10) {
		t.Errorf("expected a virtual placement with the id of the animation, got: %v", keys[0])
	}
	if rows := readPlaceholder(t, strings.TrimSuffix(rest, "\n")); len(rows) != 2 {
		t.Errorf("expected 2 rows of placeholder cells, got: %d", len(rows))
	}
	buf.Reset()
	if _, err := a.Frame(flatImage(40, 40), KittyFrame{}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if _, rest := unwrapTmux(t, buf.String()); rest != "" {
		t.Errorf("expected each sequence of a frame in the passthrough, got: %q", rest)
	}
}

func TestKittyGap(t *testing.T) {
	tests := []struct {
		d   time.Duration
		exp int
	}{
		{-time.Second, -1},
		{-1, -1},
		{0, 1},
		{time.Microsecond, 1},
		{time.Millisecond, 1},
		{40 * time.Millisecond, 40},
		{1<<63 - 1, 1<<31 - 1},
	}
	for _, test := range tests {
		if got := kittyGap(test.d); got != test.exp {
			t.Errorf("%v: expected %d, got: %d", test.d, test.exp, got)
		}
	}
}
