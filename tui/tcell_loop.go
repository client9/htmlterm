// Package tui drives a document.Document interactively against a real
// terminal via tcell.Screen. It is the only part of htmlterm that depends on
// tcell, kept separate so a consumer that only wants one-shot or
// mutate-and-rerender rendering, via htmlterm.New and Render or
// document.Document, never has to pull tcell, and its raw-mode, ioctl, and
// terminfo dependencies, into their binary.
package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/client9/htmlterm/document"
	"github.com/client9/htmlterm/internal/textcell"
	"github.com/gdamore/tcell/v3"
)

// Loop drives a Document interactively against a real terminal via
// tcell.Screen. Screen.Init and EventQ own raw mode, resize, and keyboard,
// mouse, and paste decoding. Run translates the resulting events into
// Document's public dispatch API — DispatchKey, DispatchClick, DispatchWheel,
// SetSize — and repaints after each one. It also repaints after a SetInterval
// or SetTimeout timer fires (timer.go, delivered as an event sent on the same
// EventQ channel), so periodic, non-input-driven updates such as a spinner or
// a live clock repaint too. It depends only on Document's public API, the
// same layer any other caller uses, plus tcell.Screen for everything
// terminal-facing.
type Loop struct {
	doc    *document.Document
	screen tcell.Screen

	timers      map[timerID]*timerState
	nextTimerID timerID

	quit bool

	// pasting and pasteBuf accumulate a bracketed paste's content. tcell
	// delivers the pasted text as ordinary EventKeys, as if typed, bracketed
	// by an EventPaste{Start: true} and EventPaste{Start: false} pair, rather
	// than handing the whole string over at once. See input.go's inputParser,
	// where keyPasteStart and keyPasteEnd become EventPaste and everything
	// between stays EventKey. While pasting is true, Run buffers each key via
	// pasteKeyText instead of dispatching it as a normal "keydown", so a
	// paste never triggers per-key default actions along the way, such as Tab
	// moving focus or Enter submitting. Only the final DispatchPaste call,
	// once EventPaste{Start: false} arrives, acts on the accumulated text as
	// one unit, mirroring a real "paste" event's single-shot semantics.
	pasting  bool
	pasteBuf strings.Builder
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
// directly. It is the seam tests use to substitute a vt.MockTerm-backed
// Screen (see cellbridge_test.go's newTestScreen) for a real terminal.
func newLoopWithScreen(doc *document.Document, screen tcell.Screen) *Loop {
	return &Loop{
		doc:    doc,
		screen: screen,
		timers: make(map[timerID]*timerState),
	}
}

// Run initializes the screen, meaning raw mode and mouse reporting, and
// repaints doc after every keyboard and mouse event, after every fired timer
// (SetInterval and SetTimeout, timer.go), and after every terminal resize,
// until Ctrl-C is read or the screen's event stream ends, which happens when
// Screen.EventQ closes, as it does after Fini. The terminal is always
// restored to its original state before Run returns, even on error, via a
// deferred Screen.Fini. tcell.Screen owns the whole terminal from Init
// onward, so unlike the previous DSR- and originRow-based Loop, there is no
// inline or preserve-scrollback mode: this is a full-screen-owning TUI.
func (l *Loop) Run() error {
	if err := l.screen.Init(); err != nil {
		return err
	}
	// Deferred LIFO: stopAllTimers, registered second, runs before Fini,
	// registered first. Every timer's forwarding goroutine observes its
	// done channel closed, and stops sending to Screen.EventQ, before the
	// screen and its event queue are torn down.
	defer l.screen.Fini()
	defer l.stopAllTimers()
	l.screen.EnableMouse(tcell.MouseButtonEvents)
	l.screen.EnablePaste()

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
			if l.pasting {
				// Accumulate rather than dispatch; see pasteBuf's doc
				// comment on Loop. The paste is acted on as one unit once
				// EventPaste{Start: false} arrives, below.
				if text, ok := pasteKeyText(ev); ok {
					l.pasteBuf.WriteString(text)
				}
				continue
			}
			if ev.Key() == tcell.KeyCtrlC {
				return nil
			}
			if ev.Key() == tcell.KeyCtrlX {
				// Check the clipboard BEFORE dispatching. DispatchCut's
				// default action removes the text as part of dispatch, so
				// cutting on a terminal with no clipboard support, meaning no
				// OSC 52, would destroy it with nowhere for it to go and no
				// undo. And with a collapsed caret a cut takes the whole
				// field, not just a selection.
				if !l.screen.HasClipboard() {
					break
				}
				if text, ok := l.doc.DispatchCut(); ok {
					l.screen.SetClipboard([]byte(text))
				}
				break
			}
			key, ok := keyName(ev)
			if !ok {
				continue
			}
			l.doc.DispatchKey(key, keyModifiers(ev))

		case *tcell.EventPaste:
			if ev.Start() {
				l.pasting = true
				l.pasteBuf.Reset()
				continue // nothing to repaint yet: the paste has no content until it ends
			}
			l.pasting = false
			l.doc.DispatchPaste(l.pasteBuf.String())

		case *tcell.EventMouse:
			col, row := ev.Position()
			buttons := ev.Buttons()
			switch {
			case buttons&tcell.ButtonPrimary != 0:
				l.doc.DispatchClick(row, col, modifiers(ev.Modifiers()))
			case buttons&(tcell.WheelUp|tcell.WheelDown|tcell.WheelLeft|tcell.WheelRight) != 0:
				dx, dy := wheelDelta(buttons)
				if ev.Modifiers()&tcell.ModShift != 0 && dy != 0 && dx == 0 {
					// Shift plus a vertical wheel is the common browser and
					// terminal convention for horizontal scroll, a fallback
					// for terminals and mice that never report WheelLeft or
					// WheelRight directly. tcell defines those constants, but
					// plenty of real wheel hardware and terminal reporting
					// only ever sends WheelUp and WheelDown.
					dx, dy = dy, 0
				}
				l.doc.DispatchWheel(row, col, dx, dy)
			default:
				continue // ignored mouse report (release, drag, other button)
			}

		case *tcell.EventResize:
			w, h := ev.Size()
			l.doc.SetSize(w, h)
			// No default action to prevent; see event.go's Event doc
			// comment on "submit". htmlterm has no re-layout concept of
			// its own beyond what SetSize just did, and a listener reacts
			// to the new size via Document.Size and Rect.
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
	return nil // EventQ closed: the screen was finalized from elsewhere
}

// Quit requests that Run return after the event currently being handled
// finishes. It is the programmatic equivalent of the user pressing Ctrl-C.
// Like SetInterval and SetTimeout callbacks, it's meant to be called from
// Run's own goroutine, for instance from inside a Document event listener
// reacting to a "quit" command typed into the app, matching the package's
// single-goroutine-mutates-everything contract (see CLAUDE.md's "no locking
// in the interactive layer" invariant). There is no synchronization on the
// quit flag. It skips the final repaint, same as the existing Ctrl-C path,
// since the screen is about to be torn down anyway. A no-op if Run has
// already returned or hasn't started.
func (l *Loop) Quit() {
	l.quit = true
}

// keyName maps a tcell.EventKey to htmlterm's existing DispatchKey
// vocabulary (docs/INTERACTIVE.md): a single printable rune as a UTF-8 string,
// or a named key from a fixed set — "Enter", "Backspace", "Delete", "Tab",
// "Escape", "Home", "End", "ArrowUp"/"Down"/"Left"/"Right", and
// "PageUp"/"PageDown". ok is false for anything outside that vocabulary,
// such as function keys and modifier-only events, which the caller ignores.
// That is the same restricted-subset stance the previous hand-rolled decoder
// took.
//
// "Home", "End", and "Delete" are needed for DispatchKey's caret and
// selection default actions (see docs/proposals/CARET_SELECTION.md); without
// them, those key presses never reach Document.DispatchKey at all. Shift+Tab
// maps to "Tab" as well, since tcell reports it as its own KeyBacktab code
// under legacy keyboard reporting. The Shift that makes DispatchKey walk the
// tab order backwards is supplied by keyModifiers, not by this function.
func keyName(ev *tcell.EventKey) (key string, ok bool) {
	switch ev.Key() {
	case tcell.KeyEnter:
		return "Enter", true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return "Backspace", true
	case tcell.KeyDelete:
		return "Delete", true
	case tcell.KeyTab:
		return "Tab", true
	case tcell.KeyBacktab:
		// Shift+Tab. Legacy keyboard reporting gives it its own key code
		// rather than Tab-with-ModShift, so the Shift half is re-attached
		// separately; see keyModifiers.
		return "Tab", true
	case tcell.KeyEsc:
		return "Escape", true
	case tcell.KeyHome:
		return "Home", true
	case tcell.KeyEnd:
		return "End", true
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

// keyModifiers is keyName's companion: the document.Modifiers to dispatch
// alongside the key name it returned. Normally that's just tcell's own
// reported modifier mask (see modifiers), but tcell.KeyBacktab is a key code
// that *means* Shift+Tab without necessarily carrying ModShift, so the Shift
// flag is re-attached here. Otherwise DispatchKey sees a plain "Tab" and
// moves focus forward, making Shift+Tab indistinguishable from Tab.
func keyModifiers(ev *tcell.EventKey) document.Modifiers {
	mods := modifiers(ev.Modifiers())
	if ev.Key() == tcell.KeyBacktab {
		mods.Shift = true
	}
	return mods
}

// pasteKeyText decodes one EventKey received while Loop.pasting is true back
// into the literal text it represents. tcell delivers a bracketed paste's
// content as a stream of ordinary key events (see pasteBuf's doc comment on
// Loop), so reconstructing the pasted string means undoing that decoding
// rather than reading it off the EventPaste itself. Printable runes map back
// to their literal characters, as do the two whitespace keys a paste can
// plausibly contain: an embedded newline, decoded as Enter, and an embedded
// tab. ok is false for anything else, such as arrow and function keys, which
// shouldn't appear inside a paste but are silently dropped rather than
// corrupting the buffer if a terminal ever sends one anyway.
func pasteKeyText(ev *tcell.EventKey) (string, bool) {
	switch ev.Key() {
	case tcell.KeyRune:
		return ev.Str(), true
	case tcell.KeyEnter:
		return "\n", true
	case tcell.KeyTab:
		return "\t", true
	default:
		return "", false
	}
}

// modifiers translates tcell's raw modifier bitmask, as reported on both
// EventKey and EventMouse, into document.Modifiers. It is the one place this
// package's tcell dependency leaks a modifier-key concept into Document's
// vocabulary, mirroring keyName's job for key names.
func modifiers(mod tcell.ModMask) document.Modifiers {
	return document.Modifiers{
		Shift: mod&tcell.ModShift != 0,
		Ctrl:  mod&tcell.ModCtrl != 0,
		Alt:   mod&tcell.ModAlt != 0,
		Meta:  mod&tcell.ModMeta != 0,
	}
}

// wheelDelta translates tcell's wheel button bits into a (deltaX, deltaY)
// pair matching a real WheelEvent's deltaX and deltaY. Positive deltaY is
// "down" or "away" and positive deltaX is "right", matching DispatchWheel's
// own convention, unchanged from its pre-horizontal vertical-only delta sign.
// More than one wheel bit can't be set in practice, since tcell reports one
// wheel impulse at a time, but the bits are checked independently rather than
// in a single switch so nothing breaks if that ever changes.
func wheelDelta(buttons tcell.ButtonMask) (dx, dy int) {
	if buttons&tcell.WheelUp != 0 {
		dy -= 1
	}
	if buttons&tcell.WheelDown != 0 {
		dy += 1
	}
	if buttons&tcell.WheelLeft != 0 {
		dx -= 1
	}
	if buttons&tcell.WheelRight != 0 {
		dx += 1
	}
	return dx, dy
}

// paint renders doc and writes it into the screen via the ANSI-line-to-cell
// bridge (cellbridge.go's paintLines), positions the terminal's real cursor
// on the focused element if one is focused and currently visible
// (Element.ScrollVisible), and calls Screen.Show to let tcell's own
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

// caretLineCol locates rune offset caret within value's "\n"-delimited
// lines, returning the zero-based line index and the column within that
// line. It is focusCursorPos's <textarea> helper, mirroring document.go's own
// lineBounds and DispatchKey caret math (see
// docs/proposals/CARET_SELECTION.md) rather than reimplementing it
// differently here. caret is assumed already clamped to
// [0, len(value) in runes], which holds for anything read from
// Element.SelectionEnd.
func caretLineCol(value string, caret int) (line, col int) {
	lines := strings.Split(value, "\n")
	remaining := caret
	for i, l := range lines {
		n := utf8.RuneCountInString(l)
		if i == len(lines)-1 || remaining <= n {
			return i, remaining
		}
		remaining -= n + 1 // +1 consumes this line's own trailing "\n"
	}
	return len(lines) - 1, utf8.RuneCountInString(lines[len(lines)-1])
}

// focusCursorPos reports where the terminal's real cursor should land for
// doc's currently focused element, in doc's own coordinate space. Unlike
// the previous originRow-based version, tcell.Screen owns the whole
// terminal from (0,0), so no origin-row offset is needed.
//
// For a text-like <input> or <textarea> it lands at the element's current
// caret, Element.SelectionEnd, the same edge a real UA parks its blinking
// caret at even when a range is selected (see
// docs/proposals/CARET_SELECTION.md), clamped inside the element's own box.
// For any other focusable element, such as a checkbox, radio, or button, it
// lands on the box's first column. ok is false if nothing is focused, the
// focused element has no recorded Rect, or it's currently scrolled out of
// view by one of its scrollable ancestors (Element.ScrollVisible).
func focusCursorPos(doc *document.Document) (row, col int, ok bool) {
	el := doc.FocusedElement()
	if el == nil {
		return 0, 0, false
	}
	if !el.ScrollVisible() {
		return 0, 0, false
	}
	rect, ok := el.Rect()
	if !ok {
		return 0, 0, false
	}
	row, col = rect.Row, rect.Col
	if el.IsTextEntry() {
		caret := el.SelectionEnd()
		// A <textarea>'s value can span multiple lines, since DispatchKey's
		// Enter default action can insert "\n" anywhere rather than only
		// appending it (see document.go's replaceSelection). So the caret's
		// row and column need locating within the right "\n"-delimited line,
		// via caretLineCol, not assumed to be the last one. This doesn't
		// account for a single line getting further wrapped by its own width
		// (wordWrapTokens, block.go), an accepted approximation gap, and a
		// narrower one than not handling embedded newlines at all.
		if strings.ToLower(el.TagName()) == "textarea" {
			// doc.ContentOffset and ContentOffsetX (see their doc comments)
			// are the row and column shifts from rect.Row and rect.Col to
			// this textarea's own first content cell: border-top plus
			// padding-top, and border-left plus padding-left. They are
			// needed here because Rect alone is the full border box (see
			// Rect's doc comment) and can't say where content starts within
			// it. The column half matters for the UA stylesheet's own
			// default <textarea> styling, which draws a border and one
			// column of padding on each side. Without it the cursor lands
			// two columns left of the real caret, on the border itself.
			value := el.Value()
			line, lineCol := caretLineCol(value, caret)
			offset, _ := doc.ContentOffset(el)
			offsetX, _ := doc.ContentOffsetX(el)
			row = rect.Row + offset + line
			// lineCol is a rune offset; the cursor needs a column, and the
			// two only coincide for single-width text (see
			// textcell.ColumnForRuneIndex).
			col = rect.Col + offsetX + textcell.ColumnForRuneIndex(strings.Split(value, "\n")[line], lineCol)
			if maxRow := rect.Row + rect.Height - 1; row > maxRow {
				row = maxRow
			}
		} else {
			// A text-like <input> is the only tag reaching this branch:
			// textarea takes the branch above, and checkbox, radio, submit,
			// button, reset, and hidden are excluded from IsTextEntry
			// entirely. It renders its value plain (formcontrol.go's
			// inputDisplayText), so the box's first column is the value's
			// own first character. No offset is needed, but caret is still a
			// rune offset that has to be measured out in columns.
			col = rect.Col + textcell.ColumnForRuneIndex(el.Value(), caret)
		}
		if maxCol := rect.Col + rect.Width - 1; col > maxCol {
			col = maxCol
		}
	}
	return row, col, true
}
