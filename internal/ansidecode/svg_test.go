package ansidecode

import (
	"encoding/xml"
	"image/color"
	"io"
	"strings"
	"testing"
)

// TestToSVGWellFormed confirms the output parses as XML at all. Every
// escaping bug in writeRunText or escapeXMLAttr would otherwise show up
// only as a broken image on GitHub, not a test failure.
func TestToSVGWellFormed(t *testing.T) {
	lines := []Line{
		{Runs: []Run{
			{Text: "plain", Width: 5},
			{Text: "bold red", Width: 8, Style: Style{FG: color.RGBA{R: 0xff, A: 0xff}, Bold: true}},
		}},
		{Runs: []Run{
			{Text: "a & b < c > d \"e\"", Width: 17},
		}},
	}
	svg := ToSVG(lines, SVGOptions{})
	dec := xml.NewDecoder(strings.NewReader(svg))
	for {
		if _, err := dec.Token(); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("ToSVG produced invalid XML: %v\n%s", err, svg)
		}
	}
	if !strings.Contains(svg, "&amp;") {
		t.Errorf("expected the literal & in run text to be escaped as &amp;, got:\n%s", svg)
	}
}

func TestToSVGReverseSwapsColors(t *testing.T) {
	lines := []Line{{Runs: []Run{
		{Text: "x", Width: 1, Style: Style{FG: color.RGBA{R: 0xff, A: 0xff}, Reverse: true}},
	}}}
	svg := ToSVG(lines, SVGOptions{Foreground: "#111111", Background: "#222222"})
	// Reversed: the run's own fg (#ff0000) becomes the background fill, and
	// since no bg was set, the page default foreground becomes the text
	// fill.
	if !strings.Contains(svg, `fill="#ff0000"`) {
		t.Errorf("expected a #ff0000 fill (the run's fg, now the bg rect) in:\n%s", svg)
	}
}

func TestToSVGHyperlinkWrapsAnchor(t *testing.T) {
	lines := []Line{{Runs: []Run{
		{Text: "click", Width: 5, Style: Style{Href: "https://example.com/a&b"}},
	}}}
	svg := ToSVG(lines, SVGOptions{})
	if !strings.Contains(svg, `xlink:href="https://example.com/a&amp;b"`) {
		t.Errorf("expected an escaped xlink:href attribute in:\n%s", svg)
	}
}
