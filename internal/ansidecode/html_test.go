package ansidecode

import (
	"image/color"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
)

// TestToHTMLParsesAndRoundTripsText confirms the fragment parses as HTML
// and that walking its text nodes recovers exactly the original text,
// escaping included. A broken escapeAttr/escapeText would show up here as
// either a parse error or as mangled node text, not as raw "&" in the DOM.
func TestToHTMLParsesAndRoundTripsText(t *testing.T) {
	lines := []Line{
		{Runs: []Run{
			{Text: "plain", Width: 5},
			{Text: "bold red", Width: 8, Style: Style{FG: color.RGBA{R: 0xff, A: 0xff}, Bold: true}},
		}},
		{Runs: []Run{{Text: "a & b < c > d", Width: 13}}},
	}
	got := ToHTML(lines, HTMLOptions{})

	doc, err := xhtml.Parse(strings.NewReader(got))
	if err != nil {
		t.Fatalf("ToHTML produced unparseable HTML: %v\n%s", err, got)
	}
	var text strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.TextNode {
			text.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	// The pre element's own content ends in "\n" (before "</pre>"), and
	// ToHTML's own trailing "\n" after "</pre>" becomes a second, sibling
	// text node once parsed as a standalone document: harmless in the
	// gallery page this is actually embedded in, but visible here since
	// the test parses the fragment on its own.
	want := "plainbold red\na & b < c > d\n\n"
	if got := text.String(); got != want {
		t.Errorf("recovered text = %q, want %q", got, want)
	}
	if !strings.Contains(got, `color:#ff0000`) {
		t.Errorf("expected a color:#ff0000 declaration in:\n%s", got)
	}
	if !strings.Contains(got, `font-weight:bold`) {
		t.Errorf("expected a font-weight:bold declaration in:\n%s", got)
	}
}

func TestToHTMLPlainRunHasNoSpan(t *testing.T) {
	lines := []Line{{Runs: []Run{{Text: "unstyled", Width: 8}}}}
	got := ToHTML(lines, HTMLOptions{})
	if strings.Contains(got, "<span") {
		t.Errorf("expected no <span> for an unstyled run, got:\n%s", got)
	}
	if !strings.Contains(got, "unstyled") {
		t.Errorf("expected the plain text itself to appear, got:\n%s", got)
	}
}

func TestToHTMLReverseSwapsColors(t *testing.T) {
	lines := []Line{{Runs: []Run{
		{Text: "x", Width: 1, Style: Style{FG: color.RGBA{R: 0xff, A: 0xff}, Reverse: true}},
	}}}
	got := ToHTML(lines, HTMLOptions{Foreground: "#111111", Background: "#222222"})
	// Reversed: the run's own fg (#ff0000) becomes the background, and
	// since no bg was set, the page default foreground (#111111) becomes
	// the text color.
	if !strings.Contains(got, "background-color:#ff0000") {
		t.Errorf("expected background-color:#ff0000 (the run's fg, reversed) in:\n%s", got)
	}
	if !strings.Contains(got, "color:#111111") {
		t.Errorf("expected color:#111111 (the page fg, reversed into text color) in:\n%s", got)
	}
}

func TestToHTMLHyperlink(t *testing.T) {
	lines := []Line{{Runs: []Run{
		{Text: "click", Width: 5, Style: Style{Href: "https://example.com/a&b"}},
	}}}
	got := ToHTML(lines, HTMLOptions{})
	if !strings.Contains(got, `href="https://example.com/a&amp;b"`) {
		t.Errorf("expected an escaped href attribute in:\n%s", got)
	}
}
