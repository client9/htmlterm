package document

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

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
