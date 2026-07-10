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

// selectHighlightAttr is the reserved marker attribute (see focusAttr) an
// <option> carries while it's the arrow-key-highlighted row within its
// <select>'s open popup — separate from "selected" (the committed value)
// so that browsing options with ArrowUp/ArrowDown while the popup is open
// doesn't change the select's actual value or fire "change" until the
// choice is confirmed (Enter, or clicking an option), matching a real
// HTML <select>'s combobox/listbox keyboard model. Only ever set on an
// option belonging to a currently-open select; openSelectPopup/
// closeSelectPopup/confirmSelectPopup keep it in sync with selectOpenAttr.
const selectHighlightAttr = "data-htmlterm-select-highlight"

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
		if isSelectControl(n) && nodeHasAttr(n, selectOpenAttr) {
			d.closeSelectPopup(n)
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
		if isSelectControl(n) && n != keepSel && nodeHasAttr(n, selectOpenAttr) {
			d.closeSelectPopup(n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(d.doc)
}

// toggleSelectOpen opens sel's dropdown if closed (closing any other open
// select first) or closes it without committing if already open — the
// default action for a click on a focused, closed <select>'s own control,
// or clicking that same control again while open. Enter/Space get their own
// handling in DispatchKey (openSelectPopup to open, confirmSelectPopup to
// commit-and-close) since, unlike a plain click-to-toggle, confirming via
// keyboard needs to commit whatever's highlighted.
func (d *Document) toggleSelectOpen(sel *html.Node) {
	if nodeHasAttr(sel, selectOpenAttr) {
		d.closeSelectPopup(sel)
		return
	}
	d.openSelectPopup(sel)
}

// openSelectPopup opens sel's dropdown (closing any other open select
// first) and seeds the highlighted option (selectHighlightAttr) from
// whichever option is currently selected, or the first option if none is —
// the starting point for arrow-key browsing.
func (d *Document) openSelectPopup(sel *html.Node) {
	d.closeAllSelects()
	setAttr(sel, selectOpenAttr, "")
	d.setSelectHighlight(sel, currentOrFirstOption(sel))
}

// closeSelectPopup closes sel's dropdown without committing whatever option
// is highlighted — sel's actual value (its "selected" option) is left
// exactly as it was before the popup opened. The default action for
// Escape, for clicking the select's own control a second time while open,
// and for focus/click-elsewhere dismissal (closeAllSelects/
// closeSelectsExcept clear selectOpenAttr directly for those paths, which
// is equivalent to this for value purposes since neither ever touched
// "selected").
func (d *Document) closeSelectPopup(sel *html.Node) {
	removeAttr(sel, selectOpenAttr)
	d.clearSelectHighlight(sel)
}

// confirmSelectPopup commits opt as sel's sole selected option, closes the
// dropdown, and dispatches "change" on sel — but only if the value actually
// changed (opening a popup and confirming the same already-selected option
// without moving is a no-op, matching a real <select>). The default action
// for clicking an option in an open select's popup, and for Enter/Space
// while the popup is open (confirming whatever's currently highlighted).
func (d *Document) confirmSelectPopup(sel, opt *html.Node) {
	before := selectValue(sel)
	for _, o := range selectOptionNodes(sel) {
		removeAttr(o, "selected")
	}
	setAttr(opt, "selected", "")
	removeAttr(sel, selectOpenAttr)
	d.clearSelectHighlight(sel)
	if selectValue(sel) != before {
		d.dispatch(sel, "change", "")
	}
}

// moveSelectSelection is the default action for ArrowUp/ArrowDown on a
// focused <select>. Behavior depends on whether the dropdown is open: while
// closed, arrow keys change (and commit) the selection directly, matching a
// real <select>'s closed-control keyboard behavior — every press fires
// "change" immediately, since there's no popup to browse first. While open,
// arrow keys only move the highlighted row (moveSelectHighlight) without
// touching the committed value or firing "change" — matching the
// combobox/listbox convention of browsing options before confirming with
// Enter or a click.
func (d *Document) moveSelectSelection(sel *html.Node, down bool) {
	if nodeHasAttr(sel, selectOpenAttr) {
		d.moveSelectHighlight(sel, down)
		return
	}
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

// currentOrFirstOption returns sel's currently selected option, or its
// first option if none is selected, or nil if it has no options — the
// anchor point openSelectPopup starts highlighting from.
func currentOrFirstOption(sel *html.Node) *html.Node {
	options := selectOptionNodes(sel)
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

// setSelectHighlight marks opt (nil clears without setting a new one) as
// sel's sole highlighted option.
func (d *Document) setSelectHighlight(sel, opt *html.Node) {
	for _, o := range selectOptionNodes(sel) {
		removeAttr(o, selectHighlightAttr)
	}
	if opt != nil {
		setAttr(opt, selectHighlightAttr, "")
	}
}

// clearSelectHighlight removes selectHighlightAttr from every option in
// sel — called whenever sel's popup closes, open or not, so a stale
// highlight never lingers into the next time it opens (openSelectPopup
// always reseeds it anyway, but this keeps state consistent for direct
// inspection between opens too).
func (d *Document) clearSelectHighlight(sel *html.Node) {
	for _, o := range selectOptionNodes(sel) {
		removeAttr(o, selectHighlightAttr)
	}
}

// selectHighlightedOption returns sel's currently highlighted option (see
// selectHighlightAttr), or nil if none is marked.
func selectHighlightedOption(sel *html.Node) *html.Node {
	for _, o := range selectOptionNodes(sel) {
		if nodeHasAttr(o, selectHighlightAttr) {
			return o
		}
	}
	return nil
}

// moveSelectHighlight moves sel's highlighted option to the next/previous
// one (clamped, not wrapping), falling back to currentOrFirstOption if
// nothing is highlighted yet. Does not touch "selected" or dispatch
// "change" — see moveSelectSelection's doc comment.
func (d *Document) moveSelectHighlight(sel *html.Node, down bool) {
	options := selectOptionNodes(sel)
	if len(options) == 0 {
		return
	}
	idx := -1
	for i, o := range options {
		if nodeHasAttr(o, selectHighlightAttr) {
			idx = i
			break
		}
	}
	if idx == -1 {
		anchor := currentOrFirstOption(sel)
		for i, o := range options {
			if o == anchor {
				idx = i
				break
			}
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
	d.setSelectHighlight(sel, options[idx])
}

// applySelectClick runs the default action for a click that hit target,
// where target may be an <option> inside an open select's popup (confirm
// it, closing the popup) or a <select> itself (toggle its dropdown open/
// closed). A no-op for any other target.
func (d *Document) applySelectClick(target *html.Node) {
	if opt := nearestOption(target); opt != nil {
		if sel := nearestSelect(opt); sel != nil && nodeHasAttr(sel, selectOpenAttr) {
			d.confirmSelectPopup(sel, opt)
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
