// Package tui drives a document.Document interactively against a real
// terminal via tcell.Screen — the only part of htmlterm that depends on
// tcell, kept separate so a consumer that only wants one-shot or
// mutate-and-rerender rendering (htmlterm.New/Render, document.Document)
// never has to pull tcell (and its raw-mode/ioctl/terminfo dependencies)
// into their binary.
package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/client9/htmlterm/document"
	"github.com/gdamore/tcell/v3"
)

// Loop drives a Document interactively against a real terminal via
// tcell.Screen: Screen.Init/EventQ own raw mode, resize, and
// keyboard/mouse/paste decoding; Run translates the resulting events into
// Document's public dispatch API (DispatchKey/DispatchClick/DispatchWheel/
// SetSize) and repaints after each one — or after a SetInterval/SetTimeout
// timer fires (timer.go, delivered as an event sent on the same EventQ
// channel), so periodic, non-input-driven updates (a spinner, a live
// clock) repaint too. It depends only on Document's public API — the same
// layer any other caller uses — plus tcell.Screen for everything
// terminal-facing.
type Loop struct {
	doc    *document.Document
	screen tcell.Screen

	timers      map[timerID]*timerState
	nextTimerID timerID

	quit bool
}

// NewLoop returns a Loop backed by a new tcell.Screen for the process's
// controlling terminal. Timers (SetInterval/SetTimeout) may be registered
// any time after construction, including before Run is called.
func NewLoop(doc *document.Document) (*Loop, error) {
	screen, err := tcell.NewScreen()
	if err != nil {
		return nil, err
	}
	return newLoopWithScreen(doc, screen), nil
}

// newLoopWithScreen is NewLoop's constructor with the Screen injected
// directly — the seam tests use to substitute a vt.MockTerm-backed Screen
// (see cellbridge_test.go's newTestScreen) instead of a real terminal.
func newLoopWithScreen(doc *document.Document, screen tcell.Screen) *Loop {
	return &Loop{
		doc:    doc,
		screen: screen,
		timers: make(map[timerID]*timerState),
	}
}

// Run initializes the screen (raw mode, mouse reporting) and repaints doc
// after every keyboard/mouse event, after every fired timer
// (SetInterval/SetTimeout, timer.go), and after every terminal resize,
// until Ctrl-C is read or the screen's event stream ends (Screen.EventQ
// closing, e.g. after Fini). The terminal is always restored to its
// original state before Run returns, even on error (Screen.Fini,
// deferred) — tcell.Screen owns the whole terminal from Init onward, so
// unlike the previous DSR/originRow-based Loop, there is no inline/
// preserve-scrollback mode: this is a full-screen-owning TUI.
func (l *Loop) Run() error {
	if err := l.screen.Init(); err != nil {
		return err
	}
	// Deferred LIFO: stopAllTimers (registered second) runs before Fini
	// (registered first) — every timer's forwarding goroutine observes its
	// done channel closed, and stops sending to Screen.EventQ, before the
	// screen (and its event queue) is torn down.
	defer l.screen.Fini()
	defer l.stopAllTimers()
	l.screen.EnableMouse(tcell.MouseButtonEvents)

	width, height := l.screen.Size()
	l.doc.SetSize(width, height)

	if err := l.paint(); err != nil {
		return err
	}

	for ev := range l.screen.EventQ() {
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if !ev.Pressed() {
				continue // ignore key-release events (see keyName's press-only vocabulary)
			}
			if ev.Key() == tcell.KeyCtrlC {
				return nil
			}
			key, ok := keyName(ev)
			if !ok {
				continue
			}
			l.doc.DispatchKey(key)

		case *tcell.EventMouse:
			col, row := ev.Position()
			switch {
			case ev.Buttons()&tcell.ButtonPrimary != 0:
				l.doc.DispatchClick(row, col)
			case ev.Buttons()&tcell.WheelUp != 0:
				l.doc.DispatchWheel(row, col, -1)
			case ev.Buttons()&tcell.WheelDown != 0:
				l.doc.DispatchWheel(row, col, 1)
			default:
				continue // ignored mouse report (release, drag, other button)
			}

		case *tcell.EventResize:
			w, h := ev.Size()
			l.doc.SetSize(w, h)
			// No default action to prevent (see event.go's Event doc
			// comment on "submit") — htmlterm has no re-layout concept of
			// its own beyond what SetSize just did; a listener reacts to
			// the new size via Document.Size/Rect.
			l.doc.DispatchResize()

		case *timerFireEvent:
			l.handleTimerFire(ev.id)

		default:
			continue // an event kind we don't act on (focus, paste, etc.)
		}

		if l.quit {
			return nil
		}

		if err := l.paint(); err != nil {
			return err
		}
	}
	return nil // EventQ closed (screen finalized from elsewhere)
}

// Quit requests that Run return after the event currently being handled
// finishes — the programmatic equivalent of the user pressing Ctrl-C. Like
// SetInterval/SetTimeout callbacks, it's meant to be called from Run's own
// goroutine (e.g. from inside a Document event listener reacting to a "quit"
// command typed into the app), matching the package's single-goroutine-
// mutates-everything contract (see CLAUDE.md's "no locking in the
// interactive layer" invariant) — there is no synchronization on the quit
// flag. Skips the final repaint (same as the existing Ctrl-C path) since the
// screen is about to be torn down anyway. A no-op if Run has already
// returned or hasn't started.
func (l *Loop) Quit() {
	l.quit = true
}

// keyName maps a tcell.EventKey to htmlterm's existing DispatchKey
// vocabulary (docs/INTERACTIVE.md): a single printable rune as a UTF-8 string,
// or a named key from a fixed set ("Enter", "Backspace", "Tab", "Escape",
// "ArrowUp"/"Down"/"Left"/"Right", "PageUp"/"PageDown"). ok is false for
// anything outside that vocabulary (function keys, modifier-only events,
// etc.), which the caller simply ignores — the same restricted-subset
// stance the previous hand-rolled decoder took.
func keyName(ev *tcell.EventKey) (key string, ok bool) {
	switch ev.Key() {
	case tcell.KeyEnter:
		return "Enter", true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return "Backspace", true
	case tcell.KeyTab:
		return "Tab", true
	case tcell.KeyEsc:
		return "Escape", true
	case tcell.KeyUp:
		return "ArrowUp", true
	case tcell.KeyDown:
		return "ArrowDown", true
	case tcell.KeyLeft:
		return "ArrowLeft", true
	case tcell.KeyRight:
		return "ArrowRight", true
	case tcell.KeyPgUp:
		return "PageUp", true
	case tcell.KeyPgDn:
		return "PageDown", true
	case tcell.KeyRune:
		return ev.Str(), true
	default:
		return "", false
	}
}

// paint renders doc and writes it into the screen via the ANSI-line-to-cell
// bridge (cellbridge.go's paintLines), positions the terminal's real cursor
// on the focused element if one is focused and currently visible
// (Document.ScrollVisible), and calls Screen.Show to let tcell's own
// diffing renderer decide what actually needs writing to the terminal.
func (l *Loop) paint() error {
	frame, err := l.doc.Render()
	if err != nil {
		return err
	}
	paintLines(l.screen, splitLines(frame))

	if row, col, ok := focusCursorPos(l.doc); ok {
		l.screen.ShowCursor(col, row)
	} else {
		l.screen.HideCursor()
	}
	l.screen.Show()
	return nil
}

// splitLines splits frame on "\n" the way Document.Render's output is
// structured (one rendered line per element), for paintLines to place one
// per screen row.
func splitLines(frame string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(frame); i++ {
		if frame[i] == '\n' {
			lines = append(lines, frame[start:i])
			start = i + 1
		}
	}
	lines = append(lines, frame[start:])
	return lines
}

// focusCursorPos reports where the terminal's real cursor should land for
// doc's currently focused element, in doc's own coordinate space — unlike
// the previous originRow-based version, tcell.Screen owns the whole
// terminal from (0,0), so no origin-row offset is needed. For a text-like
// input/textarea it lands just past the end of the current value (an
// insertion-point approximation, clamped inside the element's own box,
// e.g. "[value]"); for any other focusable element (checkbox, radio,
// button) it lands on the box's first column. ok is false if nothing is
// focused, the focused element has no recorded Rect, or it's currently
// scrolled out of view by one of its scrollable ancestors
// (Document.ScrollVisible).
func focusCursorPos(doc *document.Document) (row, col int, ok bool) {
	el := doc.FocusedElement()
	if el == nil {
		return 0, 0, false
	}
	if !doc.ScrollVisible(el) {
		return 0, 0, false
	}
	rect, ok := el.Rect()
	if !ok {
		return 0, 0, false
	}
	row, col = rect.Row, rect.Col
	if el.IsTextEntry() {
		value := el.Value()
		// A <textarea>'s value can span multiple lines (DispatchKey's Enter
		// default action appends "\n" — see document.go), and every
		// dispatched edit only ever appends at the end, so the insertion
		// point is always the end of the last "\n"-delimited line: advance
		// row by the number of embedded newlines, and measure column from
		// that last line alone rather than the whole value's total rune
		// count. This doesn't account for a single line getting further
		// wrapped by its own width (wordWrapTokens, block.go) — an accepted
		// narrower approximation gap than not handling embedded newlines at
		// all.
		if strings.ToLower(el.TagName()) == "textarea" {
			// doc.ContentOffset (see its doc comment) is the row shift from
			// rect.Row down to this textarea's own first content row —
			// border-top plus padding-top — needed here because Rect alone
			// is the full border box (see Rect's doc comment) and can't say
			// where content actually starts within it.
			lines := strings.Split(value, "\n")
			offset, _ := doc.ContentOffset(el)
			row = rect.Row + offset + len(lines) - 1
			col = rect.Col + utf8.RuneCountInString(lines[len(lines)-1])
			if maxRow := rect.Row + rect.Height - 1; row > maxRow {
				row = maxRow
			}
		} else {
			col = rect.Col + utf8.RuneCountInString(value)
		}
		if maxCol := rect.Col + rect.Width - 1; col > maxCol {
			col = maxCol
		}
	}
	return row, col, true
}
