package tui

import (
	"sync"
	"testing"
	"time"

	"github.com/client9/htmlterm"
	"github.com/client9/htmlterm/document"
	"github.com/gdamore/tcell/v3"
	"github.com/gdamore/tcell/v3/vt"
)

// newUninitScreen is like newTestScreen (cellbridge_test.go) but leaves the
// Screen un-Init'd and doesn't register a Fini cleanup — for tests that
// exercise Loop.Run itself, since Run calls Init/Fini on the Screen it's
// given, and Init can't be called twice.
func newUninitScreen(t *testing.T, cols, rows int) (tcell.Screen, vt.MockTerm) {
	t.Helper()
	mt := vt.NewMockTerm(vt.MockOptColors(1 << 24))
	mt.SetSize(vt.Coord{X: vt.Col(cols), Y: vt.Row(rows)})
	scr, err := tcell.NewTerminfoScreenFromTty(mt)
	if err != nil {
		t.Fatalf("NewTerminfoScreenFromTty: %v", err)
	}
	return scr, mt
}

// TestLoopRunDispatchesKeyboardMouseAndExits drives a real Loop.Run against
// a vt.MockTerm-backed Screen (newTestScreen, cellbridge_test.go),
// injecting a keypress, a mouse click, and Ctrl-C, and asserts the
// corresponding Document dispatch happened — end-to-end coverage of
// tcell_loop.go's event translation (keyName, EventMouse button handling,
// Ctrl-C exit), which is new code introduced by this migration; Document's
// own dispatch behavior (event_test.go) is unchanged and not re-tested
// here.
func TestLoopRunDispatchesKeyboardMouseAndExits(t *testing.T) {
	doc, err := document.ParseDocument(`<div><input type="text" id="name"></div><div><input type="checkbox" id="cb"></div>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	name := doc.GetElementByID("name")
	name.Focus()
	cb := doc.GetElementByID("cb")
	if _, err := doc.Render(); err != nil {
		t.Fatalf("initial Render: %v", err)
	}
	rect, ok := cb.Rect()
	if !ok {
		t.Fatalf("checkbox has no recorded Rect")
	}

	scr, mt := newUninitScreen(t, 40, 5)
	loop := newLoopWithScreen(doc, scr)

	var wg sync.WaitGroup
	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		runErr = loop.Run()
	}()

	time.Sleep(50 * time.Millisecond) // let Run's Init + first paint land

	mt.KeyTap(vt.KeyA)
	mt.Drain()
	time.Sleep(25 * time.Millisecond)

	pos := vt.Coord{X: vt.Col(rect.Col), Y: vt.Row(rect.Row)}
	mt.MouseEvent(vt.MouseEvent{Position: pos, Button: vt.Button1, Down: true})
	mt.MouseEvent(vt.MouseEvent{Position: pos, Button: vt.NoButton, Down: false})
	mt.Drain()
	time.Sleep(25 * time.Millisecond)

	mt.KeyTap(vt.KeyLCtrl, vt.KeyC)
	mt.Drain()
	wg.Wait()

	if runErr != nil {
		t.Errorf("Run returned error: %v", runErr)
	}
	if got := name.Value(); got != "a" {
		t.Errorf("after typing 'a', name value = %q, want %q", got, "a")
	}
	if !cb.Checked() {
		t.Errorf("checkbox not checked after simulated click")
	}
}

// TestLoopQuit drives a real Loop.Run and asserts that a "quit" event
// listener calling Loop.Quit() (the programmatic equivalent of Ctrl-C, added
// so a host app can implement its own typed quit command) makes Run return,
// without requiring the raw Ctrl-C key sequence.
func TestLoopQuit(t *testing.T) {
	doc, err := document.ParseDocument(`<input type="text" id="cmd">`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	cmd := doc.GetElementByID("cmd")
	cmd.Focus()

	scr, mt := newUninitScreen(t, 40, 5)
	loop := newLoopWithScreen(doc, scr)

	doc.AddEventListener(cmd, "keydown", false, func(e *document.Event) {
		if e.Key == "q" {
			loop.Quit()
		}
	})

	var wg sync.WaitGroup
	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		runErr = loop.Run()
	}()

	time.Sleep(50 * time.Millisecond) // let Run's Init + first paint land

	mt.KeyTap(vt.KeyQ)
	mt.Drain()
	wg.Wait()

	if runErr != nil {
		t.Errorf("Run returned error: %v", runErr)
	}
}

// TestLoopRunHandlesBracketedPaste drives a real Loop.Run and injects a raw
// bracketed-paste sequence (CSI 200~ ... CSI 201~, the hardcoded escape pair
// tcell's input parser recognizes regardless of terminfo — see input.go's
// keyPasteStart/keyPasteEnd table) around some plain text, asserting the
// text lands in the focused input's value as a single paste rather than
// being interpreted key-by-key (which would, e.g., let a literal "\t" in
// the pasted content move focus via Tab's normal default action instead of
// being inserted literally).
func TestLoopRunHandlesBracketedPaste(t *testing.T) {
	doc, err := document.ParseDocument(`<input type="text" id="name">`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	name := doc.GetElementByID("name")
	name.Focus()

	scr, mt := newUninitScreen(t, 40, 5)
	loop := newLoopWithScreen(doc, scr)

	var wg sync.WaitGroup
	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		runErr = loop.Run()
	}()

	time.Sleep(50 * time.Millisecond) // let Run's Init + first paint land

	mt.SendRaw([]byte("\x1b[200~pasted\x1b[201~"))
	mt.Drain()
	time.Sleep(25 * time.Millisecond)

	mt.KeyTap(vt.KeyLCtrl, vt.KeyC)
	mt.Drain()
	wg.Wait()

	if runErr != nil {
		t.Errorf("Run returned error: %v", runErr)
	}
	if got := name.Value(); got != "pasted" {
		t.Errorf("after bracketed paste, name value = %q, want %q", got, "pasted")
	}
}

// TestLoopRunHandlesCtrlXCut drives a real Loop.Run, focuses a text input
// with an existing value, sends Ctrl-X, and asserts the document-level
// effect of Document.DispatchCut ran (the value cleared) — end-to-end
// coverage of tcell_loop.go's KeyCtrlX wiring. The vt.MockTerm-backed test
// Screen does report clipboard support, which is what makes the cut run at
// all — see TestLoopCtrlXWithoutClipboardKeepsValue for the other branch.
// What actually reaches the OS clipboard is tcell's plumbing, not this
// package's, and isn't asserted here.
func TestLoopRunHandlesCtrlXCut(t *testing.T) {
	doc, err := document.ParseDocument(`<input type="text" id="name" value="secret">`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	name := doc.GetElementByID("name")
	name.Focus()

	scr, mt := newUninitScreen(t, 40, 5)
	loop := newLoopWithScreen(doc, scr)

	var wg sync.WaitGroup
	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		runErr = loop.Run()
	}()

	time.Sleep(50 * time.Millisecond) // let Run's Init + first paint land

	mt.KeyTap(vt.KeyLCtrl, vt.KeyX)
	mt.Drain()
	time.Sleep(25 * time.Millisecond)

	mt.KeyTap(vt.KeyLCtrl, vt.KeyC)
	mt.Drain()
	wg.Wait()

	if runErr != nil {
		t.Errorf("Run returned error: %v", runErr)
	}
	if got := name.Value(); got != "" {
		t.Errorf("after Ctrl-X, name value = %q, want empty", got)
	}
}

// TestLoopRunDispatchesKeyUp checks the wiring added for keyup support: an
// EventKey with Pressed() false reaches Document.DispatchKeyUp, not
// DispatchKey, and still triggers a repaint. vt.MockTerm has no terminal-side
// Kitty-protocol encoder (see the vt package's emulate.go, "legacy protocol
// does not support key release"), so a release can't be produced by typing
// through it the way the other tests in this file drive input; a real
// terminal that negotiated the Kitty or Win32 keyboard protocol is what
// would deliver one in production (see COMPATIBILITY.md's "keyup" entry).
// This test instead posts a hand-built release EventKey directly to the
// Screen's own EventQ channel, the same channel tcell's real decoder
// delivers every event on, to exercise Loop.Run's translation in isolation
// from that terminal-side gap.
// initSignalScreen closes ready once the wrapped Screen's Init has
// returned. tcell.Screen's Init and EventQ touch the same internal field
// with no lock of their own (tscreen.go), so a goroutine that wants to post
// straight to the channel, as TestLoopRunDispatchesKeyUp does below, needs a
// real happens-before edge showing Init has finished, not a sleep-based
// guess: the race detector flags the guess as a data race even when the
// timing happens to work out, since a sleep establishes no synchronization
// the Go memory model recognizes.
type initSignalScreen struct {
	tcell.Screen
	ready chan struct{}
}

func (s *initSignalScreen) Init() error {
	err := s.Screen.Init()
	close(s.ready)
	return err
}

func TestLoopRunDispatchesKeyUp(t *testing.T) {
	doc, err := document.ParseDocument(`<input type="text" id="name">`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	name := doc.GetElementByID("name")
	name.Focus()

	var gotType, gotKey string
	doc.AddEventListener(name, "keyup", false, func(e *document.Event) {
		gotType, gotKey = e.Type, e.Key
	})
	doc.AddEventListener(name, "keydown", false, func(e *document.Event) {
		t.Errorf("keydown fired for a release EventKey, want keyup only")
	})

	scr, _ := newUninitScreen(t, 40, 5)
	ready := make(chan struct{})
	loop := newLoopWithScreen(doc, &initSignalScreen{Screen: scr, ready: ready})

	var wg sync.WaitGroup
	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		runErr = loop.Run()
	}()

	<-ready                           // Init has returned; safe to post to scr's EventQ from here
	time.Sleep(50 * time.Millisecond) // let the first paint land

	scr.EventQ() <- tcell.NewEventKeyEx(tcell.KeyRune, "a", tcell.ModNone, false, tcell.KeyA, 0)
	time.Sleep(25 * time.Millisecond)

	scr.EventQ() <- tcell.NewEventKeyEx(tcell.KeyCtrlC, "", tcell.ModCtrl, true, tcell.KeyCtrlC, 0)
	wg.Wait()

	if runErr != nil {
		t.Errorf("Run returned error: %v", runErr)
	}
	if gotType != "keyup" || gotKey != "a" {
		t.Errorf("keyup listener saw Type=%q Key=%q, want Type=keyup Key=a", gotType, gotKey)
	}
	if got := name.Value(); got != "" {
		t.Errorf("name value after a release-only EventKey = %q, want empty (no typing default action)", got)
	}
}

// TestFocusCursorPosMultiLineTextarea is a regression test for a bug where
// focusCursorPos computed a focused <textarea>'s cursor row/column from its
// whole value's total rune count with no awareness of embedded newlines,
// always landing on the box's first row (rect.Row) regardless of how many
// lines had actually been typed — and, once fixed to split on "\n", a second
// bug where the row wasn't shifted past the textarea's own border-top/
// padding-top rows (Document.ContentOffset), landing one row short of the
// last line for the default bordered <textarea>.
func TestFocusCursorPosMultiLineTextarea(t *testing.T) {
	doc, err := document.ParseDocument(`<textarea id="ta" style="width:20" value="line one
line two
line three"></textarea>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.GetElementByID("ta")
	el.Focus()

	rect, ok := el.Rect()
	if !ok {
		t.Fatalf("textarea has no recorded Rect")
	}
	row, col, ok := focusCursorPos(doc)
	if !ok {
		t.Fatal("focusCursorPos ok = false, want true")
	}
	// Default UA textarea styling draws a border on every side, so content
	// starts one row below rect.Row: row 0 border, row 1 "line one", row 2
	// "line two", row 3 "line three" (the cursor's expected row).
	if wantRow := rect.Row + 3; row != wantRow {
		t.Errorf("row = %d, want %d (rect=%+v)", row, wantRow, rect)
	}
	// ...and, on the same axis logic, two columns right of rect.Col: the UA
	// stylesheet's border-left plus padding-left (Document.ContentOffsetX).
	// This was a second, symmetric bug to the row one above — the cursor
	// used to land on the left border glyph itself.
	offsetX, ok := doc.ContentOffsetX(el)
	if !ok || offsetX != 2 {
		t.Fatalf("ContentOffsetX = %d, %v; want 2, true (border-left + padding-left)", offsetX, ok)
	}
	if wantCol := rect.Col + offsetX + len("line three"); col != wantCol {
		t.Errorf("col = %d, want %d (end of last line, past border+padding)", col, wantCol)
	}
}

// TestFocusCursorPosWideRunes pins that caret placement measures columns, not
// runes: "日本語" is three runes but six cells, so a caret after them belongs
// at column 6. Placing it at column 3 (the old rune-count math) put the
// terminal cursor in the middle of 語.
func TestFocusCursorPosWideRunes(t *testing.T) {
	doc, err := document.ParseDocument(`<input id="i" value="日本語ab">`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.GetElementByID("i")
	el.Focus()

	for _, tc := range []struct {
		caret   int
		wantCol int
	}{
		{0, 0}, // before 日
		{1, 2}, // after 日
		{3, 6}, // after 語
		{5, 8}, // after "ab" — end of value
	} {
		el.SetSelectionRange(tc.caret, tc.caret)
		if _, err := doc.Render(); err != nil {
			t.Fatalf("Render: %v", err)
		}
		rect, _ := el.Rect()
		_, col, ok := focusCursorPos(doc)
		if !ok {
			t.Fatal("focusCursorPos ok = false, want true")
		}
		if want := rect.Col + tc.wantCol; col != want {
			t.Errorf("caret %d: col = %d, want %d", tc.caret, col, want)
		}
	}
}

// TestClickCaretRoundTripWideRunes is TestFocusCursorPosWideRunes's inverse:
// clicking a cell must produce a caret whose cursor lands back on that same
// cell, including for double-width characters (clicking either half of one
// puts the caret before it).
func TestClickCaretRoundTripWideRunes(t *testing.T) {
	doc, err := document.ParseDocument(`<input id="i" value="日本語ab">`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.GetElementByID("i")
	rect, _ := el.Rect()

	for _, tc := range []struct {
		clickCol  int
		wantCaret int
	}{
		{0, 0}, // left half of 日
		{1, 0}, // right half of 日 — still before it
		{2, 1}, // left half of 本
		{6, 3}, // "a"
		{7, 4}, // "b"
	} {
		doc.DispatchClick(rect.Row, rect.Col+tc.clickCol, document.Modifiers{})
		if got := el.SelectionStart(); got != tc.wantCaret {
			t.Errorf("click col %d: caret = %d, want %d", tc.clickCol, got, tc.wantCaret)
		}
		if _, err := doc.Render(); err != nil {
			t.Fatalf("Render: %v", err)
		}
		_, col, ok := focusCursorPos(doc)
		if !ok {
			t.Fatal("focusCursorPos ok = false, want true")
		}
		// The cursor lands on the clicked cluster's own first cell — the
		// clicked cell itself for a single-width character.
		if wantCol := rect.Col + columnOfCaret("日本語ab", tc.wantCaret); col != wantCol {
			t.Errorf("click col %d: cursor col = %d, want %d", tc.clickCol, col, wantCol)
		}
	}
}

// columnOfCaret is the test's own independent restatement of the rune-offset
// to column mapping, so the assertion above doesn't just re-derive itself
// from the production helper it's checking.
func columnOfCaret(value string, caret int) int {
	col := 0
	for i, r := range []rune(value) {
		if i >= caret {
			break
		}
		if r > 0x2000 {
			col += 2 // the CJK characters this test uses
		} else {
			col++
		}
	}
	return col
}

// TestFocusCursorPosTextareaClickRoundTrip pins that a click's caret
// placement (Document.DispatchClick → caretIndexFromClick) and the terminal
// cursor's own placement (focusCursorPos) agree on where a given cell is:
// clicking a character must put the cursor back on that same character.
// Before both learned about ContentOffsetX they were off by the textarea's
// border-left + padding-left in opposite directions — a 4-column disagreement
// on the default UA styling.
func TestFocusCursorPosTextareaClickRoundTrip(t *testing.T) {
	doc, err := document.ParseDocument(`<textarea id="ta" style="width:20" value="abcdef"></textarea>`,
		htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.GetElementByID("ta")
	rect, ok := el.Rect()
	if !ok {
		t.Fatalf("textarea has no recorded Rect")
	}
	offset, _ := doc.ContentOffset(el)
	offsetX, _ := doc.ContentOffsetX(el)

	// Click the 'c' — the third content column of the first content row.
	clickRow, clickCol := rect.Row+offset, rect.Col+offsetX+2
	doc.DispatchClick(clickRow, clickCol, document.Modifiers{})
	if got := el.SelectionStart(); got != 2 {
		t.Errorf("SelectionStart after clicking 'c' = %d, want 2", got)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	row, col, ok := focusCursorPos(doc)
	if !ok {
		t.Fatal("focusCursorPos ok = false, want true")
	}
	if row != clickRow || col != clickCol {
		t.Errorf("cursor = (%d, %d), want the clicked cell (%d, %d)", row, col, clickRow, clickCol)
	}
}

// TestFocusCursorPosTextareaWithoutBorderOrPadding checks that
// focusCursorPos's row math generalizes to a <textarea> with its default
// border/padding stripped via CSS, rather than hardcoding an assumption
// about the UA stylesheet's default border-style.
func TestFocusCursorPosTextareaWithoutBorderOrPadding(t *testing.T) {
	doc, err := document.ParseDocument(`<textarea id="ta" style="border-style:none;padding:0;width:20" value="a
b"></textarea>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.GetElementByID("ta")
	el.Focus()

	rect, ok := el.Rect()
	if !ok {
		t.Fatalf("textarea has no recorded Rect")
	}
	row, col, ok := focusCursorPos(doc)
	if !ok {
		t.Fatal("focusCursorPos ok = false, want true")
	}
	if wantRow := rect.Row + 1; row != wantRow {
		t.Errorf("row = %d, want %d (rect=%+v)", row, wantRow, rect)
	}
	if wantCol := rect.Col + len("b"); col != wantCol {
		t.Errorf("col = %d, want %d", col, wantCol)
	}
}

// TestFocusCursorPosUsesSelectionEndNotAlwaysEndOfValue pins that
// focusCursorPos follows a non-collapsed-at-end caret set via
// SetSelectionRange, rather than always landing at the end of the value —
// see docs/proposals/CARET_SELECTION.md.
func TestFocusCursorPosUsesSelectionEndNotAlwaysEndOfValue(t *testing.T) {
	doc, err := document.ParseDocument(`<input type="text" id="in" value="hello">`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.GetElementByID("in")
	el.Focus()
	el.SetSelectionRange(2, 2) // caret after "he"

	rect, ok := el.Rect()
	if !ok {
		t.Fatalf("input has no recorded Rect")
	}
	row, col, ok := focusCursorPos(doc)
	if !ok {
		t.Fatal("focusCursorPos ok = false, want true")
	}
	if row != rect.Row {
		t.Errorf("row = %d, want %d", row, rect.Row)
	}
	if wantCol := rect.Col + 2; col != wantCol {
		t.Errorf("col = %d, want %d (caret after \"he\", not end of value)", col, wantCol)
	}
}

// TestFocusCursorPosTextareaCaretMidValue pins that focusCursorPos's
// caretLineCol locates a caret on an earlier line, not just the last one —
// exercising the same "Enter can insert a newline anywhere, not just
// append" change DispatchKey's replaceSelection made (see
// docs/proposals/CARET_SELECTION.md).
func TestFocusCursorPosTextareaCaretMidValue(t *testing.T) {
	doc, err := document.ParseDocument(`<textarea id="ta" style="border-style:none;padding:0;width:20" value="line one
line two
line three"></textarea>`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.GetElementByID("ta")
	el.Focus()
	// "line one\n" is 9 runes; +4 lands after "line" on the second line.
	el.SetSelectionRange(13, 13)

	rect, ok := el.Rect()
	if !ok {
		t.Fatalf("textarea has no recorded Rect")
	}
	row, col, ok := focusCursorPos(doc)
	if !ok {
		t.Fatal("focusCursorPos ok = false, want true")
	}
	if wantRow := rect.Row + 1; row != wantRow {
		t.Errorf("row = %d, want %d (second line, not the last)", row, wantRow)
	}
	if wantCol := rect.Col + len("line"); col != wantCol {
		t.Errorf("col = %d, want %d", col, wantCol)
	}
}

func TestKeyNameHomeEndDelete(t *testing.T) {
	tests := []struct {
		key     tcell.Key
		want    string
		wantOK  bool
		comment string
	}{
		{tcell.KeyHome, "Home", true, "Home"},
		{tcell.KeyEnd, "End", true, "End"},
		{tcell.KeyDelete, "Delete", true, "Delete"},
	}
	for _, tt := range tests {
		ev := tcell.NewEventKey(tt.key, "", tcell.ModNone)
		got, ok := keyName(ev)
		if got != tt.want || ok != tt.wantOK {
			t.Errorf("keyName(%s) = (%q, %v), want (%q, %v)", tt.comment, got, ok, tt.want, tt.wantOK)
		}
	}
}

// TestFocusCursorPosSingleLineInputUnaffected pins that a single-line text
// input (no embedded newlines, and never display:block so it never has a
// ContentOffset entry) still uses the simple rect.Row/whole-value-length
// placement, unaffected by the <textarea> multi-line handling above.
func TestFocusCursorPosSingleLineInputUnaffected(t *testing.T) {
	doc, err := document.ParseDocument(`<input type="text" id="in" value="hello">`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.GetElementByID("in")
	el.Focus()

	rect, ok := el.Rect()
	if !ok {
		t.Fatalf("input has no recorded Rect")
	}
	row, col, ok := focusCursorPos(doc)
	if !ok {
		t.Fatal("focusCursorPos ok = false, want true")
	}
	if row != rect.Row {
		t.Errorf("row = %d, want %d", row, rect.Row)
	}
	if wantCol := rect.Col + len("hello"); col != wantCol {
		t.Errorf("col = %d, want %d", col, wantCol)
	}
}

// TestKeyNameBacktabIsShiftTab pins that tcell's own Shift+Tab key code
// arrives at Document as ("Tab", Shift) — the pair DispatchKey needs to walk
// the tab order backwards. Legacy keyboard reporting sends KeyBacktab with no
// ModShift at all, so keyName alone can't carry it; keyModifiers re-attaches
// the Shift. Before this, Shift+Tab was dropped by keyName's default case and
// never reached Document.
func TestKeyNameBacktabIsShiftTab(t *testing.T) {
	ev := tcell.NewEventKey(tcell.KeyBacktab, "", tcell.ModNone)
	key, ok := keyName(ev)
	if key != "Tab" || !ok {
		t.Fatalf("keyName(KeyBacktab) = (%q, %v), want (%q, true)", key, ok, "Tab")
	}
	if mods := keyModifiers(ev); !mods.Shift {
		t.Errorf("keyModifiers(KeyBacktab).Shift = false, want true")
	}
	// A plain Tab must not pick up a phantom Shift.
	if mods := keyModifiers(tcell.NewEventKey(tcell.KeyTab, "", tcell.ModNone)); mods.Shift {
		t.Errorf("keyModifiers(KeyTab).Shift = true, want false")
	}
}

// noClipboardScreen is a tcell.Screen that reports no clipboard support,
// whatever the underlying screen negotiated — the seam for testing Ctrl-X on
// a terminal without OSC 52.
type noClipboardScreen struct {
	tcell.Screen
	setCalls int
}

func (s *noClipboardScreen) HasClipboard() bool { return false }

func (s *noClipboardScreen) SetClipboard(b []byte) { s.setCalls++ }

// TestLoopCtrlXWithoutClipboardKeepsValue pins that Ctrl-X is inert when the
// terminal can't accept clipboard content. DispatchCut removes the text as
// part of dispatching, so calling it first and only then checking
// HasClipboard (as this used to) destroyed the value with nowhere to put it
// and no undo — and a collapsed caret makes a cut take the whole field.
func TestLoopCtrlXWithoutClipboardKeepsValue(t *testing.T) {
	doc, err := document.ParseDocument(`<input type="text" id="name" value="secret">`, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	name := doc.GetElementByID("name")
	name.Focus()

	scr, mt := newUninitScreen(t, 40, 5)
	wrapped := &noClipboardScreen{Screen: scr}
	loop := newLoopWithScreen(doc, wrapped)

	var wg sync.WaitGroup
	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		runErr = loop.Run()
	}()

	time.Sleep(50 * time.Millisecond) // let Run's Init + first paint land

	mt.KeyTap(vt.KeyLCtrl, vt.KeyX)
	mt.Drain()
	time.Sleep(25 * time.Millisecond)

	mt.KeyTap(vt.KeyLCtrl, vt.KeyC)
	mt.Drain()
	wg.Wait()

	if runErr != nil {
		t.Errorf("Run returned error: %v", runErr)
	}
	if got := name.Value(); got != "secret" {
		t.Errorf("value after Ctrl-X with no clipboard = %q, want it preserved", got)
	}
	if wrapped.setCalls != 0 {
		t.Errorf("SetClipboard called %d times on a clipboard-less terminal, want 0", wrapped.setCalls)
	}
}
