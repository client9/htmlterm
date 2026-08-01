package document

import (
	"strings"

	"golang.org/x/net/html"
)

// selectOpenAttr is the reserved marker attribute (see focusAttr) a <select>
// carries while its dropdown popup is open. toggleSelectOpen/closeAllSelects
// set and clear it; render's compositeOpenSelects (select_popup.go) checks
// for it to decide which selects need their popup spliced into the frame. It
// is threaded through via renderOptions' SelectOpenAttr the same way focusAttr
// is.
const selectOpenAttr = "data-htmlterm-select-open"

// selectHighlightAttr is the reserved marker attribute (see focusAttr) an
// <option> carries while it's the arrow-key-highlighted row within its
// <select>'s open popup. It is separate from "selected", the committed
// value, so that browsing options with ArrowUp and ArrowDown while the popup
// is open doesn't change the select's actual value or fire "change" until the
// choice is confirmed, by Enter or by clicking an option, matching a real
// HTML <select>'s combobox and listbox keyboard model. It is only ever set on
// an option belonging to a currently-open select, and openSelectPopup,
// closeSelectPopup, and confirmSelectPopup keep it in sync with
// selectOpenAttr.
const selectHighlightAttr = "data-htmlterm-select-highlight"

// isSelectControl reports whether n is a <select> element.
func isSelectControl(n *html.Node) bool {
	return n.Type == html.ElementNode && strings.EqualFold(n.Data, "select")
}

// selectOptionNodes returns sel's <option> descendants usable for navigation,
// in document order: direct <option> children, plus, one level deep, the
// <option> children of any direct <optgroup> child. It mirrors
// internal/render's formcontrol.go (see docs/SELECT.md). The two packages
// don't share code (see docs/ARCHITECTURE.md's package-split rationale), so
// this copy
// must be kept in lockstep with that one.
func selectOptionNodes(sel *html.Node) []*html.Node {
	var out []*html.Node
	for c := sel.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch {
		case strings.EqualFold(c.Data, "option"):
			out = append(out, c)
		case strings.EqualFold(c.Data, "optgroup"):
			for gc := c.FirstChild; gc != nil; gc = gc.NextSibling {
				if gc.Type == html.ElementNode && strings.EqualFold(gc.Data, "option") {
					out = append(out, gc)
				}
			}
		}
	}
	return out
}

// optionGroup returns opt's containing <optgroup>, or nil if opt is an
// ungrouped, direct <option> child of its <select>.
func optionGroup(opt *html.Node) *html.Node {
	if opt.Parent != nil && opt.Parent.Type == html.ElementNode && strings.EqualFold(opt.Parent.Data, "optgroup") {
		return opt.Parent
	}
	return nil
}

// optionDisabled reports whether opt is disabled, by its own disabled
// attribute or by its containing <optgroup>'s. A group's disabled attribute
// cascades to every option inside it, matching HTMLOptionElement.disabled's
// real inherited-from-optgroup behavior.
func optionDisabled(opt *html.Node) bool {
	if nodeHasAttr(opt, "disabled") {
		return true
	}
	if g := optionGroup(opt); g != nil && nodeHasAttr(g, "disabled") {
		return true
	}
	return false
}

// nextEnabledOptionIndex returns the next (down) or previous (!down) index in
// options relative to idx that isn't disabled (see optionDisabled), or -1 if
// none remains in that direction. It is shared by moveSelectHighlight and
// moveSelectSelection's arrow-key stepping, so a disabled <option>, whether
// bare or via a disabled <optgroup>, is never landed on by keyboard
// navigation.
func nextEnabledOptionIndex(options []*html.Node, idx int, down bool) int {
	for {
		if down {
			idx++
		} else {
			idx--
		}
		if idx < 0 || idx >= len(options) {
			return -1
		}
		if !optionDisabled(options[idx]) {
			return idx
		}
	}
}

// optionValue returns opt's value attribute, falling back to its trimmed
// text content when absent, mirroring the DOM's HTMLOptionElement.value.
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

// closeAllSelects clears selectOpenAttr from every <select> in the document.
// It is used before opening one, since only one dropdown is ever open at a
// time, and when focus leaves an open select.
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
// keep is inside, meaning nearestSelect(keep), or keep itself if it is
// already a <select>. A nil keep closes all of them.
//
// This is the mechanism behind "click outside dismisses the popup".
// DispatchClick calls it with the click's hit-test target before anything
// else, so a click on unrelated content closes whatever was open, even a
// click on a disabled element or one that misses every element's Rect
// entirely. That is the same way clicking a different <select>'s own control
// closes the first one, via toggleSelectOpen's closeAllSelects.
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

// toggleSelectOpen opens sel's dropdown if closed, closing any other open
// select first, or closes it without committing if already open. It is the
// default action for a click on a focused, closed <select>'s own control, and
// for clicking that same control again while open. Enter and Space get their
// own handling in DispatchKey, openSelectPopup to open and confirmSelectPopup
// to commit and close, since, unlike a plain click-to-toggle, confirming via
// keyboard needs to commit whatever's highlighted.
func (d *Document) toggleSelectOpen(sel *html.Node) {
	if nodeHasAttr(sel, selectOpenAttr) {
		d.closeSelectPopup(sel)
		return
	}
	d.openSelectPopup(sel)
}

// openSelectPopup opens sel's dropdown, closing any other open select first,
// and seeds the highlighted option, via selectHighlightAttr, from whichever
// option is currently selected, or from the first option if none is. That is
// the starting point for arrow-key browsing.
func (d *Document) openSelectPopup(sel *html.Node) {
	d.closeAllSelects()
	setAttr(sel, selectOpenAttr, "")
	d.setSelectHighlight(sel, currentOrFirstOption(sel))
}

// closeSelectPopup closes sel's dropdown without committing whatever option
// is highlighted. sel's actual value, its "selected" option, is left exactly
// as it was before the popup opened. It is the default action for Escape, for
// clicking the select's own control a second time while open, and for
// dismissal by focus change or a click elsewhere. Those last two paths go
// through closeAllSelects and closeSelectsExcept, which clear selectOpenAttr
// directly, equivalent to this for value purposes since neither ever touched
// "selected".
func (d *Document) closeSelectPopup(sel *html.Node) {
	removeAttr(sel, selectOpenAttr)
	d.clearSelectHighlight(sel)
}

// confirmSelectPopup commits opt as sel's sole selected option, closes the
// dropdown, and dispatches "change" on sel, but only if the value changed.
// Opening a popup and confirming the same already-selected option without
// moving is a no-op, matching a real <select>. It is the default action for
// clicking an option in an open select's popup, and for Enter and Space while
// the popup is open, which confirm whatever is currently highlighted.
//
// It is a no-op, leaving the popup open and nothing changed, if opt is
// disabled, whether bare or via a disabled <optgroup>. That is enforced here
// rather than only by applySelectClick's own guard, so every path that can
// reach a highlighted-but-disabled option is covered, not just a direct
// click. One such path is every option in the select being disabled, which
// lets currentOrFirstOption's final fallback seed the highlight on one.
func (d *Document) confirmSelectPopup(sel, opt *html.Node) {
	if optionDisabled(opt) {
		return
	}
	before := selectValue(sel)
	for _, o := range selectOptionNodes(sel) {
		removeAttr(o, "selected")
	}
	setAttr(opt, "selected", "")
	removeAttr(sel, selectOpenAttr)
	d.clearSelectHighlight(sel)
	if selectValue(sel) != before {
		d.dispatch(sel, "change", "", Modifiers{})
	}
}

// moveSelectSelection is the default action for ArrowUp and ArrowDown on a
// focused <select>. Behavior depends on whether the dropdown is open. While
// closed, arrow keys change and commit the selection directly, matching a
// real <select>'s closed-control keyboard behavior, so every press fires
// "change" immediately, there being no popup to browse first. While open,
// arrow keys only move the highlighted row, via moveSelectHighlight, without
// touching the committed value or firing "change", matching the combobox and
// listbox convention of browsing options before confirming with Enter or a
// click.
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
	next := nextEnabledOptionIndex(options, idx, down)
	if next == -1 {
		return
	}
	for _, o := range options {
		removeAttr(o, "selected")
	}
	setAttr(options[next], "selected", "")
	d.dispatch(sel, "change", "", Modifiers{})
}

// currentOrFirstOption returns sel's currently selected option, or its first
// enabled option if none is selected, falling back to its first option of any
// kind if every one is disabled, or nil if it has no options. It is the
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
	for _, o := range options {
		if !optionDisabled(o) {
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
// sel. It is called whenever sel's popup closes, open or not, so a stale
// highlight never lingers into the next time it opens. openSelectPopup
// always reseeds it anyway, but this keeps state consistent for direct
// inspection between opens too.
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

// moveSelectHighlight moves sel's highlighted option to the next or previous
// one, clamped rather than wrapping, falling back to currentOrFirstOption if
// nothing is highlighted yet. It does not touch "selected" or dispatch
// "change"; see moveSelectSelection's doc comment.
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
	next := nextEnabledOptionIndex(options, idx, down)
	if next == -1 {
		return
	}
	d.setSelectHighlight(sel, options[next])
}

// applySelectClick runs the default action for a click that hit target.
// target may be an <option> inside an open select's popup, which confirms it
// and closes the popup, or a <select> itself, which toggles its dropdown open
// or closed. It is a no-op for any other target, including a click on a
// disabled option, whether bare or via a disabled <optgroup>: the popup stays
// open and nothing is confirmed, matching a real <select>. confirmSelectPopup
// itself enforces that too (see its own doc comment), not just this call
// site.
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
// "" if sel has no options, mirroring the DOM's HTMLSelectElement.value.
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
// sel's sole selected option, mirroring the DOM's HTMLSelectElement.value
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
