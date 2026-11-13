package rasterm

import (
	"context"
	"errors"
	"image"
	"os"
	"reflect"
	"slices"
	"strconv"
	"testing"
)

func TestReplyScanner(t *testing.T) {
	const da1 = "\x1b[?62;4c"
	tests := []struct {
		in      string
		replies []string
		pending string
		done    bool
	}{
		{da1, []string{da1}, "", true},
		{"\x1b_Gi=31;OK\x1b\\" + da1, []string{"\x1b_Gi=31;OK\x1b\\", da1}, "", true},
		{"\x1bP>|WezTerm 20240203\x1b\\" + da1, []string{"\x1bP>|WezTerm 20240203\x1b\\", da1}, "", true},
		{"\x1b[?1;0;256S\x1b[6;20;10t" + da1, []string{"\x1b[?1;0;256S", "\x1b[6;20;10t", da1}, "", true},
		// keys, and escape sequences that are not answers, are keys
		{"ab" + da1, []string{da1}, "ab", true},
		{"\x1b[A\x1b[15~" + da1, []string{da1}, "\x1b[A\x1b[15~", true},
		{"\x1bx" + da1, []string{da1}, "\x1bx", true},
		{"\x1b\x1b[?62;4c", []string{da1}, "\x1b", true},
		{"\x1b[\x1b[?62;4c", []string{da1}, "\x1b[", true},
		// an OSC is not an answer
		{"\x1b]11;rgb:0/0/0\a" + da1, []string{da1}, "\x1b]11;rgb:0/0/0\a", true},
		// no DA1 yet, and the start of a sequence that did not end
		{"\x1b_Gi=31;OK\x1b\\\x1b[?6", []string{"\x1b_Gi=31;OK\x1b\\"}, "\x1b[?6", false},
	}
	for _, test := range tests {
		t.Run(strconv.Quote(test.in), func(t *testing.T) {
			var s replyScanner
			for i := range len(test.in) {
				if s.done {
					t.Fatalf("expected to stop at DA1, got more bytes")
				}
				s.add(test.in[i])
			}
			var got []string
			for _, r := range s.replies {
				got = append(got, string(r))
			}
			if !slices.Equal(got, test.replies) {
				t.Errorf("expected the answers %q, got: %q", test.replies, got)
			}
			if pending := string(s.flush()); pending != test.pending {
				t.Errorf("expected the keys %q, got: %q", test.pending, pending)
			}
			if s.done != test.done {
				t.Errorf("expected done %t, got: %t", test.done, s.done)
			}
		})
	}
}

func TestParseReplies(t *testing.T) {
	tests := []struct {
		name    string
		replies []string
		exp     Terminal
	}{
		{
			"kitty 0.50",
			[]string{"\x1b_Gi=31;OK\x1b\\", "\x1b_Gi=32,e=a;OK\x1b\\", "\x1bP>|kitty(0.50.0)\x1b\\", "\x1b[6;20;10t", "\x1b[?62;22c"},
			Terminal{Protocols: []Protocol{Kitty}, Name: "kitty", Version: "0.50.0", Cell: image.Pt(10, 20), KittyAnimation: true},
		},
		{
			// a feature query that the terminal answers without e has no
			// animation
			"no animation",
			[]string{"\x1b_Gi=31;OK\x1b\\", "\x1b_Gi=32;OK\x1b\\", "\x1b[?62c"},
			Terminal{Protocols: []Protocol{Kitty}},
		},
		{
			"kitty error",
			[]string{"\x1b_Gi=31;ENOTSUPPORTED:no\x1b\\", "\x1b[?62c"},
			Terminal{},
		},
		{
			"iterm2 and sixel",
			[]string{"\x1bP>|iTerm2 3.5.14\x1b\\", "\x1b[?1;0;256S", "\x1b[?62;4;22c"},
			Terminal{Protocols: []Protocol{ITerm, Sixel}, Name: "iTerm2", Version: "3.5.14", SixelColors: 256},
		},
		{
			// the first attribute is the terminal id, which can be 4
			"vt132",
			[]string{"\x1b[?4;6c"},
			Terminal{},
		},
		{
			// XTSMGRAPHICS reports a failure with a status other than 0
			"sixel colors failed",
			[]string{"\x1b[?1;3;0S", "\x1b[?62;4c"},
			Terminal{Protocols: []Protocol{Sixel}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var replies [][]byte
			for _, r := range test.replies {
				replies = append(replies, []byte(r))
			}
			if got := parseReplies(replies); !reflect.DeepEqual(got, test.exp) {
				t.Errorf("expected %+v, got: %+v", test.exp, got)
			}
		})
	}
}

func TestSplitTermName(t *testing.T) {
	tests := []struct {
		s, name, version string
	}{
		{"kitty(0.49.2)", "kitty", "0.49.2"},
		{"ghostty 1.4.0-arch1", "ghostty", "1.4.0-arch1"},
		{"WezTerm 20240203-110809-5046fc22", "WezTerm", "20240203-110809-5046fc22"},
		{"iTerm2 3.5.14", "iTerm2", "3.5.14"},
		{"foot", "foot", ""},
		{"", "", ""},
	}
	for _, test := range tests {
		if name, version := splitTermName(test.s); name != test.name || version != test.version {
			t.Errorf("%q: expected %q %q, got: %q %q", test.s, test.name, test.version, name, version)
		}
	}
}

func TestEnvProtocols(t *testing.T) {
	tests := []struct {
		env map[string]string
		exp []Protocol
	}{
		{map[string]string{"KITTY_PID": "1", "KITTY_WINDOW_ID": "1"}, []Protocol{Kitty}},
		{map[string]string{"TERM": "xterm-kitty"}, []Protocol{Kitty}},
		{map[string]string{"TERM_PROGRAM": "ghostty"}, []Protocol{Kitty}},
		{map[string]string{"TERM": "mintty"}, []Protocol{ITerm}},
		{map[string]string{"LC_TERMINAL": "iTerm2"}, []Protocol{ITerm}},
		{map[string]string{"TERM_PROGRAM": "WezTerm"}, []Protocol{ITerm}},
		{map[string]string{"TERM": "xterm-256color"}, nil},
	}
	for _, test := range tests {
		if got := envProtocols(func(k string) string { return test.env[k] }); !slices.Equal(got, test.exp) {
			t.Errorf("%v: expected %v, got: %v", test.env, test.exp, got)
		}
	}
}

func TestDetectForced(t *testing.T) {
	for _, s := range []string{"kitty", "iterm", "sixel", "halfblock", "none"} {
		t.Run(s, func(t *testing.T) {
			// the environment also names another terminal, which is ignored
			env := map[string]string{"TERM_GRAPHICS": s, "TERM": "xterm-kitty"}
			term, pending, err := Detect(context.Background(), WithGetenv(func(k string) string { return env[k] }))
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			exp, _ := ParseProtocol(s)
			if term.Protocol != exp || term.Source != SourceForced || len(term.Protocols) != 0 || pending != nil {
				t.Errorf("expected %v forced, got: %+v %q", exp, term, pending)
			}
		})
	}
}

func TestDetectNotATerminal(t *testing.T) {
	// a program whose input and output are not a terminal reads the
	// environment, and asks nothing
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer f.Close()
	tests := []struct {
		env    map[string]string
		exp    Protocol
		source Source
	}{
		{map[string]string{"TERM": "xterm-kitty"}, Kitty, SourceEnv},
		{map[string]string{"TERM": "xterm-256color"}, None, SourceNone},
	}
	for _, test := range tests {
		term, _, err := Detect(context.Background(), WithInput(f), WithOutput(f),
			WithGetenv(func(k string) string { return test.env[k] }))
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if term.Protocol != test.exp || term.Source != test.source {
			t.Errorf("%v: expected %v from %v, got: %v from %v", test.env, test.exp, test.source, term.Protocol, term.Source)
		}
	}
}

func TestDetectCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := Detect(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected %v, got: %v", context.Canceled, err)
	}
}

func TestNameProtocols(t *testing.T) {
	tests := []struct {
		name string
		exp  []Protocol
	}{
		{"kitty", []Protocol{Kitty}},
		{"ghostty", []Protocol{Kitty}},
		{"iTerm2", []Protocol{ITerm}},
		{"WezTerm", []Protocol{ITerm}},
		{"mintty", []Protocol{ITerm}},
		{"foot", nil},
		{"tmux", nil},
	}
	for _, test := range tests {
		if got := nameProtocols(test.name); !slices.Equal(got, test.exp) {
			t.Errorf("%s: expected %v, got: %v", test.name, test.exp, got)
		}
	}
}
