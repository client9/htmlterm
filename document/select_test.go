package document_test

import (
	"strings"
	"testing"

	"github.com/client9/htmlterm"
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
	rect, ok := sel.Rect()
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

	cherryRect, ok := doc.QuerySelector(`option[value="c"]`).Rect()
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

// TestSelectPopupStyledWithBorderPaddingStillClickable is an end-to-end
// regression for the popup's CSS-driven border/padding: opening the popup
// through a real click (not the internal render package's own unit tests)
// must produce a bordered popup in the rendered output, and clicking an
// option's Rect — now offset by the border+padding columns/rows — must still
// hit exactly that option, matching real DOM click behavior for a styled
// dropdown.
func TestSelectPopupStyledWithBorderPaddingStillClickable(t *testing.T) {
	src := `<select id="s" style="border: solid; padding: 1">
<option value="a">Apple</option>
<option value="b" selected>Banana</option>
<option value="c">Cherry</option>
</select>`
	doc, err := document.ParseDocument(src, htmlterm.Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	sel := doc.GetElementByID("s")
	rect, ok := sel.Rect()
	if !ok {
		t.Fatalf("Rect(sel) not found")
	}

	if !doc.DispatchClick(rect.Row, rect.Col) {
		t.Fatalf("click on closed select did not hit it")
	}
	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "─") {
		t.Fatalf("popup did not render its CSS border: %q", out)
	}

	cherryRect, ok := doc.QuerySelector(`option[value="c"]`).Rect()
	if !ok {
		t.Fatalf("no Rect recorded for the Cherry option while open")
	}
	if !doc.DispatchClick(cherryRect.Row, cherryRect.Col) {
		t.Fatalf("click on Cherry option (offset by border+padding) did not hit it")
	}
	if got := sel.Value(); got != "c" {
		t.Errorf("sel.Value() = %q, want %q", got, "c")
	}
}

func TestSelectClickTogglesOpenClosed(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	rect, _ := sel.Rect()

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
	sel.Focus()

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
	sel.Focus()

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

func TestSelectArrowsWhileOpenDoNotCommitUntilConfirmed(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	sel.Focus()

	changed := 0
	doc.AddEventListener(sel, "change", false, func(e *document.Event) { changed++ })

	doc.DispatchKey("Enter") // open — starts selected on "b" (Banana)
	doc.DispatchKey("ArrowDown")
	doc.DispatchKey("ArrowDown")
	if got := sel.Value(); got != "b" {
		t.Errorf("browsing with the popup open should not move Value(): got %q, want %q", got, "b")
	}
	if changed != 0 {
		t.Errorf("change fired %d times while browsing an open popup, want 0", changed)
	}

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(out, "Cherry") {
		t.Fatalf("popup should still be open and rendered: %q", out)
	}
	// The visible marker follows the highlighted (browsed-to) option, not
	// the still-uncommitted selected value.
	if !strings.Contains(out, "▸ Cherry") {
		t.Errorf("highlight marker should be on the browsed-to option (Cherry): %q", out)
	}

	doc.DispatchKey("Enter") // confirm
	if got := sel.Value(); got != "c" {
		t.Errorf("after confirming, Value() = %q, want %q", got, "c")
	}
	if changed != 1 {
		t.Errorf("change fired %d times total, want exactly 1 (on confirm)", changed)
	}

	out, err = doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "Cherry") && strings.Contains(out, "\n") {
		t.Errorf("popup should be closed after confirming: %q", out)
	}
}

func TestSelectEscapeAfterBrowsingDoesNotCommit(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	sel.Focus()

	changed := 0
	doc.AddEventListener(sel, "change", false, func(e *document.Event) { changed++ })

	doc.DispatchKey("Enter") // open
	doc.DispatchKey("ArrowDown")
	doc.DispatchKey("Escape")

	if got := sel.Value(); got != "b" {
		t.Errorf("Escape after browsing should leave Value() unchanged: got %q, want %q", got, "b")
	}
	if changed != 0 {
		t.Errorf("change fired %d times, want 0 (Escape cancels, never commits)", changed)
	}

	out, err := doc.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(out, "Cherry") && strings.Contains(out, "\n") {
		t.Errorf("popup should be closed after Escape: %q", out)
	}
}

func TestSelectConfirmWithoutMovingFiresNoChange(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	sel.Focus()

	changed := 0
	doc.AddEventListener(sel, "change", false, func(e *document.Event) { changed++ })

	doc.DispatchKey("Enter") // open
	doc.DispatchKey("Enter") // confirm immediately, no browsing

	if got := sel.Value(); got != "b" {
		t.Errorf("Value() = %q, want unchanged %q", got, "b")
	}
	if changed != 0 {
		t.Errorf("change fired %d times, want 0 (confirming the already-selected option is a no-op)", changed)
	}
}

func TestSelectClickOptionAfterArrowingConfirmsHighlighted(t *testing.T) {
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML)
	sel.Focus()
	doc.DispatchKey("Enter")
	doc.DispatchKey("ArrowUp") // highlight moves to Apple
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	appleRect, ok := doc.QuerySelector(`option[value="a"]`).Rect()
	if !ok {
		t.Fatalf("no Rect recorded for the Apple option while open")
	}
	if !doc.DispatchClick(appleRect.Row, appleRect.Col) {
		t.Fatalf("click on Apple option did not hit it")
	}
	if got := sel.Value(); got != "a" {
		t.Errorf("Value() = %q, want %q", got, "a")
	}
}

func TestSelectClickOutsideClosesPopup(t *testing.T) {
	// The dropdown popup overlays rows below the select without reflowing
	// later content, so "unrelated content" needs enough filler <br>s ahead
	// of it to land below the (3-option) popup's footprint, or the click
	// meant for it would land on the popup instead.
	doc, sel := mustParseSelectDoc(t, fruitSelectHTML+`<br><br><br><br><p id="p">unrelated text</p>`)
	rect, _ := sel.Rect()

	doc.DispatchClick(rect.Row, rect.Col) // open it
	out, _ := doc.Render()
	if !strings.Contains(out, "Apple") {
		t.Fatalf("popup did not open: %q", out)
	}

	p := doc.GetElementByID("p")
	pRect, ok := p.Rect()
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
	rect, _ := sel.Rect()

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
	sel.Focus()
	doc.DispatchKey("Enter")
	out, _ := doc.Render()
	if !strings.Contains(out, "Apple") {
		t.Fatalf("popup did not open: %q", out)
	}

	inp := doc.GetElementByID("i")
	if !inp.Focus() {
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
	rect, ok := sel.Rect()
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
