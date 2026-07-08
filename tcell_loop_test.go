package htmlterm

import (
	"sync"
	"testing"
	"time"

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
	doc, err := ParseDocument(`<input type="text" id="name"><input type="checkbox" id="cb">`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	name := doc.GetElementByID("name")
	doc.Focus(name)

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

	if got := name.Value(); got != "a" {
		t.Errorf("after typing 'a', name value = %q, want %q", got, "a")
	}

	cb := doc.GetElementByID("cb")
	rect, ok := doc.Rect(cb)
	if !ok {
		t.Fatalf("checkbox has no recorded Rect")
	}
	pos := vt.Coord{X: vt.Col(rect.Col), Y: vt.Row(rect.Row)}
	mt.MouseEvent(vt.MouseEvent{Position: pos, Button: vt.Button1, Down: true})
	mt.MouseEvent(vt.MouseEvent{Position: pos, Button: vt.NoButton, Down: false})
	mt.Drain()
	time.Sleep(25 * time.Millisecond)

	if !cb.Checked() {
		t.Errorf("checkbox not checked after simulated click")
	}

	mt.KeyTap(vt.KeyLCtrl, vt.KeyC)
	mt.Drain()
	wg.Wait()

	if runErr != nil {
		t.Errorf("Run returned error: %v", runErr)
	}
}
