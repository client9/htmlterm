package render

import (
	"strings"

	"golang.org/x/net/html"
)

// nodeHasAttr reports whether key is present on n, regardless of its value —
// needed for boolean attributes like "checked", where nodeAttr's "" return
// can't distinguish absent from present-but-empty (e.g. checked="").
func nodeHasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

// inputDisplayText synthesizes an <input>'s visual content from its
// attributes rather than children (it has none) — see INTERACTIVE.md's
// "render actual form controls" section. checkbox/radio show a glyph
// reflecting the checked attribute; submit/button/reset show a bracketed
// label (falling back to a type-appropriate default when value is unset);
// hidden renders nothing; every other type (the text-like default) shows
// its value, falling back to its placeholder.
func inputDisplayText(n *html.Node) string {
	typ := strings.ToLower(nodeAttr(n, "type"))
	if typ == "" {
		typ = "text"
	}
	switch typ {
	case "hidden":
		return ""
	case "checkbox":
		if nodeHasAttr(n, "checked") {
			return "☑"
		}
		return "☐"
	case "radio":
		if nodeHasAttr(n, "checked") {
			return "●"
		}
		return "○"
	case "submit", "button", "reset":
		label := nodeAttr(n, "value")
		if label == "" {
			switch typ {
			case "submit":
				label = "Submit"
			case "reset":
				label = "Reset"
			default:
				label = "Button"
			}
		}
		return "[ " + label + " ]"
	default:
		val := nodeAttr(n, "value")
		if val == "" {
			val = nodeAttr(n, "placeholder")
		}
		return "[" + val + "]"
	}
}

// selectOptionNodes returns n's direct <option> element children, in
// document order. Options nested inside an <optgroup> are not supported —
// see CSS.md's <select> entry.
func selectOptionNodes(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.EqualFold(c.Data, "option") {
			out = append(out, c)
		}
	}
	return out
}

// selectOptionLabel returns opt's visible label: its label attribute if set,
// else its concatenated descendant text content.
func selectOptionLabel(opt *html.Node) string {
	if label := nodeAttr(opt, "label"); label != "" {
		return label
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(c *html.Node) {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
		for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
			walk(cc)
		}
	}
	walk(opt)
	return strings.TrimSpace(sb.String())
}

// selectedOption returns n's currently selected <option> — the first one
// carrying a selected attribute, else the first option (matching HTML's
// default-selection rule) — or nil if n has no options.
func selectedOption(n *html.Node) *html.Node {
	options := selectOptionNodes(n)
	if len(options) == 0 {
		return nil
	}
	for _, o := range options {
		if nodeHasAttr(o, "selected") {
			return o
		}
	}
	return options[0]
}

// selectDisplayText synthesizes a closed <select>'s visual content — the
// selected option's label, bracketed with a disclosure indicator. Mirrors
// inputDisplayText's role for <input>. The open dropdown itself is a
// separate compositing step — see compositeOpenSelects in select_popup.go.
func selectDisplayText(n *html.Node) string {
	label := ""
	if sel := selectedOption(n); sel != nil {
		label = selectOptionLabel(sel)
	}
	return "[ " + label + " ▾]"
}
