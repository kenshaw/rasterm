package rasterm

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// These tests open a pseudo-terminal. The test plays the terminal on the
// master side, and the package asks its question on the slave side, as on a
// real terminal (D23). Opening a pseudo-terminal this way works on Linux.

func TestTermAttributes(t *testing.T) {
	tests := []struct {
		name    string
		reply   string
		attrs   []int
		pending string
		rest    string
	}{
		{"sixel", "\x1b[?62;4;22c", []int{62, 4, 22}, "", ""},
		{"no sixel", "\x1b[?1;2c", []int{1, 2}, "", ""},
		// input after the reply is not read, so the program gets it
		{"input after the reply", "\x1b[?62;4cecho hello\n", []int{62, 4}, "", "echo hello\n"},
		// keys before the reply are set aside, and the reply is still read
		{"keys before the reply", "select 1;\n\x1b[?62;4c", []int{62, 4}, "select 1;\n", ""},
		{"keys around the reply", "s\x1b[?62;4celect 1;\n", []int{62, 4}, "s", "elect 1;\n"},
		// an arrow key starts as the reply does
		{"arrow key", "\x1b[A\x1b[?62;4c", []int{62, 4}, "\x1b[A", ""},
		{"escape key", "\x1b\x1b[?62;4c", []int{62, 4}, "\x1b", ""},
		{"escape and bracket", "\x1b[\x1b[?62;4c", []int{62, 4}, "\x1b[", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			master, slave := openPty(t)
			go answer(t, master, test.reply)
			attrs, pending, err := termAttributes(slave, slave, time.Second)
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if string(pending) != test.pending {
				t.Errorf("expected the keys %q, got: %q", test.pending, pending)
			}
			if !slices.Equal(attrs, test.attrs) {
				t.Errorf("expected %v, got: %v", test.attrs, attrs)
			}
			if test.rest != "" {
				if got := readLine(t, slave); got != test.rest {
					t.Errorf("expected the program to get %q, got: %q", test.rest, got)
				}
			}
		})
	}
}

func TestTermAttributesNoAnswer(t *testing.T) {
	// a terminal that does not answer, as Emacs or a serial console
	master, slave := openPty(t)
	go answer(t, master, "")
	start := time.Now()
	_, pending, err := termAttributes(slave, slave, 100*time.Millisecond)
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

func TestTermAttributesKeysNoAnswer(t *testing.T) {
	// keys from a user, and no answer from the terminal
	master, slave := openPty(t)
	go answer(t, master, "\\echo\x1b[")
	_, pending, err := termAttributes(slave, slave, 100*time.Millisecond)
	if !errors.Is(err, ErrTermResponseTimedOut) {
		t.Fatalf("expected %v, got: %v", ErrTermResponseTimedOut, err)
	}
	if got, want := string(pending), "\\echo\x1b["; got != want {
		t.Errorf("expected the keys %q, got: %q", want, got)
	}
}

func TestPendingInput(t *testing.T) {
	addPendingInput([]byte("se"))
	addPendingInput(nil)
	addPendingInput([]byte("lect"))
	if got := string(PendingInput()); got != "select" {
		t.Errorf("expected %q, got: %q", "select", got)
	}
	// each byte is returned once
	if got := PendingInput(); got != nil {
		t.Errorf("expected nil, got: %q", got)
	}
}

func TestDAReply(t *testing.T) {
	tests := []struct {
		s        string
		ok, done bool
	}{
		{"\x1b", true, false},
		{"\x1b[", true, false},
		{"\x1b[?", true, false},
		{"\x1b[?62", true, false},
		{"\x1b[?62;4", true, false},
		{"\x1b[?62;4c", true, true},
		{"x", false, false},
		{"\x1b]", false, false},
		{"\x1b[1", false, false},
		{"\x1b[?62R", false, false},
	}
	for _, test := range tests {
		t.Run(strconv.Quote(test.s), func(t *testing.T) {
			if ok, done := daReply([]byte(test.s)); ok != test.ok || done != test.done {
				t.Errorf("expected %t %t, got: %t %t", test.ok, test.done, ok, done)
			}
		})
	}
}

func TestCanQueryTerm(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"dumb", map[string]string{"TERM": "dumb"}},
		{"emacs", map[string]string{"TERM": "xterm-256color", "INSIDE_EMACS": "30.1,term:0.96"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if canQueryTerm(func(k string) string { return test.env[k] }) {
				t.Errorf("expected no query")
			}
		})
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
	for !bytes.Contains(got, []byte("\x1b[0c")) {
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
	if err := pollIn(int(slave.Fd()), 2*time.Second); err != nil {
		t.Fatalf("expected input for the program, got: %v", err)
	}
	buf := make([]byte, 256)
	n, err := unix.Read(int(slave.Fd()), buf)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	return strings.ReplaceAll(string(buf[:n]), "\r", "")
}
