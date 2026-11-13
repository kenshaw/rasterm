package rasterm

import (
	"bytes"
	"context"
	"errors"
	"image"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DefaultDetectTimeout is how long [Detect] waits for the terminal to answer,
// unless [WithTimeout] sets it. A terminal on the same host answers in under
// a millisecond, and one over SSH in one round trip (D23).
var DefaultDetectTimeout = 250 * time.Millisecond

// DetectOption is an option of [Detect].
type DetectOption func(*detectOptions)

// detectOptions are the options of [Detect].
type detectOptions struct {
	in, out *os.File
	timeout time.Duration
	getenv  func(string) string
}

// WithInput is a detect option that reads the answers of the terminal from
// f. The default is [os.Stdin].
func WithInput(f *os.File) DetectOption {
	return func(o *detectOptions) {
		o.in = f
	}
}

// WithOutput is a detect option that writes the queries to f. The default is
// [os.Stdout].
func WithOutput(f *os.File) DetectOption {
	return func(o *detectOptions) {
		o.out = f
	}
}

// WithTimeout is a detect option that sets how long [Detect] waits for the
// terminal to answer. The default is [DefaultDetectTimeout].
func WithTimeout(d time.Duration) DetectOption {
	return func(o *detectOptions) {
		o.timeout = d
	}
}

// WithGetenv is a detect option that reads the environment with getenv. The
// default is [os.Getenv]. A program that has its own value for
// TERM_GRAPHICS can give it here.
func WithGetenv(getenv func(string) string) DetectOption {
	return func(o *detectOptions) {
		o.getenv = getenv
	}
}

// Detect finds the terminal graphics protocol of the terminal, in this order
// (D26, D27):
//
//  1. If TERM_GRAPHICS names a protocol, or none, Detect uses it, and asks
//     the terminal nothing. In tmux, it still asks tmux for the size of a
//     cell, which reads no input.
//  2. In tmux, Detect asks tmux for the terminal of its client, and finds
//     the protocols by the name of the terminal. It does not read the other
//     variables of the environment, because they can belong to another
//     terminal (D21).
//  3. If the input and the output are terminals, Detect sends its queries in
//     one write, and reads the answers until the answer to DA1, which every
//     terminal sends last. It waits up to the timeout.
//  4. If the terminal cannot be asked, or does not answer, Detect reads the
//     other variables of the environment, such as TERM and TERM_PROGRAM.
//
// When the terminal has more than one protocol, Detect selects Kitty, then
// iTerm, then Sixel.
//
// While Detect waits for the answers, it reads the input. It returns the keys
// that it read and that are not part of an answer, in the order that the
// user typed them. A program that reads the input after Detect must use them
// as its first input (D24).
//
// A terminal that does not answer is not an error. The error is from ctx, or
// from a failure to read or to write.
func Detect(ctx context.Context, opts ...DetectOption) (Terminal, []byte, error) {
	o := detectOptions{
		in:      os.Stdin,
		out:     os.Stdout,
		timeout: DefaultDetectTimeout,
		getenv:  os.Getenv,
	}
	for _, opt := range opts {
		opt(&o)
	}
	if err := ctx.Err(); err != nil {
		return Terminal{}, nil, err
	}
	tmux := o.getenv("TMUX") != "" || o.getenv("TMUX_PANE") != ""
	if p, ok := ParseProtocol(o.getenv("TERM_GRAPHICS")); ok {
		t := Terminal{Protocol: p, Source: SourceForced, Tmux: tmux}
		if tmux {
			if c, ok := askTmux(ctx, o.getenv); ok {
				t.Name, t.Version, t.Cell, t.Passthrough = c.name, c.version, c.cell, c.passthrough
			}
		}
		return t, nil, nil
	}
	if tmux {
		t := Terminal{Source: SourceTmux, Tmux: true}
		if c, ok := askTmux(ctx, o.getenv); ok {
			t.Name, t.Version, t.Cell, t.Passthrough = c.name, c.version, c.cell, c.passthrough
			t.Protocols = nameProtocols(c.name)
			// tmux drops each sequence when passthrough is off
			if c.passthrough {
				t.Protocol = first(t.Protocols)
			}
		}
		return t, nil, ctx.Err()
	}
	var pending []byte
	if canQueryTerm(o.in, o.out, o.getenv) {
		replies, keys, err := termQuery(ctx, o.in, o.out, termQueries, o.timeout)
		pending = keys
		switch {
		case err == nil:
			t := parseReplies(replies)
			t.Source, t.Protocol = SourceQuery, first(t.Protocols)
			return t, pending, nil
		case !errors.Is(err, ErrTermResponseTimedOut):
			return Terminal{}, pending, err
		}
	}
	t := Terminal{Protocols: envProtocols(o.getenv)}
	if t.Protocol = first(t.Protocols); t.Protocol != None {
		t.Source = SourceEnv
	}
	return t, pending, nil
}

// first returns the first protocol of v, or [None].
func first(v []Protocol) Protocol {
	if len(v) == 0 {
		return None
	}
	return v[0]
}

// termQueries are the queries that [Detect] sends, in one write. The answer
// to DA1 comes last, because a terminal answers each query in order, and
// every terminal answers DA1. The Kitty spec recommends DA1 after its query
// for this reason.
const termQueries = "" +
	// Kitty graphics: a query for a 1 x 1 image, answered by "i=31;OK"
	"\x1b_Gi=31,s=1,v=1,a=q,t=d,f=24;AAAA\x1b\\" +
	// Kitty animation, kitty 0.50.0 and later: answered by "i=32,e=a;OK"
	"\x1b_Ga=q,i=32,e=a\x1b\\" +
	// XTVERSION: the name and the version of the terminal
	"\x1b[>0q" +
	// XTSMGRAPHICS: the number of color registers of Sixel
	"\x1b[?1;1;0S" +
	// the size of a cell in pixels
	"\x1b[16t" +
	// DA1: the device attributes, where 4 is Sixel
	"\x1b[c"

// maxReply is the longest answer that [replyScanner] keeps. A longer string
// sequence is not an answer, and its bytes are keys.
const maxReply = 1024

// replyScanner splits the bytes that the terminal writes, one at a time, into
// answers and keys. An answer is an escape sequence of a form that
// [termQueries] asks for. Each other byte is a key, as is an escape sequence
// of another form, such as an arrow key (D24).
type replyScanner struct {
	// replies are the answers, in the order that they came.
	replies [][]byte
	// pending are the keys, in the order that they came.
	pending []byte
	// done is true after the answer to DA1.
	done bool
	// seq is the start of an escape sequence that did not end yet.
	seq []byte
}

// add adds b, the next byte from the terminal.
func (s *replyScanner) add(b byte) {
	switch {
	case len(s.seq) == 0:
		if b == 0x1b {
			s.seq = append(s.seq, b)
			return
		}
		s.pending = append(s.pending, b)
	case len(s.seq) == 1:
		switch b {
		case '[', '_', 'P', ']':
			s.seq = append(s.seq, b)
		default:
			// ESC and a key, as with Alt, or ESC alone
			s.keys(b)
		}
	case s.seq[1] == '[':
		switch {
		case 0x20 <= b && b <= 0x3f:
			s.seq = append(s.seq, b)
		case 0x40 <= b && b <= 0x7e:
			s.seq = append(s.seq, b)
			s.end()
		default:
			s.keys(b)
		}
	default:
		// APC, DCS and OSC end with ST, which is ESC \, or OSC with BEL
		s.seq = append(s.seq, b)
		n := len(s.seq)
		switch {
		case b == '\\' && s.seq[n-2] == 0x1b, b == 0x07 && s.seq[1] == ']':
			s.end()
		case n > maxReply:
			s.pending = append(s.pending, s.seq...)
			s.seq = s.seq[:0]
		}
	}
}

// keys sets aside the start of the sequence as keys, and adds b again,
// because b can start an answer, as the ESC of an answer after a key does.
func (s *replyScanner) keys(b byte) {
	s.pending = append(s.pending, s.seq...)
	s.seq = s.seq[:0]
	s.add(b)
}

// end ends the sequence. It is an answer when it has the form of one, and
// keys when it does not.
func (s *replyScanner) end() {
	switch seq := s.seq; {
	case isDA1(seq):
		s.replies, s.done = append(s.replies, bytes.Clone(seq)), true
	case isReply(seq):
		s.replies = append(s.replies, bytes.Clone(seq))
	default:
		s.pending = append(s.pending, seq...)
	}
	s.seq = s.seq[:0]
}

// flush sets aside the start of a sequence that did not end as keys, and
// returns the keys.
func (s *replyScanner) flush() []byte {
	s.pending = append(s.pending, s.seq...)
	s.seq = s.seq[:0]
	return s.pending
}

// isDA1 returns true when seq is an answer to DA1, CSI ? Ps ; ... c.
func isDA1(seq []byte) bool {
	return bytes.HasPrefix(seq, []byte("\x1b[?")) && seq[len(seq)-1] == 'c'
}

// isReply returns true when seq has the form of an answer to one of
// [termQueries] other than DA1.
func isReply(seq []byte) bool {
	switch {
	case bytes.HasPrefix(seq, []byte("\x1b_G")):
		return true
	case bytes.HasPrefix(seq, []byte("\x1bP>|")):
		return true
	case bytes.HasPrefix(seq, []byte("\x1b[?")) && seq[len(seq)-1] == 'S':
		return true
	case bytes.HasPrefix(seq, []byte("\x1b[6;")) && seq[len(seq)-1] == 't':
		return true
	}
	return false
}

// parseReplies returns what the answers to [termQueries] tell about the
// terminal. It does not set the protocol or the source.
func parseReplies(replies [][]byte) Terminal {
	var t Terminal
	var kitty, sixel bool
	for _, r := range replies {
		switch {
		case bytes.HasPrefix(r, []byte("\x1b_G")):
			// ESC _ G keys ; message ESC \
			keys, msg, _ := strings.Cut(string(r[3:len(r)-2]), ";")
			k := strings.Split(keys, ",")
			switch {
			case msg != "OK":
			case slices.Contains(k, "i=31"):
				kitty = true
			case slices.Contains(k, "i=32") && slices.Contains(k, "e=a"):
				t.KittyAnimation = true
			}
		case bytes.HasPrefix(r, []byte("\x1bP>|")):
			// DCS > | name version ST
			t.Name, t.Version = splitTermName(string(r[4 : len(r)-2]))
		case isDA1(r):
			// the first attribute is the terminal id, which can be 4
			attrs := params(r[3 : len(r)-1])
			sixel = len(attrs) > 1 && slices.Contains(attrs[1:], 4)
		case bytes.HasPrefix(r, []byte("\x1b[?")):
			// CSI ? 1 ; 0 ; colors S
			if p := params(r[3 : len(r)-1]); len(p) == 3 && p[0] == 1 && p[1] == 0 {
				t.SixelColors = p[2]
			}
		case bytes.HasPrefix(r, []byte("\x1b[6;")):
			// CSI 6 ; height ; width t
			if p := params(r[2 : len(r)-1]); len(p) == 3 && p[1] > 0 && p[2] > 0 {
				t.Cell = image.Pt(p[2], p[1])
			}
		}
	}
	if kitty {
		t.Protocols = append(t.Protocols, Kitty)
	}
	if slices.Contains(nameProtocols(t.Name), ITerm) {
		t.Protocols = append(t.Protocols, ITerm)
	}
	if sixel {
		t.Protocols = append(t.Protocols, Sixel)
	}
	return t
}

// params returns the numbers in buf, which are separated by semicolons. A
// field that is not a number is skipped.
func params(buf []byte) []int {
	var v []int
	for f := range strings.SplitSeq(string(buf), ";") {
		if n, err := strconv.Atoi(f); err == nil {
			v = append(v, n)
		}
	}
	return v
}

// splitTermName splits the name and the version that a terminal reports in
// its answer to XTVERSION, such as "kitty(0.49.2)", "ghostty 1.4.0" or
// "WezTerm 20240203-110809-5046fc22".
func splitTermName(s string) (string, string) {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " ("); i >= 0 {
		return s[:i], strings.TrimSuffix(strings.TrimSpace(s[i+1:]), ")")
	}
	return s, ""
}

// nameProtocols returns the protocols that the terminal of the name supports,
// as far as the package knows. No query asks for the inline images of iTerm,
// and tmux answers the queries itself, so the name tells them.
func nameProtocols(name string) []Protocol {
	switch strings.ToLower(name) {
	case "kitty", "ghostty":
		return []Protocol{Kitty}
	case "iterm2", "wezterm", "mintty":
		return []Protocol{ITerm}
	}
	return nil
}

// envProtocols returns the protocols that the variables of the environment
// tell (D3).
func envProtocols(getenv func(string) string) []Protocol {
	var v []Protocol
	term, program := strings.ToLower(getenv("TERM")), strings.ToLower(getenv("TERM_PROGRAM"))
	if getenv("KITTY_PID") != "" && getenv("KITTY_WINDOW_ID") != "" || term == "xterm-kitty" || program == "ghostty" {
		v = append(v, Kitty)
	}
	if term == "mintty" || strings.ToLower(getenv("LC_TERMINAL")) == "iterm2" || program == "wezterm" {
		v = append(v, ITerm)
	}
	return v
}
