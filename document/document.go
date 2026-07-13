package document

import (
	"fmt"
	"strings"
	"unicode/utf8"

	htmlterm "github.com/client9/htmlterm"
	"github.com/client9/htmlterm/internal/cssengine"
	"github.com/client9/htmlterm/internal/render"
	"golang.org/x/net/html"
)

type Options = htmlterm.Options

// Rect is a rendered element's position and size in document coordinates.
type Rect struct {
	Row, Col      int
	Width, Height int
}

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
	scrollViewport map[*html.Node]render.Viewport

	// contentOffsets holds, for every block element rendered as of the most
	// recent Render call, the row shift from that element's own Rect.Row
	// (its full CSS border box) down to its first actual content row — see
	// Renderer.liveContentOffsets and the ContentOffset accessor below. Used
	// by tui's focusCursorPos to place a multi-line <textarea>'s cursor on
	// the right visual row. Rebuilt fresh every Render, same as
	// scrollOffsets/scrollViewport. Absent for an element with no border/
	// padding rows above its content (offset 0), or one that isn't
	// display:block at all (e.g. <input>, which never has this ambiguity —
	// see focusCursorPos).
	contentOffsets map[*html.Node]int

	// cachedRenderer and cachedRules memoize the parts of a Render call that
	// are invariant across the Document's whole lifetime, so a host driving
	// repeated renders in a tight loop (e.g. Loop.paint on every keystroke)
	// doesn't re-lex the UA stylesheet + Options.CSS and re-run
	// colorprofile.Detect (an os.Environ scan) on every single frame, the
	// way calling New(d.opts) fresh each time used to. This is safe without
	// any locking or invalidation logic because nothing in Document's public
	// API can change what these two values would resolve to a second time:
	// there is no setter for Options.CSS/IgnoreDocumentCSS/Profile/
	// NoOSC8Links/MaxBlankLines/StripHiddenInline after ParseDocument, and
	// no API to add/remove a <style> element or edit one's text content (see
	// contentOffsets above for the same "the DOM's element-mutation surface
	// is attribute-only" reasoning) — so cachedRules (r.rules plus any
	// <style>-element rules, via documentRules) never goes stale either.
	// Only Options.Width/Height (SetSize) and scrollOffsets change per
	// render; Render refreshes cachedRenderer's copies of those directly
	// rather than invalidating the cache. Like every other Document field,
	// this assumes the existing single-goroutine-mutates-Document contract
	// (see CLAUDE.md's "no locking in the interactive layer" invariant) — it
	// is not safe to call Render concurrently on the same Document, but
	// nothing in Document ever has been.
	cachedEngine *render.Engine
	cachedRules  []cssengine.Rule
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

func renderOptions(opts Options) render.Options {
	return render.Options{
		CSS:                 opts.CSS,
		Stylesheets:         opts.Stylesheets,
		Width:               opts.Width,
		Height:              opts.Height,
		IgnoreDocumentCSS:   opts.IgnoreDocumentCSS,
		Profile:             opts.Profile,
		NoOSC8Links:         opts.NoOSC8Links,
		MaxBlankLines:       opts.MaxBlankLines,
		StripHiddenInline:   opts.StripHiddenInline,
		FocusAttr:           focusAttr,
		SelectOpenAttr:      selectOpenAttr,
		SelectHighlightAttr: selectHighlightAttr,
	}
}

// Render renders the document's current tree to a styled terminal string,
// reflecting any mutations made since ParseDocument or the previous Render.
// It also refreshes the position map Rect reads from, so a host that mutates
// the tree and re-renders in a loop always has Rects matching what it just
// displayed.
func (d *Document) Render() (string, error) {
	if d.cachedEngine == nil {
		engine, err := render.New(renderOptions(d.opts))
		if err != nil {
			return "", err
		}
		d.cachedEngine = engine
		d.cachedRules = engine.DocumentRules(d.doc)
	}
	result := d.cachedEngine.RenderNode(d.doc, render.Request{
		Width:         d.opts.Width,
		Height:        d.opts.Height,
		Rules:         d.cachedRules,
		ScrollOffsets: d.scrollOffsets,
	})
	d.positions = convertPositions(result.Positions)
	d.scrollOffsets = result.ScrollOffsets
	d.scrollViewport = result.ScrollViewport
	d.contentOffsets = result.ContentOffsets
	return result.Output, nil
}

func convertPositions(in map[*html.Node]render.Rect) map[*html.Node]Rect {
	if len(in) == 0 {
		return nil
	}
	out := make(map[*html.Node]Rect, len(in))
	for n, rect := range in {
		out[n] = Rect{
			Row:    rect.Row,
			Col:    rect.Col,
			Width:  rect.Width,
			Height: rect.Height,
		}
	}
	return out
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
// no separate window-level concept in this package, so this doubles as
// DispatchResize's target — register a listener via
// AddEventListener(doc.DocumentElement(), "resize", ...) the same way any
// other element's listeners are registered.
func (d *Document) DocumentElement() *Element {
	return &Element{node: d.doc, doc: d}
}

// rect is Element.Rect's implementation — see its doc comment. Lives on
// Document, rather than being computed by Element directly, because it reads
// d.positions, the whole-tree position map from the most recent Render call.
func (d *Document) rect(el *Element) (Rect, bool) {
	if d.positions == nil || el == nil {
		return Rect{}, false
	}
	rect, ok := d.positions[el.node]
	return rect, ok
}

// ContentOffset returns the row shift from el's own Rect.Row (its full CSS
// border box — see Rect's doc comment) down to el's first actual content
// row, as of the most recent Render call — e.g. for placing a cursor inside
// a multi-line <textarea> on the right visual row rather than assuming
// content starts at Rect.Row. ok is false if el is nil, Render hasn't been
// called yet, or el has no recorded offset (no border/padding rows above
// its content, or not a block-level element at all — see contentOffsets'
// doc comment).
func (d *Document) ContentOffset(el *Element) (int, bool) {
	if el == nil {
		return 0, false
	}
	offset, ok := d.contentOffsets[el.node]
	return offset, ok
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

// elementAt returns the innermost element whose Rect contains (row, col), or
// nil if none does. Multiple recorded Rects can contain the same point (e.g.
// a <label> wrapping an <input> — both cover the click point); the deepest
// node in the tree wins, matching DOM hit-testing semantics. Ties at equal
// depth (which shouldn't arise from normal box layout, but would from
// overlapping Rects at the same nesting level) are broken by document order,
// keeping the first one walked — deterministic, unlike ranging over
// d.positions directly, whose Go map iteration order varies from call to
// call.
func (d *Document) elementAt(row, col int) *html.Node {
	var best *html.Node
	bestDepth := -1
	var walk func(n *html.Node, depth int)
	walk = func(n *html.Node, depth int) {
		if r, ok := d.positions[n]; ok &&
			row >= r.Row && row < r.Row+r.Height && col >= r.Col && col < r.Col+r.Width &&
			depth > bestDepth {
			best, bestDepth = n, depth
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, depth+1)
		}
	}
	walk(d.doc, 0)
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
// action runs. Any open <select> dropdown other than one target is itself
// inside (see closeSelectsExcept) is closed first, unconditionally — a click
// anywhere else, including on a disabled element or entirely outside every
// element's Rect, dismisses it, matching a real dropdown's click-outside
// behavior. Returns false if no element was hit.
func (d *Document) DispatchClick(row, col int) bool {
	target := d.elementAt(row, col)
	d.closeSelectsExcept(target)
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
	d.applySelectClick(target)
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

// DispatchResize dispatches a "resize" event targeting DocumentElement (the
// document's root node) — the mechanism a host driving terminal resizes
// (tui's Loop, on a SIGWINCH-derived tcell.EventResize) uses after calling
// SetSize, since htmlterm has no re-layout concept of its own beyond what
// SetSize already did; a listener reacts to the new size via Size/Rect.
// There is no default action to prevent.
func (d *Document) DispatchResize() {
	d.dispatch(d.doc, "resize", "")
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
	case (key == "Enter" || key == " ") && isSelectControl(target):
		if nodeHasAttr(target, selectOpenAttr) {
			if opt := selectHighlightedOption(target); opt != nil {
				d.confirmSelectPopup(target, opt)
			} else {
				d.closeSelectPopup(target)
			}
		} else {
			d.openSelectPopup(target)
		}
	case key == "Escape" && isSelectControl(target):
		d.closeSelectPopup(target)
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
			if vp, ok := d.scrollViewport[scrollable]; ok && vp.Height > 0 {
				step = vp.Height
			}
			if key == "PageUp" {
				step = -step
			}
			d.scrollOffsets[scrollable] += step
		}
	case (key == "ArrowUp" || key == "ArrowDown") && isSelectControl(target):
		d.moveSelectSelection(target, key == "ArrowDown")
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

// focus is Element.Focus's implementation — see its doc comment. Lives on
// Document, rather than being done by Element directly, because it needs to
// find and clear whichever other node was previously focused (d.focused).
func (d *Document) focus(el *Element) bool {
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
		if isSelectControl(prev) {
			d.closeSelectPopup(prev)
		}
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
		contentTop := ancRect.Row + vp.TopOffset
		relTop := elRect.Row - contentTop
		relBottom := relTop + elRect.Height - 1
		switch {
		case relTop < 0:
			offset += relTop
		case relBottom >= vp.Height:
			offset += relBottom - vp.Height + 1
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
		contentTop := ancRect.Row + vp.TopOffset
		relTop := rect.Row - contentTop
		relBottom := relTop + rect.Height - 1
		if relBottom < 0 || relTop >= vp.Height {
			return false
		}
	}
	return true
}

// blur is Element.Blur's implementation, unconditionally clearing whatever
// is currently focused — Element.Blur itself is what checks that the
// receiver is actually the focused node (matching a real blur()'s no-op
// behavior otherwise) before calling this.
func (d *Document) blur() {
	prev := d.focused
	d.focused = nil
	removeAttr(prev, focusAttr)
	if isSelectControl(prev) {
		d.closeSelectPopup(prev)
	}
	d.dispatch(prev, "blur", "")
}

// FocusedElement returns the currently focused element, or nil if none.
func (d *Document) FocusedElement() *Element {
	if d.focused == nil {
		return nil
	}
	return &Element{node: d.focused, doc: d}
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
	next := &Element{node: list[(idx+1)%len(list)], doc: d}
	d.focus(next)
	return next
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
	prev := &Element{node: list[(idx-1+len(list))%len(list)], doc: d}
	d.focus(prev)
	return prev
}

// SetInnerHTML parses htmlStr as an HTML fragment (parsed in el's own
// context, the same rule ParseFragment uses — e.g. a fragment containing
// bare <tr>s needs el to itself be a <table>/<tbody> for the fragment parser
// to accept them) and replaces el's children with the result, discarding
// el's previous children entirely. This is the mechanism for injecting
// structural, host-controlled content — a freshly-fetched envelope table, a
// rendered email body — into a container that a Loop is actively driving,
// without needing to replace the Document itself (Loop holds one Document
// pointer for its whole run; there is no document-swap API, so a container
// declared once up front and refreshed via SetInnerHTML is the supported
// pattern for content that changes shape, as opposed to attribute-driven
// mutation for content that doesn't — see INTERACTIVE.md's ImportHTML note,
// which this supersedes).
//
// The fragment must not itself contain <style> elements: Document caches its
// resolved stylesheet rules once, from the tree ParseDocument first saw (see
// cachedRules), and SetInnerHTML does not invalidate that cache — a
// <style>-bearing fragment would silently have no effect on the cascade.
// Page-level CSS belongs in Options.CSS/Stylesheets, set once at
// ParseDocument time; SetInnerHTML is for markup, not styling.
//
// If the currently focused element is inside the replaced subtree, focus is
// silently cleared (no "blur" dispatched — the element is gone, not
// blurred) rather than left dangling on a detached node. Any event listeners
// registered on now-detached descendants become unreachable (the same
// listener-leak behavior a real DOM has when you drop a subtree without
// removeEventListener) — call RemoveEventListener first if that matters.
// scrollOffsets/scrollViewport/contentOffsets need no such cleanup: all
// three are rebuilt wholesale on the next Render, so stale entries for
// removed nodes are simply dropped rather than lingering (see their own doc
// comments on Document).
func (d *Document) SetInnerHTML(el *Element, htmlStr string) error {
	if el == nil {
		return fmt.Errorf("htmlterm: SetInnerHTML on nil element")
	}
	nodes, err := html.ParseFragment(strings.NewReader(htmlStr), el.node)
	if err != nil {
		return fmt.Errorf("htmlterm: %w", err)
	}
	for c := el.node.FirstChild; c != nil; {
		next := c.NextSibling
		el.node.RemoveChild(c)
		c = next
	}
	for _, n := range nodes {
		el.node.AppendChild(n)
	}
	if d.focused != nil && !isDescendant(d.doc, d.focused) {
		d.focused = nil
	}
	return nil
}

// SetPreRendered replaces el's content with ansi verbatim, bypassing HTML
// parsing and — critically — sanitizeTerminalText (see SECURITY.md's
// "Trusted pre-rendered content"). It does this by attaching a single
// html.RawNode child carrying ansi as its Data. html.RawNode is never
// produced by html.Parse/ParseFragment (see its doc comment in
// golang.org/x/net/html: "RawNode nodes are not returned by the parser, but
// can be part of the Node tree passed to func Render to insert raw
// HTML")— so a RawNode can only enter a Document's tree via this method.
// The trust boundary is which code path built the node, not a naming
// convention on a string: SetInnerHTML's input, however it's spelled, can
// never become one.
//
// ansi is intended to be the output of a prior Document.Render() (or
// htmlterm.Renderer.Render()) call — i.e. content this package already
// rendered (and, in producing it, already sanitized) once. It is inserted
// exactly as given, with no re-wrapping: word-wrap and overflow-y
// scroll-clipping still apply around it the same as any other content (both
// are already ANSI-aware — see wordWrapANSI/textutil.go — so embedded
// SGR/OSC8 sequences are treated as zero-width), but SetPreRendered performs
// no validation that ansi's line widths actually match el's resolved
// content width; a caller re-using stale ansi after a resize will see it
// clipped or padded to the new width without being rewrapped to it.
//
// The caller is solely responsible for ansi's trustworthiness: passing
// unsanitized, attacker-controlled, or otherwise arbitrary text here
// reintroduces exactly the terminal-escape-injection risk
// sanitizeTerminalText exists to prevent. Only pass content this package
// itself produced.
func (d *Document) SetPreRendered(el *Element, ansi string) {
	if el == nil {
		return
	}
	for c := el.node.FirstChild; c != nil; {
		next := c.NextSibling
		el.node.RemoveChild(c)
		c = next
	}
	el.node.AppendChild(&html.Node{Type: html.RawNode, Data: ansi})
	if d.focused != nil && !isDescendant(d.doc, d.focused) {
		d.focused = nil
	}
}

// isDescendant reports whether n is root or a descendant of root, by walking
// up n's parent chain — used by SetInnerHTML to detect a focused node that
// just got cut out of the tree.
func isDescendant(root, n *html.Node) bool {
	for cur := n; cur != nil; cur = cur.Parent {
		if cur == root {
			return true
		}
	}
	return false
}

// CreateElement returns a new element node with tag as its tag name,
// detached from the tree — mirroring the DOM's Document.createElement.
// Attach it with Element.AppendChild or Element.InsertBefore.
func (d *Document) CreateElement(tag string) *Element {
	return &Element{node: &html.Node{Type: html.ElementNode, Data: tag}, doc: d}
}

// CreateTextNode returns a new text node carrying text, detached from the
// tree — mirroring the DOM's Document.createTextNode. Attach it with
// Element.AppendChild or Element.InsertBefore.
func (d *Document) CreateTextNode(text string) *Element {
	return &Element{node: &html.Node{Type: html.TextNode, Data: text}, doc: d}
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
	return &Element{node: found, doc: d}
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
	return &Element{node: found, doc: d}
}

// QuerySelectorAll returns every element in document order matching sel.
// sel accepts the same selector grammar as CSS rules (see CSS.md), including
// comma-separated selector groups.
func (d *Document) QuerySelectorAll(sel string) []*Element {
	var out []*Element
	d.walkMatching(sel, func(n *html.Node) bool {
		out = append(out, &Element{node: n, doc: d})
		return true // keep going
	})
	return out
}

// walkMatching parses sel (splitting comma-separated groups the same way
// css.go does for stylesheet rules) and walks the document in order, calling
// visit for each matching element until visit returns false.
func (d *Document) walkMatching(sel string, visit func(n *html.Node) bool) {
	group := cssengine.ParseSelectorGroup(sel)
	var walk func(n *html.Node) bool
	walk = func(n *html.Node) bool {
		if n.Type == html.ElementNode {
			if group.Match(n, focusAttr) {
				if !visit(n) {
					return false
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
