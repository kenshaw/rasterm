package rasterm

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// inTmux returns true when the program runs in a tmux pane. tmux sets $TMUX
// and $TMUX_PANE in each pane that it starts, and a shell can lose $TMUX, so
// either one is enough. It reads the environment once (D20, D21).
//
// It is a variable so that a test can replace it.
var inTmux = sync.OnceValue(func() bool {
	return os.Getenv("TMUX") != "" || os.Getenv("TMUX_PANE") != ""
})

// tmuxClient is what tmux reports about the terminal that its client runs
// in.
type tmuxClient struct {
	// protocol is the graphics protocol of the terminal, as [tmuxProtocol]
	// returns it.
	protocol string
	// cellWidth and cellHeight are the size of a cell in pixels, or 0 when
	// tmux does not know it.
	cellWidth, cellHeight int
}

// tmuxFormat is the format that [tmuxClientOnce] asks tmux to expand.
const tmuxFormat = "#{client_termtype}|#{allow-passthrough}|#{client_cell_width}|#{client_cell_height}"

// tmuxClientOnce returns what tmux reports about the terminal of its client,
// from one call to tmux. It is the zero value when tmux cannot be asked. It
// asks tmux once (D21, D22).
//
// It is a variable so that a test can replace it.
var tmuxClientOnce = sync.OnceValue(func() tmuxClient {
	args := []string{"display-message", "-p"}
	if pane := os.Getenv("TMUX_PANE"); pane != "" {
		args = append(args, "-t", pane)
	}
	args = append(args, tmuxFormat)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", args...).Output()
	if err != nil {
		return tmuxClient{}
	}
	return parseTmuxClient(strings.TrimSpace(string(out)))
})

// parseTmuxClient reads a line that tmux writes for [tmuxFormat].
func parseTmuxClient(line string) tmuxClient {
	f := strings.Split(line, "|")
	if len(f) != 4 {
		return tmuxClient{}
	}
	w, _ := strconv.Atoi(f[2])
	h, _ := strconv.Atoi(f[3])
	return tmuxClient{
		protocol:   tmuxProtocol(f[0], f[1]),
		cellWidth:  max(w, 0),
		cellHeight: max(h, 0),
	}
}

// tmuxProtocol returns the protocol for the client terminal type typ and
// the value pass of allow-passthrough. The client terminal type is the name
// and the version that the terminal reports to tmux, such as "ghostty 1.3.1"
// or "kitty(0.49.2)". It returns kitty for Kitty and Ghostty and iterm for
// iTerm2 and WezTerm. It returns the empty string for any other terminal, and
// when allow-passthrough is off, because tmux then drops each sequence.
func tmuxProtocol(typ, pass string) string {
	if pass != "on" && pass != "all" {
		return ""
	}
	switch typ = strings.ToLower(typ); {
	case strings.HasPrefix(typ, "kitty"), strings.HasPrefix(typ, "ghostty"):
		return "kitty"
	case strings.HasPrefix(typ, "iterm2"), strings.HasPrefix(typ, "wezterm"):
		return "iterm"
	}
	return ""
}

// The DCS passthrough of tmux. tmux writes what is between tmuxStart and
// tmuxEnd to the outer terminal as it is, after it changes each doubled ESC
// to one ESC. tmux does this only when its allow-passthrough option is on.
const (
	tmuxStart = "\x1bPtmux;"
	tmuxEnd   = "\x1b\\"
)

// The start and the end of the escape sequences of Kitty and iTerm. The data
// between a start and an end is base64 or digits, and holds no ESC, so in
// tmux only the start and the end change.
const (
	apcStart     = "\x1b_G"
	apcEnd       = "\x1b\\"
	apcTmuxStart = tmuxStart + "\x1b\x1b_G"
	apcTmuxEnd   = "\x1b\x1b\\" + tmuxEnd

	oscStart     = "\x1b]1337;File=inline=1:"
	oscEnd       = "\a"
	oscTmuxStart = tmuxStart + "\x1b\x1b]1337;File=inline=1:"
	oscTmuxEnd   = "\a" + tmuxEnd
)

// kittySeq returns the start and the end of a Kitty graphics escape sequence,
// in the DCS passthrough of tmux when tmux is true.
func kittySeq(tmux bool) (string, string) {
	if tmux {
		return apcTmuxStart, apcTmuxEnd
	}
	return apcStart, apcEnd
}

// itermSeq returns the start and the end of an iTerm inline image escape
// sequence, in the DCS passthrough of tmux when tmux is true.
func itermSeq(tmux bool) (string, string) {
	if tmux {
		return oscTmuxStart, oscTmuxEnd
	}
	return oscStart, oscEnd
}
