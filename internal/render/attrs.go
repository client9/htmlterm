package render

import (
	"strings"

	"golang.org/x/net/html"
)

const defaultFocusAttr = "data-htmlterm-focus"

// defaultSelectOpenAttr is the reserved marker attribute (see defaultFocusAttr)
// a <select> carries while its dropdown popup is open. It is set and cleared
// by document's toggleSelectOpen and checked here by compositeOpenSelects
// (select_popup.go) to decide which selects need their popup composited into
// the frame.
const defaultSelectOpenAttr = "data-htmlterm-select-open"

// defaultSelectHighlightAttr is the reserved marker attribute (see
// defaultFocusAttr) an <option> carries while it's the arrow-key-highlighted
// row within its <select>'s open popup. It is set and cleared by document's
// openSelectPopup, moveSelectHighlight, confirmSelectPopup, and
// closeSelectPopup, and checked here by compositeSelectPopup
// (select_popup.go) to decide which option gets the "▸" marker. It falls back
// to the "selected" attribute when no option carries this one, as with a
// popup force-opened directly in markup, with no live browsing session behind
// it (see compositeSelectPopup).
const defaultSelectHighlightAttr = "data-htmlterm-select-highlight"

// defaultSelectionStartAttr and defaultSelectionEndAttr are the reserved
// marker attributes (see defaultFocusAttr) a focused text entry carries while
// it has a non-collapsed selection. They are set and cleared together by
// document's Document.setSelection, holding the rune-offset start and end as
// decimal strings. selectionRange (formcontrol.go) reads them to resolve the
// ::selection highlight range for a text-like <input> or <textarea>. They are
// absent, rather than "0", whenever the selection is collapsed, so their mere
// presence already implies a non-empty range without needing to compare the
// two values first.
const (
	defaultSelectionStartAttr = "data-htmlterm-selection-start"
	defaultSelectionEndAttr   = "data-htmlterm-selection-end"
)

func nodeAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// rawContent returns the concatenated text of all descendant text nodes.
func rawContent(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func plainInlineText(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}
