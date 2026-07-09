package render

import (
	"strconv"
	"strings"

	"github.com/client9/htmlterm/internal/cssengine"
	"golang.org/x/net/html"
)

// stripHiddenInline removes elements whose own inline style="" attribute
// marks them as invisible, along with their children. Only high-confidence
// patterns are matched: display:none, visibility:hidden/collapse, opacity:0,
// and zero height/max-height combined with overflow:hidden (the common
// hidden-preheader trick). Visibility set via a class and a <style> rule is
// out of scope — this only looks at the node's own style attribute.
func stripHiddenInline(doc *html.Node) {
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		c := n.FirstChild
		for c != nil {
			next := c.NextSibling
			if c.Type == html.ElementNode && isHiddenInline(c) {
				n.RemoveChild(c)
			} else {
				walk(c)
			}
			c = next
		}
	}
	walk(doc)
}

func isHiddenInline(n *html.Node) bool {
	style := nodeAttr(n, "style")
	if style == "" {
		return false
	}
	decls := cssengine.ParseDeclarations(style)
	if decls["display"] == "none" {
		return true
	}
	switch decls["visibility"] {
	case "hidden", "collapse":
		return true
	}
	if isZeroValue(decls["opacity"]) {
		return true
	}
	// overflow-y specifically: this heuristic is about vertical clipping
	// (zero height/max-height), and expandShorthand (css.go) now expands a
	// plain overflow:<val> into both overflow-x/overflow-y, so this still
	// matches exactly what it did before that change for every existing
	// caller (which only ever set the shorthand).
	switch decls["overflow-y"] {
	case "hidden", "clip":
		if isZeroValue(decls["height"]) || isZeroValue(decls["max-height"]) {
			return true
		}
	}
	return false
}

// isZeroValue reports whether a CSS length or number value is zero, e.g.
// "0", "0px", "0.0em", "0%".
func isZeroValue(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	for i, r := range v {
		if (r < '0' || r > '9') && r != '.' && r != '-' && r != '+' {
			f, err := strconv.ParseFloat(v[:i], 64)
			return err == nil && f == 0
		}
	}
	f, err := strconv.ParseFloat(v, 64)
	return err == nil && f == 0
}
