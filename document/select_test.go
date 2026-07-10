package document_test

import (
	"strings"
	"testing"

	"github.com/client9/htmlterm/document"
)

func mustParseSelectDoc(t *testing.T, htmlStr string) (*document.Document, *document.Element) {
	t.Helper()
	doc := mustParseDoc(t, htmlStr)
	sel := doc.GetElementByID("s")
	if sel == nil {
		t.Fatalf("no element with id=s in %s", htmlStr)
	}
	return doc, sel
}

const fruitSelectHTML = `<select id="s">
<option value="a">Apple</option>
<option value="b" selected>Banana</option>
<option value="c">Cherry</option>
</select>`

func TestSelectClickOpensThenClickOptionSelectsAndCloses(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	rect, ok := doc.Rect(sel)
	if !ok {
		t.Fatalf("Rect(sel) not found")
	}

	changed := 0
	doc.AddEventListener(sel, "change", false, func(e *document.Event) { changed++ })

	if !doc.DispatchClick(rect.Row, rect.Col) {
		t.Fatalf("click on closed select did not hit it")
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "Cherry") {
		t.Fatalf("popup did not render after opening select: %q", out)
	}

	cherryRect, ok := doc.Rect(doc.QuerySelector(`option[value="c"]`))
	if !ok {
		t.Fatalf("no Rect recorded for the Cherry option while open")
	}
	if !doc.DispatchClick(cherryRect.Row, cherryRect.Col) {
		t.Fatalf("click on Cherry option did not hit it")
	}

	if got := sel.Value(); got != "c" {
		t.Errorf("sel.Value() = %q, want %q", got, "c")
	}
	if changed != 1 {
		t.Errorf("change fired %d times, want 1", changed)
	}

	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "Apple") {
		t.Errorf("popup still rendered after selecting an option, want it closed: %q", out)
	}
}

func TestSelectClickTogglesOpenClosed(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	rect, _ := doc.Rect(sel)

	doc.DispatchClick(rect.Row, rect.Col)
	out, _ := doc.Render()
	if !strings.Contains(out, "Apple") {
		t.Fatalf("popup did not open on first click: %q", out)
	}

	doc.DispatchClick(rect.Row, rect.Col)
	out, _ = doc.Render()
	if strings.Contains(out, "Apple") {
		t.Errorf("popup still open after clicking the control a second time: %q", out)
	}
}

func TestSelectKeyEnterOpensAndEscapeCloses(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	doc.Focus(sel)

	doc.DispatchKey("Enter")
	out, _ := doc.Render()
	if !strings.Contains(out, "Apple") {
		t.Fatalf("popup did not open on Enter: %q", out)
	}

	doc.DispatchKey("Escape")
	out, _ = doc.Render()
	if strings.Contains(out, "Apple") {
		t.Errorf("popup still open after Escape: %q", out)
	}
}

func TestSelectKeyArrowsChangeSelectionWithoutOpening(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	doc.Focus(sel)

	changed := 0
	doc.AddEventListener(sel, "change", false, func(e *document.Event) { changed++ })

	doc.DispatchKey("ArrowDown")
	if got := sel.Value(); got != "c" {
		t.Errorf("after ArrowDown, Value() = %q, want %q", got, "c")
	}
	doc.DispatchKey("ArrowDown")
	if got := sel.Value(); got != "c" {
		t.Errorf("ArrowDown past the last option should clamp: Value() = %q, want %q", got, "c")
	}
	doc.DispatchKey("ArrowUp")
	doc.DispatchKey("ArrowUp")
	if got := sel.Value(); got != "a" {
		t.Errorf("after two ArrowUp, Value() = %q, want %q", got, "a")
	}
	doc.DispatchKey("ArrowUp")
	if got := sel.Value(); got != "a" {
		t.Errorf("ArrowUp past the first option should clamp: Value() = %q, want %q", got, "a")
	}

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "Cherry") && strings.Contains(out, "\n") {
		t.Errorf("arrow-key selection changes should not open the popup: %q", out)
	}
	// 3 actual moves (b→c, c→b, b→a); the two clamped arrow presses at
	// each end are no-ops and fire no "change".
	if changed != 3 {
		t.Errorf("change fired %d times, want 3", changed)
	}
}

func TestSelectClickOutsideClosesPopup(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML+`<p id="p">unrelated text</p>`)
	rect, _ := doc.Rect(sel)

	doc.DispatchClick(rect.Row, rect.Col) // open it
	out, _ := doc.Render()
	if !strings.Contains(out, "Apple") {
		t.Fatalf("popup did not open: %q", out)
	}

	p := doc.GetElementByID("p")
	pRect, ok := doc.Rect(p)
	if !ok {
		t.Fatalf("Rect(p) not found")
	}
	doc.DispatchClick(pRect.Row, pRect.Col) // click unrelated content
	out, _ = doc.Render()
	if strings.Contains(out, "Apple") {
		t.Errorf("popup still open after clicking unrelated content: %q", out)
	}
}

func TestSelectClickMissClosesPopup(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	rect, _ := doc.Rect(sel)

	doc.DispatchClick(rect.Row, rect.Col) // open it
	out, _ := doc.Render()
	if !strings.Contains(out, "Apple") {
		t.Fatalf("popup did not open: %q", out)
	}

	if doc.DispatchClick(9999, 9999) {
		t.Fatalf("DispatchClick unexpectedly hit an element at (9999, 9999)")
	}
	out, _ = doc.Render()
	if strings.Contains(out, "Apple") {
		t.Errorf("popup still open after a hit-test-miss click: %q", out)
	}
}

func TestSelectFocusMovingAwayClosesPopup(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML+`<input id="i" type="text">`)
	doc.Focus(sel)
	doc.DispatchKey("Enter")
	out, _ := doc.Render()
	if !strings.Contains(out, "Apple") {
		t.Fatalf("popup did not open: %q", out)
	}

	inp := doc.GetElementByID("i")
	if !doc.Focus(inp) {
		t.Fatalf("Focus(input) returned false")
	}
	out, _ = doc.Render()
	if strings.Contains(out, "Apple") {
		t.Errorf("popup still open after focus moved away: %q", out)
	}
}

func TestSelectValueAndSetValue(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	if got := sel.Value(); got != "b" {
		t.Errorf("Value() = %q, want %q", got, "b")
	}

	sel.SetValue("c")
	if got := sel.Value(); got != "c" {
		t.Errorf("after SetValue(c), Value() = %q, want %q", got, "c")
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	sel.SetValue("does-not-exist")
	if got := sel.Value(); got != "c" {
		t.Errorf("SetValue with no matching option should leave selection unchanged: Value() = %q, want %q", got, "c")
	}
}

func TestSelectValueFallsBackToOptionTextWithNoValueAttr(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, `<select id="s"><option>Apple</option><option selected>Banana</option></select>`)
	if got := sel.Value(); got != "Banana" {
		t.Errorf("Value() = %q, want %q", got, "Banana")
	}
	sel.SetValue("Apple")
	if got := sel.Value(); got != "Apple" {
		t.Errorf("after SetValue(Apple), Value() = %q, want %q", got, "Apple")
	}
	_ = doc
}

func TestSelectDisabledClickIsInert(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, `<select id="s" disabled><option>Apple</option><option selected>Banana</option></select>`)
	rect, ok := doc.Rect(sel)
	if !ok {
		t.Fatalf("Rect(sel) not found")
	}
	doc.DispatchClick(rect.Row, rect.Col)
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "Apple") {
		t.Errorf("a disabled select should not open on click: %q", out)
	}
}
