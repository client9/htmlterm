package render

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestIsZeroValue(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{"empty", "", false},
		{"bare zero", "0", true},
		{"bare zero float", "0.0", true},
		{"px zero", "0px", true},
		{"pt zero", "0pt", true},
		{"percent zero", "0%", true},
		{"em zero", "0.0em", true},
		{"negative zero", "-0px", true},
		{"nonzero px", "10px", false},
		{"nonzero bare", "1", false},
		{"non numeric", "auto", false},
		{"unit only", "px", false},
		{"whitespace padded zero", "  0px  ", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isZeroValue(tc.val); got != tc.want {
				t.Errorf("isZeroValue(%q) = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

func TestIsHiddenStyle(t *testing.T) {
	tests := []struct {
		name  string
		style string
		want  bool
	}{
		{"no style", "", false},
		{"unrelated style", "color: red", false},
		{"display none", "display: none", true},
		{"display block", "display: block", false},
		{"visibility hidden", "visibility: hidden", true},
		{"visibility collapse", "visibility: collapse", true},
		{"visibility visible", "visibility: visible", false},
		{"opacity zero", "opacity: 0", true},
		{"opacity zero float", "opacity: 0.0", true},
		{"opacity nonzero", "opacity: 0.5", false},
		{"height zero with overflow hidden", "height: 0px; overflow: hidden", true},
		{"max-height zero with overflow clip", "max-height: 0; overflow: clip", true},
		{"height zero without overflow hidden", "height: 0px", false},
		{"overflow hidden without zero height", "overflow: hidden", false},
		{"height nonzero with overflow hidden", "height: 10px; overflow: hidden", false},
		// !important-qualified values: regression coverage for
		// stripImportant (css.go) — the "hidden preheader" trick this
		// file's own doc comment names as the motivating case for
		// StripHiddenInline uses !important specifically to defeat webmail
		// CSS resets, so isHiddenStyle must still recognize these.
		{"display none important with space", "display: none !important", true},
		{"display none important no space", "display:none!important", true},
		{"display none important uppercase", "display: none !IMPORTANT", true},
		{"visibility hidden important", "visibility: hidden !important", true},
		{"opacity zero important", "opacity: 0 !important", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isHiddenStyle(tc.style); got != tc.want {
				t.Errorf("isHiddenStyle(%q) = %v, want %v", tc.style, got, tc.want)
			}
		})
	}
}

func TestStripHiddenInlineRemovesSubtree(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(
		`<div><span style="display:none">hidden <b>bold child</b></span><p>visible</p></div>`,
	))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	stripHiddenInline(doc)

	var texts []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode && strings.TrimSpace(n.Data) != "" {
			texts = append(texts, strings.TrimSpace(n.Data))
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if len(texts) != 1 || texts[0] != "visible" {
		t.Errorf("remaining text = %v, want [visible]", texts)
	}
}

func TestStripHiddenInlineNoStyleAttribute(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`<p>hello</p>`))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	stripHiddenInline(doc)

	var found bool
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode && n.Data == "hello" {
			found = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if !found {
		t.Error("unstyled content was unexpectedly removed")
	}
}
