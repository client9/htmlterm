package document

import (
	"strings"

	"golang.org/x/net/html"
)

// selectOpenAttr is the reserved marker attribute (see focusAttr) a <select>
// carries while its dropdown popup is open. toggleSelectOpen/closeAllSelects
// set and clear it; render's compositeOpenSelects (select_popup.go) checks
// for it to decide which selects need their popup spliced into the frame —
// threaded through via renderOptions' SelectOpenAttr the same way focusAttr
// is.
const selectOpenAttr = "data-htmlterm-select-open"

// isSelectControl reports whether n is a <select> element.
func isSelectControl(n *html.Node) bool {
	return n.Type == html.ElementNode && strings.EqualFold(n.Data, "select")
}

// selectOptionNodes returns sel's direct <option> element children, in
// document order — options nested inside an <optgroup> are not supported,
// matching internal/render's formcontrol.go.
func selectOptionNodes(sel *html.Node) []*html.Node {
	var out []*html.Node
	for c := sel.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.EqualFold(c.Data, "option") {
			out = append(out, c)
		}
	}
	return out
}

// optionValue returns opt's value attribute, falling back to its trimmed
// text content when absent — mirroring the DOM's HTMLOptionElement.value.
func optionValue(opt *html.Node) string {
	if v := nodeAttr(opt, "value"); v != "" {
		return v
	}
	return strings.TrimSpace(rawContent(opt))
}

// nearestSelect returns n's nearest ancestor-or-self <select>, or nil.
func nearestSelect(n *html.Node) *html.Node {
	for cur := n; cur != nil; cur = cur.Parent {
		if isSelectControl(cur) {
			return cur
		}
	}
	return nil
}

// nearestOption returns n's nearest ancestor-or-self <option>, or nil.
func nearestOption(n *html.Node) *html.Node {
	for cur := n; cur != nil; cur = cur.Parent {
		if cur.Type == html.ElementNode && strings.EqualFold(cur.Data, "option") {
			return cur
		}
	}
	return nil
}

// closeAllSelects clears selectOpenAttr from every <select> in the document —
// used before opening one (only one dropdown is ever open at a time) and
// when focus leaves an open select.
func (d *Document) closeAllSelects() {
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if isSelectControl(n) {
			removeAttr(n, selectOpenAttr)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(d.doc)
}

// closeSelectsExcept closes every open <select>'s dropdown except the one
// keep is inside (nearestSelect(keep), itself if keep is already a
// <select>) — nil keep closes all. The mechanism behind "click outside
// dismisses the popup": DispatchClick calls this with the click's hit-test
// target before anything else, so a click on unrelated content — even a
// disabled element, or one that misses every element's Rect entirely —
// closes whatever was open, the same way clicking a different <select>'s own
// control closes the first one via toggleSelectOpen's closeAllSelects.
func (d *Document) closeSelectsExcept(keep *html.Node) {
	keepSel := nearestSelect(keep)
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if isSelectControl(n) && n != keepSel {
			removeAttr(n, selectOpenAttr)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(d.doc)
}

// toggleSelectOpen opens sel's dropdown if closed (closing any other open
// select first) or closes it if already open — the default action for a
// click or Enter/Space on a focused, closed <select>.
func (d *Document) toggleSelectOpen(sel *html.Node) {
	if nodeHasAttr(sel, selectOpenAttr) {
		removeAttr(sel, selectOpenAttr)
		return
	}
	d.closeAllSelects()
	setAttr(sel, selectOpenAttr, "")
}

// selectOption marks opt as sel's sole selected option, closes sel's
// dropdown, and dispatches "change" on sel — the default action for clicking
// an option in an open select's popup.
func (d *Document) selectOption(sel, opt *html.Node) {
	for _, o := range selectOptionNodes(sel) {
		if o != opt {
			removeAttr(o, "selected")
		}
	}
	setAttr(opt, "selected", "")
	removeAttr(sel, selectOpenAttr)
	d.dispatch(sel, "change", "")
}

// moveSelectSelection moves sel's selection to the next/previous option
// (clamped, not wrapping) and dispatches "change" — the default action for
// ArrowUp/ArrowDown on a focused <select>, matching a real <select>'s
// keyboard behavior of changing selection directly whether or not the
// dropdown is open.
func (d *Document) moveSelectSelection(sel *html.Node, down bool) {
	options := selectOptionNodes(sel)
	if len(options) == 0 {
		return
	}
	idx := 0
	for i, o := range options {
		if nodeHasAttr(o, "selected") {
			idx = i
			break
		}
	}
	switch {
	case down && idx < len(options)-1:
		idx++
	case !down && idx > 0:
		idx--
	default:
		return
	}
	for _, o := range options {
		removeAttr(o, "selected")
	}
	setAttr(options[idx], "selected", "")
	d.dispatch(sel, "change", "")
}

// applySelectClick runs the default action for a click that hit target,
// where target may be an <option> inside an open select's popup (select it,
// closing the popup) or a <select> itself (toggle its dropdown open/closed).
// A no-op for any other target.
func (d *Document) applySelectClick(target *html.Node) {
	if opt := nearestOption(target); opt != nil {
		if sel := nearestSelect(opt); sel != nil && nodeHasAttr(sel, selectOpenAttr) {
			d.selectOption(sel, opt)
			return
		}
	}
	if isSelectControl(target) {
		d.toggleSelectOpen(target)
	}
}

// selectValue returns sel's currently selected option's value (see
// optionValue), or its first option's value if none is marked selected, or
// "" if sel has no options — mirroring the DOM's HTMLSelectElement.value.
func selectValue(sel *html.Node) string {
	options := selectOptionNodes(sel)
	if len(options) == 0 {
		return ""
	}
	for _, o := range options {
		if nodeHasAttr(o, "selected") {
			return optionValue(o)
		}
	}
	return optionValue(options[0])
}

// setSelectValue marks the option whose value (see optionValue) equals v as
// sel's sole selected option — mirroring the DOM's HTMLSelectElement.value
// setter. Leaves sel unchanged if no option matches.
func setSelectValue(sel *html.Node, v string) {
	options := selectOptionNodes(sel)
	for _, o := range options {
		if optionValue(o) == v {
			for _, oo := range options {
				removeAttr(oo, "selected")
			}
			setAttr(o, "selected", "")
			return
		}
	}
}
