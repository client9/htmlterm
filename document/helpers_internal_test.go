package document

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func findSpan(t *testing.T, htmlStr string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "span" {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if found == nil {
		t.Fatal("span not found in parsed doc")
	}
	return found
}

func TestSelectionDefaultsToCollapsedAtEnd(t *testing.T) {
	doc, err := ParseDocument(`<input value="hello">`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.QuerySelector("input")
	if el == nil {
		t.Fatal("input not found")
	}
	if got := el.SelectionStart(); got != 5 {
		t.Errorf("SelectionStart() = %d, want 5 (end of \"hello\")", got)
	}
	if got := el.SelectionEnd(); got != 5 {
		t.Errorf("SelectionEnd() = %d, want 5", got)
	}
	if got := el.SelectionDirection(); got != "none" {
		t.Errorf("SelectionDirection() = %q, want \"none\"", got)
	}
}

func TestSetSelectionRangeClampsAndSwapsAndDirection(t *testing.T) {
	doc, err := ParseDocument(`<input value="hello">`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.QuerySelector("input")

	// Inverted range gets swapped, matching real setSelectionRange.
	el.SetSelectionRange(4, 1, "backward")
	if start, end := el.SelectionStart(), el.SelectionEnd(); start != 1 || end != 4 {
		t.Errorf("after SetSelectionRange(4, 1): start=%d end=%d, want 1,4", start, end)
	}
	if got := el.SelectionDirection(); got != "backward" {
		t.Errorf("SelectionDirection() = %q, want \"backward\"", got)
	}

	// Out-of-range offsets clamp to [0, len(value)] (5 runes).
	el.SetSelectionRange(-3, 99)
	if start, end := el.SelectionStart(), el.SelectionEnd(); start != 0 || end != 5 {
		t.Errorf("after SetSelectionRange(-3, 99): start=%d end=%d, want 0,5", start, end)
	}
	// Direction omitted normalizes to "none".
	if got := el.SelectionDirection(); got != "none" {
		t.Errorf("SelectionDirection() = %q, want \"none\"", got)
	}

	// Unrecognized direction also normalizes to "none".
	el.SetSelectionRange(0, 2, "sideways")
	if got := el.SelectionDirection(); got != "none" {
		t.Errorf("SelectionDirection() with bogus direction = %q, want \"none\"", got)
	}
}

func TestSetValueClearsSelection(t *testing.T) {
	doc, err := ParseDocument(`<input value="hello">`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	el := doc.QuerySelector("input")
	el.SetSelectionRange(1, 2)
	if start, end := el.SelectionStart(), el.SelectionEnd(); start != 1 || end != 2 {
		t.Fatalf("precondition failed: start=%d end=%d, want 1,2", start, end)
	}

	el.SetValue("hi")
	if start, end := el.SelectionStart(), el.SelectionEnd(); start != 2 || end != 2 {
		t.Errorf("after SetValue: start=%d end=%d, want collapsed at 2 (end of \"hi\")", start, end)
	}
}

func TestSetAttrAddsNew(t *testing.T) {
	n := findSpan(t, `<span>x</span>`)
	setAttr(n, "title", "hello")
	if got := nodeAttr(n, "title"); got != "hello" {
		t.Errorf("nodeAttr(title) = %q, want %q", got, "hello")
	}
	if len(n.Attr) != 1 {
		t.Errorf("len(n.Attr) = %d, want 1", len(n.Attr))
	}
}

func TestSetAttrUpdatesInPlace(t *testing.T) {
	n := findSpan(t, `<span title="a" class="b">x</span>`)
	setAttr(n, "title", "c")
	if got := nodeAttr(n, "title"); got != "c" {
		t.Errorf("nodeAttr(title) = %q, want %q", got, "c")
	}
	if len(n.Attr) != 2 {
		t.Errorf("len(n.Attr) = %d, want 2 (no duplicate appended)", len(n.Attr))
	}
}

func TestRemoveAttrRemovesPresent(t *testing.T) {
	n := findSpan(t, `<span title="a" class="b">x</span>`)
	removeAttr(n, "title")
	for _, a := range n.Attr {
		if a.Key == "title" {
			t.Errorf("title still present after removeAttr: %q", a.Val)
		}
	}
	if len(n.Attr) != 1 {
		t.Errorf("len(n.Attr) = %d, want 1", len(n.Attr))
	}
}

func TestRemoveAttrMissingIsNoop(t *testing.T) {
	n := findSpan(t, `<span class="b">x</span>`)
	removeAttr(n, "title")
	if len(n.Attr) != 1 {
		t.Errorf("len(n.Attr) = %d, want 1 (unchanged)", len(n.Attr))
	}
}

// TestElementAtTieBreakIsDeterministic pins elementAt's behavior when two
// equal-depth elements have overlapping recorded Rects (shouldn't arise from
// normal box layout, but is exactly the case a Go map-iteration-order tie
// break would make flaky): it must consistently pick the one that comes
// first in document order, not vary from call to call.
func TestElementAtTieBreakIsDeterministic(t *testing.T) {
	htmlStr := `<div id="a">a</div><div id="b">b</div>`
	root, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	var a, b *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch nodeAttr(n, "id") {
			case "a":
				a = n
			case "b":
				b = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(root)
	if a == nil || b == nil {
		t.Fatal("did not find both divs")
	}

	d := &Document{
		doc: root,
		positions: map[*html.Node]Rect{
			a: {Row: 0, Col: 0, Width: 5, Height: 1},
			b: {Row: 0, Col: 0, Width: 5, Height: 1},
		},
	}

	for i := 0; i < 20; i++ {
		if got := d.elementAt(0, 0); got != a {
			t.Fatalf("elementAt(0,0) call %d = %v, want the first-in-document-order element (a)", i, got)
		}
	}
}

func TestDocumentElementResizeDispatch(t *testing.T) {
	doc, err := ParseDocument(`<p>hi</p>`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	fired := false
	doc.AddEventListener(doc.DocumentElement(), "resize", false, func(e *Event) {
		fired = true
		if e.Type != "resize" {
			t.Errorf("Event.Type = %q, want %q", e.Type, "resize")
		}
	})
	doc.dispatch(doc.doc, "resize", "", Modifiers{})
	if !fired {
		t.Error("resize listener did not fire")
	}
}

// TestPruneDetachedStateDropsSelectionsAndValueBaselines pins that per-node
// state for nodes cut out of the tree is swept, not leaked. d.selections and
// d.valueAtFocus are the two maps Render does *not* rebuild each frame (see
// pruneDetachedState), so before this a long-running Loop that repeatedly
// refreshed a container via SetInnerHTML accumulated one entry per text entry
// it ever rendered.
func TestPruneDetachedStateDropsSelectionsAndValueBaselines(t *testing.T) {
	doc, err := ParseDocument(`<div id="pane"><input id="a" value="hello"></div><input id="keep" value="x">`,
		Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}

	// Give both inputs recorded selection state, and both focus baselines.
	a := doc.GetElementByID("a")
	keep := doc.GetElementByID("keep")
	a.Focus()
	a.SetSelectionRange(1, 3)
	keep.Focus() // focusing keep snapshots its value and blurs a
	keep.SetSelectionRange(0, 1)

	if len(doc.selections) != 2 {
		t.Fatalf("selections = %d entries, want 2 before the removal", len(doc.selections))
	}

	if err := doc.GetElementByID("pane").SetInnerHTML(`<p>replaced</p>`); err != nil {
		t.Fatalf("SetInnerHTML: %v", err)
	}

	if len(doc.selections) != 1 {
		t.Errorf("selections = %d entries after detaching one input, want 1", len(doc.selections))
	}
	if _, ok := doc.selections[keep.node]; !ok {
		t.Error("the still-attached input's selection was swept too, want it kept")
	}
	for n := range doc.valueAtFocus {
		if !isDescendant(doc.doc, n) {
			t.Error("valueAtFocus still holds an entry for a detached node")
		}
	}
	// The surviving input keeps working — the sweep didn't disturb live state.
	if start, end := keep.SelectionStart(), keep.SelectionEnd(); start != 0 || end != 1 {
		t.Errorf("surviving input's selection = [%d,%d), want [0,1)", start, end)
	}
}

// TestPruneDetachedStateViaRemoveChild pins that the sweep runs on the
// element-level tree mutations too, not just SetInnerHTML.
func TestPruneDetachedStateViaRemoveChild(t *testing.T) {
	doc, err := ParseDocument(`<div id="pane"><input id="a" value="hello"></div>`, Options{Width: 40})
	if err != nil {
		t.Fatalf("ParseDocument: %v", err)
	}
	if _, err := doc.Render(); err != nil {
		t.Fatalf("Render: %v", err)
	}
	a := doc.GetElementByID("a")
	a.Focus()
	a.SetSelectionRange(1, 3)

	doc.GetElementByID("pane").RemoveChild(a)

	if len(doc.selections) != 0 {
		t.Errorf("selections = %d entries after RemoveChild, want 0", len(doc.selections))
	}
	if doc.focused != nil {
		t.Error("focus still points at the removed node")
	}
}
