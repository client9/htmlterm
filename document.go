package htmlterm

import (
	"fmt"
	"strings"
	"unicode/utf8"

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
	doc       *html.Node
	opts      Options
	positions map[*html.Node]Rect // from the most recent Render call; nil before the first one

	listeners      map[*html.Node][]listenerEntry
	nextListenerID listenerID
	focused        *html.Node

	// scrollOffsets holds the current vertical scroll offset for every
	// element that was an overflow:scroll|auto container with a resolved
	// height as of the most recent Render call — rebuilt fresh on every
	// Render (see Renderer.liveScrollOffsets), so a node's presence as a key
	// here is itself the answer to "is this a scroll container right now"
	// (see nearestScrollable). Always non-nil (initialized by ParseDocument)
	// so render-time writes into the shared map never hit a nil map.
	scrollOffsets map[*html.Node]int

	// scrollViewport holds each scroll container's resolved content-box
	// viewport geometry as of the most recent Render call — see the
	// scrollViewport type doc comment for why Rect alone (the CSS border
	// box) isn't enough for this. Internal only (no public getter): used by
	// DispatchKey's PageUp/PageDown and scrollIntoView. Rebuilt fresh every
	// Render, same as scrollOffsets, so it's never stale about which nodes
	// are current scroll containers.
	scrollViewport map[*html.Node]scrollViewport

	// contentOffsets holds, for every block element rendered as of the most
	// recent Render call, the row shift from that element's own Rect.Row
	// (its full CSS border box) down to its first actual content row — see
	// Renderer.liveContentOffsets. Internal only (no public getter): used by
	// tcell_loop.go's focusCursorPos to place a multi-line <textarea>'s
	// cursor on the right visual row. Rebuilt fresh every Render, same as
	// scrollOffsets/scrollViewport. Absent for an element with no border/
	// padding rows above its content (offset 0), or one that isn't
	// display:block at all (e.g. <input>, which never has this ambiguity —
	// see focusCursorPos).
	contentOffsets map[*html.Node]int
}

// scrollViewport records a scroll container's visible content-area geometry
// — the resolved content-box height (heightLines, e.g. from CSS height) and
// the row offset from the container's own Rect.Row down to its first visible
// content row (padding-top plus one more if a border-top rule was drawn).
// Rect itself is the full CSS border box (see Rect's doc comment), which
// includes those same border/padding rows, so it can't be used directly to
// find the viewport's actual height or top row — this is computed
// separately, alongside the scroll offset itself, in block.go.
type scrollViewport struct {
	height    int
	topOffset int
}

// ParseDocument parses htmlStr and returns a Document backed by the
// resulting tree. opts configures rendering the same way it does for New.
func ParseDocument(htmlStr string, opts Options) (*Document, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("htmlterm: %w", err)
	}
	return &Document{doc: doc, opts: opts, scrollOffsets: map[*html.Node]int{}}, nil
}

// Render renders the document's current tree to a styled terminal string,
// reflecting any mutations made since ParseDocument or the previous Render.
// It also refreshes the position map Rect reads from, so a host that mutates
// the tree and re-renders in a loop always has Rects matching what it just
// displayed.
func (d *Document) Render() (string, error) {
	r, err := New(d.opts)
	if err != nil {
		return "", err
	}
	r.scrollOffsets = d.scrollOffsets
	out, positions, scrollOffsets, scrollViewport, contentOffsets := r.renderTree(d.doc)
	d.positions = positions
	d.scrollOffsets = scrollOffsets
	d.scrollViewport = scrollViewport
	d.contentOffsets = contentOffsets
	return out, nil
}

// SetSize updates the width/height the next Render call lays out against,
// without discarding the parsed tree — the mechanism a resize (terminal
// SIGWINCH via Loop, or any other host-driven resize logic) uses to take
// effect. width/height follow Options.Width/Options.Height's conventions
// (a concrete column/line count, or SizeNatural for height). Passing
// SizeAutomatic here is inert, same as it is in Options: resolving it
// requires querying a terminal, which Document has no access to itself —
// see Loop, which resolves it before ever calling SetSize.
func (d *Document) SetSize(width, height int) {
	d.opts.Width = width
	d.opts.Height = height
}

// Size returns the width/height the most recent ParseDocument/SetSize call
// installed — the values the next Render call will lay out against.
func (d *Document) Size() (width, height int) {
	return d.opts.Width, d.opts.Height
}

// DocumentElement returns a handle onto the document's root node. There is
// no separate window-level concept in this package (see Loop's "resize"
// event), so this doubles as that event's dispatch target — register a
// listener via AddEventListener(doc.DocumentElement(), "resize", ...) the
// same way any other element's listeners are registered.
func (d *Document) DocumentElement() *Element {
	return &Element{node: d.doc}
}

// Rect returns el's position and size as of the most recent Render call
// (the CSS border box — content+padding+border, excluding margin — see
// RENDERING.md's Position tracking section for the exact semantics and its
// documented approximations), and whether a position was recorded for it at
// all. A position is recorded for every element that produces its own box
// during composition (block-level elements, tables, lists, inline-block
// elements including form controls, and plain inline elements like <span>/
// <label> reached via token-splicing) — see inline.go's and render.go's
// "default" dispatch cases for exactly which elements that covers, and
// their doc comments for the specific, uncommon combinations (a hyperlink
// or another inline-block wrapping a further trackable descendant) where a
// nested element's own Rect isn't tracked. ok is false if Render hasn't been
// called yet, or if el has no recorded position (e.g. display:none, or one
// of those documented gaps).
func (d *Document) Rect(el *Element) (Rect, bool) {
	if d.positions == nil || el == nil {
		return Rect{}, false
	}
	rect, ok := d.positions[el.node]
	return rect, ok
}

// ScrollTop returns el's current vertical scroll offset (in lines), and
// whether el was a scroll container (overflow:scroll|auto with a resolved
// height) as of the most recent Render call — mirroring the DOM's
// element.scrollTop. ok is false if el is nil or isn't a scroll container.
func (d *Document) ScrollTop(el *Element) (int, bool) {
	if el == nil {
		return 0, false
	}
	offset, ok := d.scrollOffsets[el.node]
	return offset, ok
}

// SetScrollTop sets el's vertical scroll offset directly (e.g. to jump to
// the top of a pane, or restore a previously saved position). The value is
// clamped to the valid range on the next Render call, the same way
// DispatchWheel/DispatchKey-driven scrolling is; it has no effect if el
// isn't (or hasn't yet been rendered as) a scroll container.
func (d *Document) SetScrollTop(el *Element, offset int) {
	if el == nil {
		return
	}
	d.scrollOffsets[el.node] = offset
}

// nearestScrollable returns the nearest ancestor of n (inclusive) that was a
// scroll container (overflow:scroll|auto with a resolved height) as of the
// most recent Render call, or nil if none was. A node's presence as a key in
// d.scrollOffsets, rebuilt fresh every Render, is itself the answer to "is
// this a scroll container right now" — see Renderer.liveScrollOffsets.
func (d *Document) nearestScrollable(n *html.Node) *html.Node {
	chain := ancestorChain(n)
	for i := len(chain) - 1; i >= 0; i-- {
		if _, ok := d.scrollOffsets[chain[i]]; ok {
			return chain[i]
		}
	}
	return nil
}

// nodeDepth counts n's ancestors up to (but not including) the document
// root — used by elementAt to break ties between overlapping Rects in favor
// of the more deeply nested (more specific) element.
func nodeDepth(n *html.Node) int {
	depth := 0
	for cur := n.Parent; cur != nil; cur = cur.Parent {
		depth++
	}
	return depth
}

// elementAt returns the innermost element whose Rect contains (row, col), or
// nil if none does. Multiple recorded Rects can contain the same point (e.g.
// a <label> wrapping an <input> — both cover the click point); the deepest
// node in the tree wins, matching DOM hit-testing semantics.
func (d *Document) elementAt(row, col int) *html.Node {
	var best *html.Node
	bestDepth := -1
	for n, r := range d.positions {
		if row < r.Row || row >= r.Row+r.Height || col < r.Col || col >= r.Col+r.Width {
			continue
		}
		if depth := nodeDepth(n); depth > bestDepth {
			best, bestDepth = n, depth
		}
	}
	return best
}

// DispatchClick hit-tests (row, col) against the position map from the most
// recent Render call, dispatches a "click" event (capture/target/bubble) to
// the innermost matching element, and — unless a listener called
// Event.PreventDefault — runs the built-in default action: toggling a
// checkbox's checked state, checking a radio button and clearing its
// sibling radios (see clearRadioSiblings), or — for a submit control (an
// input[type=submit], or a button whose type is unset or "submit", matching
// HTML's default button type) — dispatching a "submit" event on the nearest
// ancestor <form> (see nearestForm). htmlterm has no navigation/network
// concept, so "submit" is exactly what a listener sees: there's no default
// action for it to prevent. A disabled target (nodeHasAttr(target,
// "disabled")) is inert — matching real browsers, which never fire click on
// a disabled form control at all — so no event is dispatched and no default
// action runs. Returns false if no element was hit.
func (d *Document) DispatchClick(row, col int) bool {
	target := d.elementAt(row, col)
	if target == nil {
		return false
	}
	if nodeHasAttr(target, "disabled") {
		return true
	}
	ev := d.dispatch(target, "click", "")
	if ev.DefaultPrevented() {
		return true
	}
	d.applyCheckToggle(target)
	if isSubmitControl(target) {
		if form := nearestForm(target); form != nil {
			d.dispatch(form, "submit", "")
		}
	}
	return true
}

// wheelScrollLines is how many lines one wheel notch scrolls — matching
// typical terminal/browser wheel step size. decodeSGRMouse (input.go)
// reports one unit per notch; this is where that gets turned into a line
// count.
const wheelScrollLines = 3

// DispatchWheel hit-tests (row, col) against the position map from the most
// recent Render call, then scrolls the nearest scrollable ancestor (an
// element that was an overflow:scroll|auto container with a resolved height
// as of that Render call — see nearestScrollable) by delta wheel notches.
// The new offset is unclamped here; it's clamped to the valid range on the
// next Render call (see block.go's overflow gate), the same "Document holds
// the possibly-stale value, Renderer clamps it next frame" pattern Rect's
// staleness already follows. Returns false if no element was hit or it has
// no scrollable ancestor.
func (d *Document) DispatchWheel(row, col, delta int) bool {
	target := d.elementAt(row, col)
	if target == nil {
		return false
	}
	scrollable := d.nearestScrollable(target)
	if scrollable == nil {
		return false
	}
	d.scrollOffsets[scrollable] += delta * wheelScrollLines
	return true
}

// applyCheckToggle runs the checkbox/radio default action for target: a
// checkbox flips its checked attribute; a radio is checked and its sibling
// radios (see clearRadioSiblings) are cleared. Shared by DispatchClick and
// DispatchKey's space-bar default action. A no-op for any other element.
func (d *Document) applyCheckToggle(target *html.Node) {
	if !strings.EqualFold(target.Data, "input") {
		return
	}
	switch strings.ToLower(nodeAttr(target, "type")) {
	case "checkbox":
		if nodeHasAttr(target, "checked") {
			removeAttr(target, "checked")
		} else {
			setAttr(target, "checked", "")
		}
	case "radio":
		setAttr(target, "checked", "")
		d.clearRadioSiblings(target)
	}
}

// isCheckable reports whether n is an input[type=checkbox] or
// input[type=radio], the elements applyCheckToggle acts on.
func isCheckable(n *html.Node) bool {
	if !strings.EqualFold(n.Data, "input") {
		return false
	}
	typ := strings.ToLower(nodeAttr(n, "type"))
	return typ == "checkbox" || typ == "radio"
}

// isTextEntry reports whether n is a <textarea> or a text-like <input> (any
// type other than checkbox/radio/submit/button/reset/hidden) — the elements
// DispatchKey's printable-character and Backspace default actions act on.
func isTextEntry(n *html.Node) bool {
	tag := strings.ToLower(n.Data)
	if tag == "textarea" {
		return true
	}
	if tag != "input" {
		return false
	}
	switch strings.ToLower(nodeAttr(n, "type")) {
	case "checkbox", "radio", "submit", "button", "reset", "hidden":
		return false
	default:
		return true
	}
}

// isSubmitControl reports whether n is a control that submits its form when
// activated: an input[type=submit], or a button whose type attribute is
// unset or "submit" — HTML's default button type is "submit", so a bare
// <button> counts, but <button type="button"> or <button type="reset">
// don't.
func isSubmitControl(n *html.Node) bool {
	typ := strings.ToLower(nodeAttr(n, "type"))
	switch strings.ToLower(n.Data) {
	case "input":
		return typ == "submit"
	case "button":
		return typ == "" || typ == "submit"
	}
	return false
}

// nearestForm returns n's nearest ancestor <form>, or nil if it has none.
func nearestForm(n *html.Node) *html.Node {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == "form" {
			return p
		}
	}
	return nil
}

// DispatchKey dispatches a "keydown" event (with Event.Key set to key) to
// the currently focused element, and — unless a listener called
// Event.PreventDefault — runs the built-in default action: "Tab" moves
// focus to the next focusable element (FocusNext); "Backspace" drops the
// last rune of a focused text entry's value; a lone space (" ") toggles a
// focused checkbox/radio (applyCheckToggle); "Enter" on a submit control or
// a focused text entry dispatches "submit" on the nearest ancestor <form> —
// matching HTML's implicit-submit-on-Enter behavior for a single-line text
// field; any other single-rune key appends to a focused text entry's (input
// or textarea) value. key follows the convention described in
// INTERACTIVE.md: a single printable rune as a UTF-8 string ("a", "5", " "),
// or a named key from a fixed vocabulary ("Enter", "Backspace", "Tab",
// "Escape", "ArrowUp"/"Down"/"Left"/"Right"). The host owns all
// raw-terminal-byte-to-key-name translation; htmlterm never reads a
// terminal itself. Returns false if nothing is focused.
func (d *Document) DispatchKey(key string) bool {
	if d.focused == nil || key == "" {
		return false
	}
	target := d.focused
	ev := d.dispatch(target, "keydown", key)
	if ev.DefaultPrevented() {
		return true
	}
	switch {
	case key == "Tab":
		d.FocusNext()
	case key == "Backspace":
		if isTextEntry(target) {
			v := nodeAttr(target, "value")
			if v != "" {
				_, size := utf8.DecodeLastRuneInString(v)
				setAttr(target, "value", v[:len(v)-size])
			}
		}
	case key == " " && isCheckable(target):
		d.applyCheckToggle(target)
	case key == "Enter" && strings.ToLower(target.Data) == "textarea":
		// A <textarea> is multi-line, so Enter inserts a newline instead of
		// submitting — matching HTML's implicit-submit-on-Enter behavior,
		// which only applies to single-line text fields (isTextEntry's other
		// members) and submit controls, not <textarea>.
		setAttr(target, "value", nodeAttr(target, "value")+"\n")
	case key == "Enter" && (isSubmitControl(target) || isTextEntry(target)):
		if form := nearestForm(target); form != nil {
			d.dispatch(form, "submit", "")
		}
	case key == "PageUp" || key == "PageDown":
		if scrollable := d.nearestScrollable(target); scrollable != nil {
			step := 1
			if vp, ok := d.scrollViewport[scrollable]; ok && vp.height > 0 {
				step = vp.height
			}
			if key == "PageUp" {
				step = -step
			}
			d.scrollOffsets[scrollable] += step
		}
	case key == "ArrowUp" || key == "ArrowDown":
		if scrollable := d.nearestScrollable(target); scrollable != nil {
			step := 1
			if key == "ArrowUp" {
				step = -1
			}
			d.scrollOffsets[scrollable] += step
		}
	default:
		if r, size := utf8.DecodeRuneInString(key); size == len(key) && r != utf8.RuneError && isTextEntry(target) {
			setAttr(target, "value", nodeAttr(target, "value")+key)
		}
	}
	return true
}

// clearRadioSiblings removes the checked attribute from every other
// input[type=radio] sharing target's name attribute, scoped to the nearest
// ancestor <form> (or the whole document, if target has no <form> ancestor).
func (d *Document) clearRadioSiblings(target *html.Node) {
	name := nodeAttr(target, "name")
	if name == "" {
		return
	}
	scope := d.doc
	if form := nearestForm(target); form != nil {
		scope = form
	}
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n != target && n.Type == html.ElementNode && strings.EqualFold(n.Data, "input") &&
			strings.EqualFold(nodeAttr(n, "type"), "radio") && nodeAttr(n, "name") == name {
			removeAttr(n, "checked")
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(scope)
}

// isFormFocusable reports whether n is a tab-stoppable form control: an
// <input> other than type="hidden", a <button>, a <textarea>, or a
// <select>, and not carrying a disabled attribute. See Document.isFocusable
// for the additional scroll-container case this alone doesn't cover.
func isFormFocusable(n *html.Node) bool {
	if n.Type != html.ElementNode || nodeHasAttr(n, "disabled") {
		return false
	}
	switch strings.ToLower(n.Data) {
	case "input":
		return !strings.EqualFold(nodeAttr(n, "type"), "hidden")
	case "button", "textarea", "select":
		return true
	}
	return false
}

// isFocusable reports whether n is a tab-stoppable element: either a form
// control (isFormFocusable), or — mirroring real browsers' keyboard-
// accessible scroll containers — an overflow:scroll|auto element with a
// resolved height (a key in d.scrollOffsets as of the most recent Render
// call; see nearestScrollable) that has no focusable descendant of its own.
// The scroll-container case exists so a scrollable region with no button/
// input inside it is still Tab-reachable for keyboard-driven scrolling
// (DispatchKey's PageUp/PageDown/ArrowUp/ArrowDown); a container that
// already has a focusable descendant is reached through that descendant
// instead, so it isn't also made its own redundant tab stop. Always false
// for the scroll-container case before the first Render (d.scrollOffsets is
// empty then, same staleness as Rect/ScrollVisible).
func (d *Document) isFocusable(n *html.Node) bool {
	if isFormFocusable(n) {
		return true
	}
	if n.Type != html.ElementNode || nodeHasAttr(n, "disabled") {
		return false
	}
	if _, isScrollContainer := d.scrollOffsets[n]; !isScrollContainer {
		return false
	}
	return !d.hasFocusableDescendant(n)
}

// hasFocusableDescendant reports whether n has any descendant (excluding n
// itself) that isFocusable considers a tab stop — see isFocusable's
// scroll-container case.
func (d *Document) hasFocusableDescendant(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if d.isFocusable(c) || d.hasFocusableDescendant(c) {
			return true
		}
	}
	return false
}

// focusableList returns every focusable element in document order.
func (d *Document) focusableList() []*html.Node {
	var out []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if d.isFocusable(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(d.doc)
	return out
}

// Focus moves focus to el, setting the reserved focusAttr marker (see
// event.go) that ":focus" matches against and dispatching "blur"/"focus"
// events (neither of which bubbles, per DOM semantics) for the previously
// and newly focused elements. Returns false, making no change, if el is nil
// or not focusable (see isFocusable).
func (d *Document) Focus(el *Element) bool {
	if el == nil || !d.isFocusable(el.node) {
		return false
	}
	if d.focused == el.node {
		return true
	}
	prev := d.focused
	d.focused = el.node
	setAttr(el.node, focusAttr, "")
	if prev != nil {
		removeAttr(prev, focusAttr)
		d.dispatch(prev, "blur", "")
	}
	d.scrollIntoView(el.node)
	d.dispatch(el.node, "focus", "")
	return true
}

// scrollIntoView adjusts every scrollable ancestor of n (see
// nearestScrollable, walked here via ancestorChain since more than one
// nested scrollable ancestor may need adjusting) so n's previous-frame Rect
// falls within that ancestor's own previous-frame visible range — mirroring
// a real browser auto-scrolling a newly focused control into view. One-frame
// stale by construction (n's/the ancestor's Rects are only as fresh as the
// last Render call, the same staleness Rect itself already documents), and a
// no-op before the first Render (d.positions is nil).
func (d *Document) scrollIntoView(n *html.Node) {
	elRect, ok := d.positions[n]
	if !ok {
		return
	}
	for _, anc := range ancestorChain(n) {
		offset, isScrollable := d.scrollOffsets[anc]
		if !isScrollable {
			continue
		}
		ancRect, ok := d.positions[anc]
		if !ok {
			continue
		}
		vp := d.scrollViewport[anc]
		contentTop := ancRect.Row + vp.topOffset
		relTop := elRect.Row - contentTop
		relBottom := relTop + elRect.Height - 1
		switch {
		case relTop < 0:
			offset += relTop
		case relBottom >= vp.height:
			offset += relBottom - vp.height + 1
		default:
			continue
		}
		if offset < 0 {
			offset = 0
		}
		d.scrollOffsets[anc] = offset
	}
}

// ScrollVisible reports whether el's Rect, as of the most recent Render
// call, currently falls at least partly within the visible content range of
// every scrollable ancestor it has. Rect itself is never clipped or hidden
// for a scrolled-off element (matching a real scrolled-off DOM element's
// getBoundingClientRect() — see Rect's doc comment), so a host placing its
// own UI (e.g. Loop's terminal cursor, via focusCursorPos) on top of an
// element's Rect needs this to know whether that position is actually
// visible right now, rather than off-screen inside a container that has
// since scrolled past it (e.g. via DispatchWheel/DispatchKey scrolling a
// pane out from under a focused control it contains). True for an element
// with no scrollable ancestor, or before the first Render.
func (d *Document) ScrollVisible(el *Element) bool {
	if el == nil {
		return true
	}
	rect, ok := d.positions[el.node]
	if !ok {
		return true
	}
	for _, anc := range ancestorChain(el.node) {
		if anc == el.node {
			continue
		}
		vp, isScrollable := d.scrollViewport[anc]
		if !isScrollable {
			continue
		}
		ancRect, ok := d.positions[anc]
		if !ok {
			continue
		}
		contentTop := ancRect.Row + vp.topOffset
		relTop := rect.Row - contentTop
		relBottom := relTop + rect.Height - 1
		if relBottom < 0 || relTop >= vp.height {
			return false
		}
	}
	return true
}

// Blur clears the currently focused element, if any, dispatching "blur".
func (d *Document) Blur() {
	if d.focused == nil {
		return
	}
	prev := d.focused
	d.focused = nil
	removeAttr(prev, focusAttr)
	d.dispatch(prev, "blur", "")
}

// FocusedElement returns the currently focused element, or nil if none.
func (d *Document) FocusedElement() *Element {
	if d.focused == nil {
		return nil
	}
	return &Element{node: d.focused}
}

// FocusNext moves focus to the next focusable element in document order
// (wrapping around), and returns it. Returns nil if the document has no
// focusable elements.
func (d *Document) FocusNext() *Element {
	list := d.focusableList()
	if len(list) == 0 {
		return nil
	}
	idx := -1
	for i, n := range list {
		if n == d.focused {
			idx = i
			break
		}
	}
	next := list[(idx+1)%len(list)]
	d.Focus(&Element{node: next})
	return &Element{node: next}
}

// FocusPrev moves focus to the previous focusable element in document order
// (wrapping around), and returns it. Returns nil if the document has no
// focusable elements.
func (d *Document) FocusPrev() *Element {
	list := d.focusableList()
	if len(list) == 0 {
		return nil
	}
	idx := 0
	for i, n := range list {
		if n == d.focused {
			idx = i
			break
		}
	}
	prev := list[(idx-1+len(list))%len(list)]
	d.Focus(&Element{node: prev})
	return &Element{node: prev}
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
