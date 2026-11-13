package rasterm

import (
	"bytes"
	"context"
	"errors"
	"image"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// These tests open a pseudo-terminal. The test plays the terminal on the
// master side, and the package asks its questions on the slave side, as on a
// real terminal (D23). Opening a pseudo-terminal this way works on Linux.

func TestTermQuery(t *testing.T) {
	const da1 = "\x1b[?62;4c"
	tests := []struct {
		name    string
		reply   string
		replies []string
		pending string
		rest    string
	}{
		{"sixel", da1, []string{da1}, "", ""},
		{"no sixel", "\x1b[?1;2c", []string{"\x1b[?1;2c"}, "", ""},
		{"kitty", "\x1b_Gi=31;OK\x1b\\" + da1, []string{"\x1b_Gi=31;OK\x1b\\", da1}, "", ""},
		{"xtversion", "\x1bP>|kitty(0.50.0)\x1b\\" + da1, []string{"\x1bP>|kitty(0.50.0)\x1b\\", da1}, "", ""},
		// input after the answers is not read, so the program gets it
		{"input after the reply", da1 + "echo hello\n", []string{da1}, "", "echo hello\n"},
		// keys before the answers are set aside, and the answers are still read
		{"keys before the reply", "select 1;\n" + da1, []string{da1}, "select 1;\n", ""},
		{"keys around the reply", "s" + da1 + "elect 1;\n", []string{da1}, "s", "elect 1;\n"},
		// an arrow key is an escape sequence that is not an answer
		{"arrow key", "\x1b[A" + da1, []string{da1}, "\x1b[A", ""},
		{"escape key", "\x1b\x1b[?62;4c", []string{da1}, "\x1b", ""},
		{"escape and bracket", "\x1b[\x1b[?62;4c", []string{da1}, "\x1b[", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			master, slave := openPty(t)
			go answer(t, master, test.reply)
			replies, pending, err := termQuery(context.Background(), slave, slave, termQueries, time.Second)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if string(pending) != test.pending {
				t.Errorf("expected the keys %q, got: %q", test.pending, pending)
			}
			var got []string
			for _, r := range replies {
				got = append(got, string(r))
			}
			if !slices.Equal(got, test.replies) {
				t.Errorf("expected %q, got: %q", test.replies, got)
			}
			if test.rest != "" {
				if got := readLine(t, slave); got != test.rest {
					t.Errorf("expected the program to get %q, got: %q", test.rest, got)
				}
			}
		})
	}
}

func TestTermQueryNoAnswer(t *testing.T) {
	// a terminal that does not answer, as Emacs or a serial console
	master, slave := openPty(t)
	go answer(t, master, "")
	start := time.Now()
	_, pending, err := termQuery(context.Background(), slave, slave, termQueries, 100*time.Millisecond)
	if !errors.Is(err, ErrTermResponseTimedOut) {
		t.Fatalf("expected %v, got: %v", ErrTermResponseTimedOut, err)
	}
	if len(pending) != 0 {
		t.Errorf("expected no keys, got: %q", pending)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("expected to wait about 100ms, waited: %v", d)
	}
	// what the user types after the wait reaches the program
	if _, err := master.WriteString("\\echo hello\n"); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got := readLine(t, slave); got != "\\echo hello\n" {
		t.Errorf("expected the program to get the whole line, got: %q", got)
	}
}

func TestTermQueryKeysNoAnswer(t *testing.T) {
	// keys from a user, and no answer from the terminal
	master, slave := openPty(t)
	go answer(t, master, "\\echo\x1b[")
	_, pending, err := termQuery(context.Background(), slave, slave, termQueries, 100*time.Millisecond)
	if !errors.Is(err, ErrTermResponseTimedOut) {
		t.Fatalf("expected %v, got: %v", ErrTermResponseTimedOut, err)
	}
	if got, want := string(pending), "\\echo\x1b["; got != want {
		t.Errorf("expected the keys %q, got: %q", want, got)
	}
}

func TestTermQueryCancel(t *testing.T) {
	// a cancel stops the wait before the timeout
	master, slave := openPty(t)
	go answer(t, master, "")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _, err := termQuery(ctx, slave, slave, termQueries, 10*time.Second)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected %v, got: %v", context.DeadlineExceeded, err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("expected to stop after about 50ms, waited: %v", d)
	}
}

func TestDetectQuery(t *testing.T) {
	master, slave := openPty(t)
	go answer(t, master, "k\x1b_Gi=31;OK\x1b\\\x1b_Gi=32,e=a;OK\x1b\\\x1bP>|kitty(0.50.0)\x1b\\\x1b[?1;0;1024S\x1b[6;20;10t\x1b[?62;4c")
	env := map[string]string{"TERM": "xterm-kitty"}
	term, pending, err := Detect(context.Background(),
		WithInput(slave), WithOutput(slave), WithTimeout(time.Second),
		WithGetenv(func(k string) string { return env[k] }))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	exp := Terminal{
		Protocol:       Kitty,
		Source:         SourceQuery,
		Protocols:      []Protocol{Kitty, Sixel},
		Name:           "kitty",
		Version:        "0.50.0",
		Cell:           image.Pt(10, 20),
		SixelColors:    1024,
		KittyAnimation: true,
	}
	if !reflect.DeepEqual(term, exp) {
		t.Errorf("expected %+v, got: %+v", exp, term)
	}
	if string(pending) != "k" {
		t.Errorf("expected the keys %q, got: %q", "k", pending)
	}
}

func TestDetectNoAnswerReadsTheEnvironment(t *testing.T) {
	master, slave := openPty(t)
	go answer(t, master, "")
	env := map[string]string{"TERM_PROGRAM": "WezTerm"}
	term, _, err := Detect(context.Background(),
		WithInput(slave), WithOutput(slave), WithTimeout(50*time.Millisecond),
		WithGetenv(func(k string) string { return env[k] }))
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if term.Protocol != ITerm || term.Source != SourceEnv {
		t.Errorf("expected iterm from env, got: %v from %v", term.Protocol, term.Source)
	}
}

func TestCanQueryTerm(t *testing.T) {
	_, slave := openPty(t)
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"dumb", map[string]string{"TERM": "dumb"}},
		{"emacs", map[string]string{"TERM": "xterm-256color", "INSIDE_EMACS": "30.1,term:0.96"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if canQueryTerm(slave, slave, func(k string) string { return test.env[k] }) {
				t.Errorf("expected no query")
			}
		})
	}
	if !canQueryTerm(slave, slave, func(string) string { return "" }) {
		t.Errorf("expected a query on a pseudo-terminal")
	}
}

// openPty opens a pseudo-terminal, and returns its master and slave sides.
func openPty(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Skipf("no pseudo-terminal: %v", err)
	}
	t.Cleanup(func() { master.Close() })
	fd := int(master.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	n, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	slave, err := os.OpenFile("/dev/pts/"+strconv.Itoa(n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	t.Cleanup(func() { slave.Close() })
	return master, slave
}

// answer reads the query from master, as a terminal does, and then writes
// reply. An empty reply is no answer.
func answer(t *testing.T, master *os.File, reply string) {
	var got []byte
	buf := make([]byte, 64)
	for !bytes.Contains(got, []byte("\x1b[c")) {
		n, err := master.Read(buf)
		if err != nil {
			return
		}
		got = append(got, buf[:n]...)
	}
	if reply != "" {
		master.WriteString(reply)
	}
}

// readLine reads one line from slave, which is back in canonical mode.
func readLine(t *testing.T, slave *os.File) string {
	t.Helper()
	if err := pollIn(context.Background(), int(slave.Fd()), time.Now().Add(2*time.Second)); err != nil {
		t.Fatalf("expected input for the program, got: %v", err)
	}
	buf := make([]byte, 256)
	n, err := unix.Read(int(slave.Fd()), buf)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return strings.ReplaceAll(string(buf[:n]), "\r", "")
}
