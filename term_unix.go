//go:build unix

package rasterm

import (
	"context"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// canQueryTerm returns true when the terminal of in and out can answer a
// query. Both must be terminals. A dumb terminal and the terminals of Emacs
// do not answer, so the package does not ask them.
func canQueryTerm(in, out *os.File, getenv func(string) string) bool {
	switch {
	case in == nil, out == nil:
		return false
	case getenv("TERM") == "dumb", getenv("INSIDE_EMACS") != "":
		return false
	}
	return term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
}

// termQuery writes the queries req to out, and returns the answers that the
// terminal writes to in, and the bytes that it read that are not part of an
// answer. in is in raw mode while the package waits, so that the answers are
// not shown, and it goes back to its mode after.
//
// The package reads one byte at a time, and stops after the answer to DA1,
// so that it never reads a byte after the answers. A byte that is not part
// of an answer is a key that the user typed. The package sets it aside, and
// waits for the answers. It waits up to timeout, with poll, so that a
// terminal that does not answer does not hold the program. It returns
// [ErrTermResponseTimedOut] when the answer to DA1 does not come in time. An
// answer that comes after timeout reaches the program as input (D23, D24).
func termQuery(ctx context.Context, in, out *os.File, req string, timeout time.Duration) (_ [][]byte, pending []byte, err error) {
	fd := int(in.Fd())
	if !term.IsTerminal(fd) {
		return nil, nil, ErrNonTTY
	}
	old, err := term.MakeRaw(fd)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		if e := term.Restore(fd, old); err == nil {
			err = e
		}
	}()
	if _, err := out.WriteString(req); err != nil {
		return nil, nil, err
	}
	deadline := time.Now().Add(timeout)
	var s replyScanner
	var b [1]byte
	for !s.done {
		if err := pollIn(ctx, fd, deadline); err != nil {
			// the start of an answer that did not end can be keys too
			return s.replies, s.flush(), err
		}
		// one byte at a time, so that no byte after the answers is read
		n, err := unix.Read(fd, b[:])
		switch {
		case errors.Is(err, unix.EINTR), errors.Is(err, unix.EAGAIN):
			continue
		case err != nil:
			return s.replies, s.flush(), err
		case n == 0:
			return s.replies, s.flush(), ErrTermResponseTimedOut
		}
		s.add(b[0])
	}
	return s.replies, s.flush(), nil
}

// pollSlice is the longest that [pollIn] waits before it checks its context.
const pollSlice = 20 * time.Millisecond

// pollIn waits until fd has input, until deadline, or until ctx is done. It
// returns [ErrTermResponseTimedOut] when deadline passes with no input, and
// the error of ctx when ctx is done. It waits in slices of [pollSlice], so
// that a cancel stops the wait soon.
func pollIn(ctx context.Context, fd int, deadline time.Time) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		d := time.Until(deadline)
		if d <= 0 {
			return ErrTermResponseTimedOut
		}
		d = min(d, pollSlice)
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		n, err := unix.Poll(fds, int((d+time.Millisecond-1)/time.Millisecond))
		switch {
		case errors.Is(err, unix.EINTR), err == nil && n == 0:
			continue
		case err != nil:
			return err
		}
		return nil
	}
}
