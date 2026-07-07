package htmlterm

import (
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

// mouseModes are the DEC private modes enabled/disabled around a Loop's run
// to receive left-button-press mouse reports in SGR encoding (see input.go's
// decodeEvent for the wire format this produces).
var mouseModes = []ansi.Mode{ansi.ModeMouseButtonEvent, ansi.ModeMouseExtSgr}

// enableMouse returns the escape sequence that turns on SGR mouse reporting.
func enableMouse() string {
	return ansi.SetMode(mouseModes...)
}

// disableMouse returns the escape sequence that turns SGR mouse reporting
// back off.
func disableMouse() string {
	return ansi.ResetMode(mouseModes...)
}

// enterRawMode puts fd into raw mode (via golang.org/x/term) and returns a
// restore func that undoes it. Raw mode disables ISIG, so the terminal will
// not deliver Ctrl-C as SIGINT while active — Loop.Run watches for the raw
// \x03 byte itself instead (see input.go).
func enterRawMode(fd int) (restore func() error, err error) {
	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	return func() error { return term.Restore(fd, state) }, nil
}
