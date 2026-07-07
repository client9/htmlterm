package htmlterm

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"
)

// Loop drives a Document interactively against a real terminal: it puts in
// into raw mode, decodes keyboard and mouse input off it, dispatches the
// resulting events to doc (Document.DispatchKey/DispatchClick), and repaints
// out after every dispatch — or after a SetInterval/SetTimeout timer fires
// (timer.go), so periodic, non-input-driven updates (a spinner, a live
// clock) repaint too. It depends only on Document's public API — Render,
// DispatchClick, DispatchKey — the same layer any other caller uses.
type Loop struct {
	doc *Document
	in  *os.File
	out io.Writer

	// autoWidth/autoHeight remember whether doc's Width/Height were
	// SizeAutomatic when NewLoop was called — captured once and separately
	// from doc's own stored Options, because applyTerminalSize's first
	// resolution overwrites SizeAutomatic (0) with a concrete number via
	// Document.SetSize; re-deriving "is this axis automatic" from
	// doc.Size() on every call would see that concrete number from the
	// second call onward and wrongly conclude tracking should stop.
	autoWidth  bool
	autoHeight bool

	timers      map[timerID]*timerState
	nextTimerID timerID
	timerCh     chan timerFire
}

// NewLoop returns a Loop that reads input from in (typically os.Stdin — a
// real file is required so raw mode can be applied via its file descriptor)
// and writes rendered frames to out (typically os.Stdout). Timers
// (SetInterval/SetTimeout) may be registered any time after construction,
// including before Run is called. Whether doc's Width/Height track the
// terminal (SizeAutomatic) is fixed as of this call — see applyTerminalSize.
func NewLoop(doc *Document, in *os.File, out io.Writer) *Loop {
	l := &Loop{
		doc:     doc,
		in:      in,
		out:     out,
		timers:  make(map[timerID]*timerState),
		timerCh: make(chan timerFire),
	}
	if doc != nil {
		width, height := doc.Size()
		l.autoWidth = width == SizeAutomatic
		l.autoHeight = height == SizeAutomatic
	}
	return l
}

// writeFrame writes doc.Render()'s output with every "\n" widened to
// "\r\n". Raw mode (enterRawMode, terminal.go) disables OPOST, so the
// terminal no longer supplies the carriage return a bare line feed would
// normally imply — without this, each line of a frame lands one column to
// the right of the last, and repaints drift diagonally down the screen
// instead of overwriting in place.
func writeFrame(w io.Writer, frame string) error {
	_, err := io.WriteString(w, strings.ReplaceAll(frame, "\n", "\r\n"))
	return err
}

// inputMsg is one decodeEvent result (or the error that ended decoding),
// relayed by Run's input-reading goroutine onto a channel so Run's main loop
// can select between it and a timer firing (timerFire, timer.go) rather than
// blocking solely on terminal input.
type inputMsg struct {
	ev  inputEvent
	err error
}

// Run puts the terminal into raw mode, enables SGR mouse reporting, and
// repaints doc after every keyboard/mouse event, after every fired timer
// (SetInterval/SetTimeout, timer.go), and after every terminal resize
// (SIGWINCH — see applyTerminalSize), until Ctrl-C (\x03) is read or in
// reaches EOF/an error. The terminal is always restored to its original mode
// and mouse reporting disabled before Run returns, even on error.
//
// SIGWINCH handling ties Run to POSIX-like platforms specifically (syscall
// has no such signal on Windows) — a stronger, compile-time version of the
// rest of the package's already-POSIX-oriented, Windows-unverified stance
// (see terminal.go's raw-mode primitives).
func (l *Loop) Run() error {
	restore, err := enterRawMode(int(l.in.Fd()))
	if err != nil {
		return err
	}
	defer func() { _ = restore() }()

	if _, err := io.WriteString(l.out, enableMouse()); err != nil {
		return err
	}
	defer func() { _, _ = io.WriteString(l.out, disableMouse()) }()

	r := bufio.NewReader(l.in)

	if _, err := io.WriteString(l.out, "\r"); err != nil {
		return err
	}
	originRow, err := queryCursorRow(r, l.out)
	if err != nil {
		return err
	}

	l.applyTerminalSize()

	if err := l.paint(originRow); err != nil {
		return err
	}

	// Input is decoded on its own goroutine and relayed here over a channel,
	// rather than Run blocking directly on decodeEvent, so the loop below
	// can select between "input arrived" and "a timer fired" — every
	// Document mutation and every paint() call still happens on this one
	// goroutine, so no locking is needed anywhere (see timer.go).
	inputCh := make(chan inputMsg)
	go func() {
		for {
			ev, err := decodeEvent(r)
			inputCh <- inputMsg{ev: ev, err: err}
			if err != nil {
				return
			}
		}
	}()

	// resizeCh relays SIGWINCH. Registered unconditionally, regardless of
	// whether doc's Width/Height are actually SizeAutomatic: a host doing
	// its own multi-pane sizing (several Documents composited outside of
	// any single Loop's control — see INTERACTIVE.md) still wants a
	// "resize" event to react to, even if this particular Document's own
	// size is fixed or SizeNatural. applyTerminalSize itself is the only
	// part that's conditional (a no-op if neither dimension is automatic).
	// Buffered so a signal arriving while Run is busy elsewhere isn't
	// dropped, per os/signal's documented recommendation.
	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)
	defer signal.Stop(resizeCh)

	for {
		select {
		case msg := <-inputCh:
			if msg.err != nil {
				if msg.err == io.EOF {
					return nil
				}
				return msg.err
			}

			switch ev := msg.ev; {
			case ev.kind == keyEvent && ev.key == "\x03":
				return nil
			case ev.kind == keyEvent && ev.key != "":
				l.doc.DispatchKey(ev.key)
			case ev.kind == clickEvent:
				l.doc.DispatchClick(ev.row-originRow, ev.col)
			default:
				continue // an ignored mouse report (release, drag, other button)
			}

		case fire := <-l.timerCh:
			l.handleTimerFire(fire)

		case <-resizeCh:
			l.applyTerminalSize()
			// No default action to prevent (see event.go's Event doc
			// comment on "submit") — htmlterm has no re-layout concept of
			// its own beyond what applyTerminalSize just did; a listener
			// reacts to the new size via Document.Size/Rect, e.g. a host
			// with several Documents recomputing each pane's own extent.
			l.doc.dispatch(l.doc.doc, "resize", "")
		}

		if err := l.paint(originRow); err != nil {
			return err
		}
	}
}

// applyTerminalSize resolves whichever of Width/Height was SizeAutomatic at
// NewLoop time (l.autoWidth/l.autoHeight) against l.in's current terminal
// size and installs the result via Document.SetSize — called once before
// Run's first paint, and again on every SIGWINCH. A no-op if neither was
// ever automatic. Deliberately consults l.autoWidth/l.autoHeight rather than
// re-deriving "is this automatic" from doc.Size()'s current value: the very
// first resolution overwrites SizeAutomatic (0) with a concrete number, so
// checking the current value again on the next call would see that number
// and wrongly conclude tracking should stop. A failed size query (e.g. in
// isn't actually a terminal) leaves an automatic Width at a safe 80-column
// fallback, matching cmd/main.go's CLI default; an automatic Height simply
// keeps its last-known value, already inert/unconstrained if that's still 0
// (see Options.Height) — no separate fallback needed.
func (l *Loop) applyTerminalSize() {
	if !l.autoWidth && !l.autoHeight {
		return
	}
	width, height := l.doc.Size()
	w, h, err := getTerminalSize(int(l.in.Fd()))
	if l.autoWidth {
		if err != nil || w <= 0 {
			w = 80
		}
		width = w
	}
	if l.autoHeight && err == nil && h > 0 {
		height = h
	}
	l.doc.SetSize(width, height)
}

// paint renders doc, redraws it at the document's fixed screen position
// (originRow, column 0), and — if an element is focused — leaves the
// terminal's real cursor sitting on it, so the user can see which control is
// active rather than the cursor being parked wherever the frame happened to
// end. Uses absolute cursor positioning (CUP, "\x1b[{row};{col}H") anchored
// at originRow throughout, rather than tracking a relative "move up by the
// previous frame's line count": that only needs one reference point instead
// of bookkeeping how far the cursor drifted since the last paint (e.g. from
// resting on a focused field partway through the frame, not at the bottom).
func (l *Loop) paint(originRow int) error {
	frame, err := l.doc.Render()
	if err != nil {
		return err
	}
	if err := cup(l.out, originRow, 0); err != nil {
		return err
	}
	if _, err := io.WriteString(l.out, "\x1b[J"); err != nil {
		return err
	}
	if err := writeFrame(l.out, frame); err != nil {
		return err
	}
	if row, col, ok := focusCursorPos(l.doc, originRow); ok {
		return cup(l.out, row, col)
	}
	return nil
}

// cup moves the terminal's cursor to the given 0-indexed row/col via CUP.
func cup(w io.Writer, row, col int) error {
	_, err := fmt.Fprintf(w, "\x1b[%d;%dH", row+1, col+1)
	return err
}

// focusCursorPos reports where the terminal's real cursor should land for
// doc's currently focused element, in absolute screen coordinates (relative
// to originRow, the terminal row doc's own row 0 occupies). For a text-like
// input/textarea it lands just past the end of the current value (an
// insertion-point approximation, clamped inside the element's own box, e.g.
// "[value]"); for any other focusable element (checkbox, radio, button) it
// lands on the box's first column. ok is false if nothing is focused or the
// focused element has no recorded Rect.
func focusCursorPos(doc *Document, originRow int) (row, col int, ok bool) {
	el := doc.FocusedElement()
	if el == nil {
		return 0, 0, false
	}
	rect, ok := doc.Rect(el)
	if !ok {
		return 0, 0, false
	}
	col = rect.Col
	if isTextEntry(el.node) {
		col = rect.Col + 1 + utf8.RuneCountInString(el.Value())
		if maxCol := rect.Col + rect.Width - 1; col > maxCol {
			col = maxCol
		}
	}
	return originRow + rect.Row, col, true
}

// queryCursorRow writes a cursor-position query (DSR, "\x1b[6n") to out and
// reads the terminal's reply ("\x1b[{row};{col}R") off r, returning the
// 0-indexed row the cursor is currently on — the terminal-absolute row a
// Document's own row 0 will occupy once Run writes its first frame.
func queryCursorRow(r *bufio.Reader, out io.Writer) (int, error) {
	if _, err := io.WriteString(out, "\x1b[6n"); err != nil {
		return 0, err
	}

	if b, err := r.ReadByte(); err != nil || b != 0x1b {
		return 0, fmt.Errorf("htmlterm: unexpected cursor position report")
	}
	if b, err := r.ReadByte(); err != nil || b != '[' {
		return 0, fmt.Errorf("htmlterm: unexpected cursor position report")
	}

	var body strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		if b == 'R' {
			break
		}
		body.WriteByte(b)
	}

	parts := strings.SplitN(body.String(), ";", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("htmlterm: malformed cursor position report %q", body.String())
	}
	row, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("htmlterm: malformed cursor position report %q", body.String())
	}
	return row - 1, nil
}
