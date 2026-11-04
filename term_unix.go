//go:build unix

package rasterm

import (
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// termQueryTimeout is how long the package waits for the terminal to answer
// a query. A terminal on the same host answers in under a millisecond, and
// one over SSH in one round trip (D23).
const termQueryTimeout = 250 * time.Millisecond

// hasSixelSupport returns true when the terminal reports Sixel graphics in
// its device attributes. It asks the terminal once, and only when
// [canQueryTerm] is true (D23).
var hasSixelSupport = sync.OnceValue(func() bool {
	if !canQueryTerm(os.Getenv) {
		return false
	}
	attrs, err := termAttributes(os.Stdin, os.Stdout, termQueryTimeout)
	if err != nil {
		return false
	}
	// ignore the first attribute, which is the terminal id, and can be 4
	return len(attrs) > 1 && slices.Contains(attrs[1:], 4)
})

// canQueryTerm returns true when the terminal can answer a query. Standard
// input and output must be terminals. A dumb terminal and the terminals of
// Emacs do not answer, so the package does not ask them.
func canQueryTerm(getenv func(string) string) bool {
	switch {
	case getenv("TERM") == "dumb", getenv("INSIDE_EMACS") != "":
		return false
	}
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

// termAttributes asks the terminal for its device attributes, and returns
// them. It waits up to timeout for the answer (D23).
//
//	CSI Ps c  Send Device Attributes (Primary DA).
//		Ps = 0  or omitted ⇒  request attributes from terminal.  The
//	response depends on the decTerminalID resource setting.
//		⇒  CSI ? 1 ; 2 c     ("VT100 with Advanced Video Option")
//		⇒  CSI ? 1 ; 0 c     ("VT101 with No Options")
//		⇒  CSI ? 4 ; 6 c     ("VT132 with Advanced Video and Graphics")
//		⇒  CSI ? 6 c         ("VT102")
//		⇒  CSI ? 7 c         ("VT131")
//		⇒  CSI ? 1 2 ; Ps c  ("VT125")
//		⇒  CSI ? 6 2 ; Ps c  ("VT220")
//		⇒  CSI ? 6 3 ; Ps c  ("VT320")
//		⇒  CSI ? 6 4 ; Ps c  ("VT420")
//
// The VT100-style response parameters do not mean anything by themselves.
// VT220 (and higher) parameters do, telling the host what features the
// terminal supports:
//
//	Ps = 1    ⇒  132-columns.
//	Ps = 2    ⇒  Printer.
//	Ps = 3    ⇒  ReGIS graphics.
//	Ps = 4    ⇒  Sixel graphics.
//	Ps = 6    ⇒  Selective erase.
//	Ps = 8    ⇒  User-defined keys.
//	Ps = 9    ⇒  National Replacement Character sets.
//	Ps = 1 5  ⇒  Technical characters.
//	Ps = 1 6  ⇒  Locator port.
//	Ps = 1 7  ⇒  Terminal state interrogation.
//	Ps = 1 8  ⇒  User windows.
//	Ps = 2 1  ⇒  Horizontal scrolling.
//	Ps = 2 2  ⇒  ANSI color, e.g., VT525.
//	Ps = 2 8  ⇒  Rectangular editing.
//	Ps = 2 9  ⇒  ANSI text locator (i.e., DEC Locator mode).
//
// NOTE: must be connected to an actual terminal for this to work
//
// See: https://invisible-island.net/xterm/ctlseqs/ctlseqs.html#h4-Functions-using-CSI-_-ordered-by-the-final-character-lparen-s-rparen:CSI-Ps-c.1CA3
func termAttributes(in, out *os.File, timeout time.Duration) ([]int, error) {
	reply, err := termQuery(in, out, "\x1b[0c", daReply, timeout)
	if err != nil {
		return nil, err
	}
	// the reply is ESC [ ? Ps ; ... ; Ps c
	fields := strings.Split(string(reply[3:len(reply)-1]), ";")
	attrs := make([]int, 0, len(fields))
	for _, f := range fields {
		if n, err := strconv.Atoi(f); err == nil {
			attrs = append(attrs, n)
		}
	}
	return attrs, nil
}

// daReply reports whether b, the start of a reply, can be the start of an
// answer to a request for the device attributes, ESC [ ? Ps ; ... ; Ps c, and
// whether it is the whole answer.
func daReply(b []byte) (ok, done bool) {
	i, c := len(b)-1, b[len(b)-1]
	switch {
	case i == 0:
		return c == 0x1b, false
	case i == 1:
		return c == '[', false
	case i == 2:
		return c == '?', false
	case c == 'c':
		return true, true
	}
	return '0' <= c && c <= '9' || c == ';', false
}

// termQuery writes the query req to out, and returns the reply that the
// terminal writes to in. in is in raw mode while the package waits, so that
// the reply is not shown, and it goes back to its mode after.
//
// The package reads the reply one byte at a time, and asks reply for each
// byte if the reply is whole, so that it never reads a byte after the reply.
// It stops at the first byte that cannot be in the reply. It waits up to
// timeout, with poll, so that a terminal that does not answer does not hold
// the program. A key that the user types before the reply is read and lost.
// A reply that comes after timeout reaches the program as input (D23).
func termQuery(in, out *os.File, req string, reply func([]byte) (ok, done bool), timeout time.Duration) (_ []byte, err error) {
	fd := int(in.Fd())
	if !term.IsTerminal(fd) {
		return nil, ErrNonTTY
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	defer func() {
		if e := term.Restore(fd, old); err == nil {
			err = e
		}
	}()
	if _, err := out.WriteString(req); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(timeout)
	var buf []byte
	var b [1]byte
	for {
		if err := pollIn(fd, time.Until(deadline)); err != nil {
			return nil, err
		}
		// one byte at a time, so that no byte after the reply is read
		n, err := unix.Read(fd, b[:])
		switch {
		case errors.Is(err, unix.EINTR), errors.Is(err, unix.EAGAIN):
			continue
		case err != nil:
			return nil, err
		case n == 0:
			return nil, ErrTermResponseTimedOut
		}
		buf = append(buf, b[0])
		switch ok, done := reply(buf); {
		case !ok:
			return nil, ErrUnexpectedTermResponse
		case done:
			return buf, nil
		}
	}
}

// pollIn waits up to d for fd to have input. It returns
// [ErrTermResponseTimedOut] when d passes with no input.
func pollIn(fd int, d time.Duration) error {
	for {
		if d <= 0 {
			return ErrTermResponseTimedOut
		}
		start := time.Now()
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, int((d+time.Millisecond-1)/time.Millisecond))
		switch {
		case errors.Is(err, unix.EINTR):
			d -= time.Since(start)
			continue
		case err != nil:
			return err
		case n == 0:
			return ErrTermResponseTimedOut
		}
		return nil
	}
}
