package document

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// datalistOpenAttr is the reserved marker attribute (see focusAttr) an
// <input list> carries while its suggestion popup is open, and
// datalistMatchAttr the one each currently-matching <option> carries. Both
// are set and cleared by refreshDatalist, and threaded through renderOptions'
// DatalistOpenAttr/DatalistMatchAttr the same way focusAttr is.
//
// The matching rule lives here alone. The renderer draws the options carrying
// datalistMatchAttr rather than re-deriving which ones match, so unlike
// selectOptionNodes there is no second copy of the rule in internal/render to
// keep in step. See docs/DATALIST.md.
const (
	datalistOpenAttr  = "data-htmlterm-datalist-open"
	datalistMatchAttr = "data-htmlterm-datalist-match"
)

// The highlighted row reuses selectHighlightAttr rather than getting a marker
// of its own. It already means "the highlighted option in an open popup",
// which is exactly what it means here, and reusing it means an author's
// `option:hover` rule styles both popups with no extra plumbing (see
// cssengine.Cascade.HoverAttr).

// isTextInput reports whether n is an <input> that can carry a datalist: a
// text-like one, the same set isTextEntry covers, minus <textarea>, which has
// no list attribute in HTML.
func isTextInput(n *html.Node) bool {
	return n != nil && n.Type == html.ElementNode &&
		strings.EqualFold(n.Data, "input") && isTextEntry(n)
}

// datalistFor returns the <datalist> named by input's list attribute, or nil
// if the attribute is absent, no element carries that id, or the element that
// does isn't a <datalist>. It mirrors internal/render's own datalistFor; the
// two packages deliberately don't share code (see docs/ARCHITECTURE.md's
// package-split rationale), so this copy must be kept in lockstep with that
// one.
func (d *Document) datalistFor(input *html.Node) *html.Node {
	if !isTextInput(input) {
		return nil
	}
	id := nodeAttr(input, "list")
	if id == "" {
		return nil
	}
	el := d.GetElementByID(id)
	if el == nil || !strings.EqualFold(el.node.Data, "datalist") {
		return nil
	}
	return el.node
}

// datalistOptions returns list's <option> children in document order. Unlike
// selectOptionNodes there is no one-level descent into <optgroup>: HTML
// allows <optgroup> only inside <select>, and a browser's own datalist
// rendering has no grouping either.
func datalistOptions(list *html.Node) []*html.Node {
	var out []*html.Node
	for c := list.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.EqualFold(c.Data, "option") {
			out = append(out, c)
		}
	}
	return out
}

// datalistMatches reports whether an option whose value is optValue should be
// suggested for the text typed so far: a case-insensitive prefix test.
//
// Prefix rather than substring, and case-insensitive, is the convention most
// browsers use for datalist and the least surprising for a terminal
// autocomplete. An empty query matches everything, which is what makes
// ArrowDown on an empty field offer the whole list. See COMPATIBILITY.md.
func datalistMatches(optValue, typed string) bool {
	return strings.HasPrefix(strings.ToLower(optValue), strings.ToLower(typed))
}

// datalistMatchingOptions returns list's options matching typed, in document
// order. Disabled options are skipped, matching how a disabled <option> is
// unselectable in a <select> popup.
func datalistMatchingOptions(list *html.Node, typed string) []*html.Node {
	var out []*html.Node
	for _, o := range datalistOptions(list) {
		if !optionDisabled(o) && datalistMatches(optionValue(o), typed) {
			out = append(out, o)
		}
	}
	return out
}

// refreshDatalist re-evaluates input's suggestion popup against its current
// value: it re-marks the matching options, opens the popup if any match, and
// closes it otherwise. It is the single entry point every value change goes
// through, called after typing, deleting, and pasting, so the popup always
// reflects what is actually in the field.
//
// The popup only opens for the focused input. A programmatic SetValue on some
// other field must not make a popup appear under it, and a value change on an
// input the user has since tabbed away from must not resurrect one.
//
// A no-op for anything that isn't an <input list> pointing at a real
// <datalist>, so the hook sites can call it unconditionally on any text
// entry.
func (d *Document) refreshDatalist(input *html.Node) {
	list := d.datalistFor(input)
	if list == nil {
		return
	}
	if d.focused != input {
		d.closeDatalistPopup(input)
		return
	}
	matches := datalistMatchingOptions(list, nodeAttr(input, "value"))
	if len(matches) == 0 {
		d.closeDatalistPopup(input)
		return
	}
	for _, o := range datalistOptions(list) {
		removeAttr(o, datalistMatchAttr)
	}
	for _, o := range matches {
		setAttr(o, datalistMatchAttr, "")
	}
	setAttr(input, datalistOpenAttr, "")
	// Keep the highlight only if it is still on a matching row; otherwise
	// drop it. Typing another character shouldn't leave a marker pointing at
	// a suggestion that is no longer offered, and shouldn't silently move it
	// to a different one either, since Enter would then commit something the
	// user never browsed to.
	if cur := d.datalistHighlighted(input); cur != nil {
		for _, o := range matches {
			if o == cur {
				return
			}
		}
	}
	d.setDatalistHighlight(input, nil)
}

// dispatchInput dispatches "input" on a text entry whose value a user edit
// just changed, then re-evaluates its suggestion popup against the new value.
//
// Every path that edits a value the user can see goes through here: typing
// and paste (replaceSelection), Backspace and Delete (deleteAt), and cut. The
// refresh is bundled with the event rather than left to each call site so a
// future edit path can't quietly forget it and leave a popup listing
// suggestions for text that is no longer in the field.
//
// The refresh runs after the event, so it reads the settled value: a listener
// is free to rewrite it, and the suggestions should match what the field
// actually ends up holding.
func (d *Document) dispatchInput(target *html.Node, key string, mods Modifiers) {
	d.dispatch(target, "input", key, mods)
	d.refreshDatalist(target)
}

// openDatalistPopup opens input's popup without a value change behind it,
// the ArrowDown-on-a-closed-field path. The filter still applies, so an empty
// field offers everything and a partly-typed one offers what it already
// matches.
func (d *Document) openDatalistPopup(input *html.Node) {
	d.refreshDatalist(input)
}

// closeDatalistPopup closes input's popup, leaving its value alone. It is the
// default action for Escape, and the dismissal path for focus moving away, a
// click elsewhere, and a value that no longer matches anything.
func (d *Document) closeDatalistPopup(input *html.Node) {
	if !nodeHasAttr(input, datalistOpenAttr) && d.datalistFor(input) == nil {
		return
	}
	removeAttr(input, datalistOpenAttr)
	if list := d.datalistFor(input); list != nil {
		for _, o := range datalistOptions(list) {
			removeAttr(o, datalistMatchAttr)
			removeAttr(o, selectHighlightAttr)
		}
	}
}

// closeDatalistsExcept closes every open suggestion popup except keep's own,
// where keep is the click's hit-test target: the input itself, or an option
// row belonging to it. A nil keep closes all of them.
//
// This is the mechanism behind "click outside dismisses the popup", and
// mirrors closeSelectsExcept, which DispatchClick calls in the same place and
// for the same reason.
func (d *Document) closeDatalistsExcept(keep *html.Node) {
	keepInput := d.datalistInputFor(keep)
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n != keepInput && nodeHasAttr(n, datalistOpenAttr) {
			d.closeDatalistPopup(n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(d.doc)
}

// datalistInputFor resolves a click target to the <input> whose popup it
// belongs to: the input itself, or the input whose currently-open popup lists
// the <option> that was hit. Returns nil for anything else.
//
// The option case can't be answered by walking up the tree the way
// nearestSelect does for a <select>'s own options, because a datalist's
// options are children of the <datalist>, not of the input. The link is the
// list attribute, so this walks open inputs and asks each whether it owns the
// option.
func (d *Document) datalistInputFor(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	if isTextInput(n) {
		return n
	}
	opt := nearestOption(n)
	if opt == nil {
		return nil
	}
	var found *html.Node
	var walk func(c *html.Node)
	walk = func(c *html.Node) {
		if found != nil {
			return
		}
		if nodeHasAttr(c, datalistOpenAttr) {
			if list := d.datalistFor(c); list != nil && opt.Parent == list {
				found = c
				return
			}
		}
		for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
			walk(cc)
		}
	}
	walk(d.doc)
	return found
}

// datalistHighlighted returns the option currently marked in input's popup,
// or nil if none is.
func (d *Document) datalistHighlighted(input *html.Node) *html.Node {
	list := d.datalistFor(input)
	if list == nil {
		return nil
	}
	for _, o := range datalistOptions(list) {
		if nodeHasAttr(o, selectHighlightAttr) {
			return o
		}
	}
	return nil
}

// setDatalistHighlight marks opt (nil clears without setting a new one) as
// the sole highlighted row of input's popup.
func (d *Document) setDatalistHighlight(input, opt *html.Node) {
	list := d.datalistFor(input)
	if list == nil {
		return
	}
	for _, o := range datalistOptions(list) {
		removeAttr(o, selectHighlightAttr)
	}
	if opt != nil {
		setAttr(opt, selectHighlightAttr, "")
	}
}

// moveDatalistHighlight moves the highlight to the next or previous suggestion,
// clamped rather than wrapping, matching moveSelectHighlight. With nothing
// highlighted yet, ArrowDown lands on the first row and ArrowUp on the last,
// so one press from a freshly opened popup always selects something.
func (d *Document) moveDatalistHighlight(input *html.Node, down bool) {
	list := d.datalistFor(input)
	if list == nil {
		return
	}
	matches := datalistMatchingOptions(list, nodeAttr(input, "value"))
	if len(matches) == 0 {
		return
	}
	cur := -1
	for i, o := range matches {
		if nodeHasAttr(o, selectHighlightAttr) {
			cur = i
			break
		}
	}
	switch {
	case cur < 0 && down:
		cur = 0
	case cur < 0:
		cur = len(matches) - 1
	case down:
		cur = min(cur+1, len(matches)-1)
	default:
		cur = max(cur-1, 0)
	}
	d.setDatalistHighlight(input, matches[cur])
}

// confirmDatalistOption commits opt as input's value, closes the popup, and
// dispatches "input" then "change".
//
// Both events, in that order, because picking a suggestion is both an edit
// and a commit, which is what a browser does too. That differs from
// confirmSelectPopup, which fires "change" alone: a <select> has no
// intermediate edit state to report, while a text entry does, and a caller
// listening for "input" to track keystrokes would otherwise miss the largest
// value change of all.
//
// The value goes through sanitizeEntryValue, the same path Element.SetValue
// uses, so a stray newline in an option's value can't produce a multi-line
// single-line field. The caret collapses to the end, matching where typing
// the same text by hand would have left it.
func (d *Document) confirmDatalistOption(input, opt *html.Node) {
	if opt == nil || optionDisabled(opt) {
		return
	}
	before := nodeAttr(input, "value")
	v := sanitizeEntryValue(input, optionValue(opt))
	setAttr(input, "value", v)
	caret := utf8.RuneCountInString(v)
	d.setSelection(input, caret, caret, "none")
	d.closeDatalistPopup(input)
	d.dispatch(input, "input", "", Modifiers{})
	if v != before {
		d.dispatch(input, "change", "", Modifiers{})
		// Resync the change baseline, so blurring the field afterward doesn't
		// fire a second "change" for the same edit. See snapshotValueAtFocus.
		d.snapshotValueAtFocus(input)
	}
}

// applyDatalistClick runs the click default action for a suggestion row:
// commit it to the input whose popup it belongs to. A no-op for anything
// else. Called from activate, beside applySelectClick, so a click and a
// keyboard confirm take the same path.
func (d *Document) applyDatalistClick(target *html.Node) {
	opt := nearestOption(target)
	if opt == nil {
		return
	}
	if input := d.datalistInputFor(opt); input != nil {
		d.confirmDatalistOption(input, opt)
	}
}
