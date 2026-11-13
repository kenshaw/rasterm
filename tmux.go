package rasterm

import (
	"context"
	"image"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// tmuxClient is what tmux reports about the terminal that its client runs
// in.
type tmuxClient struct {
	// name and version are what the terminal reported to tmux in its answer
	// to XTVERSION, such as "ghostty" and "1.4.0".
	name, version string
	// passthrough is true when allow-passthrough is on. tmux drops each
	// sequence in the DCS passthrough when it is off.
	passthrough bool
	// cell is the size of a cell in pixels, or zero when tmux does not know
	// it.
	cell image.Point
}

// tmuxFormat is the format that [askTmux] asks tmux to expand.
const tmuxFormat = "#{client_termtype}|#{allow-passthrough}|#{client_cell_width}|#{client_cell_height}"

// tmuxTimeout is how long [askTmux] waits for tmux.
const tmuxTimeout = time.Second

// askTmux asks tmux about the terminal of the client of the pane in
// $TMUX_PANE, with one call to tmux. It returns false when tmux cannot be
// asked (D21, D22).
func askTmux(ctx context.Context, getenv func(string) string) (tmuxClient, bool) {
	args := []string{"display-message", "-p"}
	if pane := getenv("TMUX_PANE"); pane != "" {
		args = append(args, "-t", pane)
	}
	args = append(args, tmuxFormat)
	ctx, cancel := context.WithTimeout(ctx, tmuxTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "tmux", args...).Output()
	if err != nil {
		return tmuxClient{}, false
	}
	return parseTmuxClient(strings.TrimSpace(string(out)))
}

// parseTmuxClient reads a line that tmux writes for [tmuxFormat]. It returns
// false when the line does not have the four fields.
func parseTmuxClient(line string) (tmuxClient, bool) {
	f := strings.Split(line, "|")
	if len(f) != 4 {
		return tmuxClient{}, false
	}
	name, version := splitTermName(f[0])
	w, _ := strconv.Atoi(f[2])
	h, _ := strconv.Atoi(f[3])
	if w <= 0 || h <= 0 {
		w, h = 0, 0
	}
	return tmuxClient{
		name:        name,
		version:     version,
		passthrough: f[1] == "on" || f[1] == "all",
		cell:        image.Pt(w, h),
	}, true
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
