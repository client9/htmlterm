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
