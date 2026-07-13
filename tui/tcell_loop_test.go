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
	if wantCol := rect.Col + len("line three"); col != wantCol {
		t.Errorf("col = %d, want %d (end of last line)", col, wantCol)
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
