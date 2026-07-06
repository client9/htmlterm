package htmlterm

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// Document is a persistent, mutable wrapper around a parsed HTML tree. Unlike
// Renderer.Render, which parses and discards a tree on every call, a Document
// is parsed once and can be queried, mutated (via Element attribute setters),
// and re-rendered repeatedly — the basis for a host-driven interactive loop
// (e.g. a form whose fields are updated in response to keystrokes).
//
// Document.Render does not run Options.StripHiddenInline: that option
// permanently deletes elements from the tree (see stripHiddenInline in
// strip.go), which is appropriate for one-shot sanitization of untrusted
// HTML passed to Renderer.Render but would be destructive and irreversible
// against a tree a host intends to keep mutating.
type Document struct {
	doc  *html.Node
	opts Options
}

// ParseDocument parses htmlStr and returns a Document backed by the
// resulting tree. opts configures rendering the same way it does for New.
func ParseDocument(htmlStr string, opts Options) (*Document, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("htmlterm: %w", err)
	}
	return &Document{doc: doc, opts: opts}, nil
}

// Render renders the document's current tree to a styled terminal string,
// reflecting any mutations made since ParseDocument or the previous Render.
func (d *Document) Render() (string, error) {
	r, err := New(d.opts)
	if err != nil {
		return "", err
	}
	return r.renderTree(d.doc)
}

// GetElementByID returns the first element in document order whose id
// attribute equals id, or nil if none matches.
func (d *Document) GetElementByID(id string) *Element {
	var found *html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && nodeAttr(n, "id") == id {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
			if found != nil {
				return
			}
		}
	}
	walk(d.doc)
	if found == nil {
		return nil
	}
	return &Element{node: found}
}

// QuerySelector returns the first element in document order matching sel,
// or nil if none matches. sel accepts the same selector grammar as CSS rules
// (see CSS.md), including comma-separated selector groups.
func (d *Document) QuerySelector(sel string) *Element {
	var found *html.Node
	d.walkMatching(sel, func(n *html.Node) bool {
		found = n
		return false // stop at first match
	})
	if found == nil {
		return nil
	}
	return &Element{node: found}
}

// QuerySelectorAll returns every element in document order matching sel.
// sel accepts the same selector grammar as CSS rules (see CSS.md), including
// comma-separated selector groups.
func (d *Document) QuerySelectorAll(sel string) []*Element {
	var out []*Element
	d.walkMatching(sel, func(n *html.Node) bool {
		out = append(out, &Element{node: n})
		return true // keep going
	})
	return out
}

// walkMatching parses sel (splitting comma-separated groups the same way
// css.go does for stylesheet rules) and walks the document in order, calling
// visit for each matching element until visit returns false.
func (d *Document) walkMatching(sel string, visit func(n *html.Node) bool) {
	var groups [][]selectorPart
	for _, s := range strings.Split(sel, ",") {
		if s = strings.TrimSpace(s); s != "" {
			groups = append(groups, parseSelector(s))
		}
	}
	var walk func(n *html.Node) bool
	walk = func(n *html.Node) bool {
		if n.Type == html.ElementNode {
			for _, parts := range groups {
				if matchSelector(n, parts) {
					if !visit(n) {
						return false
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if !walk(c) {
				return false
			}
		}
		return true
	}
	walk(d.doc)
}
