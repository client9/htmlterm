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
	doc.dispatch(doc.doc, "resize", "")
	if !fired {
		t.Error("resize listener did not fire")
	}
}
