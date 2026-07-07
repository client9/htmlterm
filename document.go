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
// It also refreshes the position map Rect reads from, so a host that mutates
// the tree and re-renders in a loop always has Rects matching what it just
// displayed.
func (d *Document) Render() (string, error) {
	r, err := New(d.opts)
	if err != nil {
		return "", err
	}
	out, positions := r.renderTree(d.doc)
	d.positions = positions
	return out, nil
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
// action for it to prevent. Returns false if no element was hit.
func (d *Document) DispatchClick(row, col int) bool {
	target := d.elementAt(row, col)
	if target == nil {
		return false
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
	case key == "Enter" && (isSubmitControl(target) || isTextEntry(target)):
		if form := nearestForm(target); form != nil {
			d.dispatch(form, "submit", "")
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

// isFocusable reports whether n is a tab-stoppable form control: an <input>
// other than type="hidden", a <button>, a <textarea>, or a <select>, and not
// carrying a disabled attribute.
func isFocusable(n *html.Node) bool {
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

// focusableList returns every focusable element in document order.
func (d *Document) focusableList() []*html.Node {
	var out []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if isFocusable(n) {
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
	if el == nil || !isFocusable(el.node) {
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
	d.dispatch(el.node, "focus", "")
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
