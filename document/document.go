package document

import (
	"fmt"
	"sort"
	"strconv"
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

	// selections holds the current caret/selection state for any text-entry
	// element that has one explicitly recorded — see selectionState and the
	// selection/setSelection accessors below, and
	// docs/proposals/CARET_SELECTION.md for the design this implements. An
	// absent entry means "collapsed at the end of value", exactly matching
	// this package's pre-existing (and still default) append/trim-at-end
	// behavior — so a Document that never calls SetSelectionRange or the
	// selection-aware key handling this map is the foundation for behaves
	// identically to before this field existed. Lazily initialized, same as
	// listeners/valueAtFocus below. Unlike the render-derived maps further
	// down, this one isn't rebuilt each frame, so entries for nodes cut out
	// of the tree are swept explicitly — see pruneDetachedState.
	selections map[*html.Node]selectionState

	// valueAtFocus snapshots a text-entry element's "value" attribute at the
	// moment it becomes focused, so focus/blur/DispatchKey's Enter-commit
	// branch can tell whether the value actually changed since — the
	// condition real DOM requires before firing "change" (as opposed to
	// "input", which fires on every mutating keystroke regardless). Lazily
	// initialized, mirroring listeners above; entries are removed once
	// consumed (on blur, on focus moving elsewhere, or on Enter-commit) so
	// the map never grows past the number of currently-focused elements
	// (always at most one).
	valueAtFocus map[*html.Node]string

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

	// scrollOffsetsX/scrollViewportX are scrollOffsets/scrollViewport's
	// horizontal-scrollbar counterparts — see nearestScrollableX,
	// tryScrollCapClickX, and docs/SCROLLING.md's horizontal-scrolling
	// addendum. Always non-nil, same as scrollOffsets.
	scrollOffsetsX  map[*html.Node]int
	scrollViewportX map[*html.Node]render.ViewportX

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

	// contentOffsetsX is contentOffsets' horizontal counterpart: the column
	// shift from an element's own Rect.Col (its full CSS border box) right
	// to its first actual content column — border-left plus padding-left
	// plus the horizontal margin Rect doesn't subtract back out (see Rect's
	// doc comment). Needed for exactly the same reason contentOffsets is,
	// one axis over: a <textarea> carries a border and padding-left from the
	// UA stylesheet, so neither tui's focusCursorPos nor caretIndexFromClick
	// can treat Rect.Col as the value's first character. Rebuilt fresh every
	// Render, same as contentOffsets.
	contentOffsetsX map[*html.Node]int

	// cachedEngine memoizes the part of a Render call that is invariant
	// across the Document's whole lifetime, so a host driving repeated
	// renders in a tight loop (e.g. Loop.paint on every keystroke) doesn't
	// re-lex the UA stylesheet + Options.CSS and re-run colorprofile.Detect
	// (an os.Environ scan) on every single frame, the way calling New(d.opts)
	// fresh each time used to. This is safe without any locking or
	// invalidation logic because nothing in Document's public API can change
	// what it would resolve to a second time: there is no setter for
	// Options.CSS/IgnoreDocumentCSS/Profile/NoOSC8Links/MaxBlankLines/
	// StripHiddenInline after ParseDocument. Only Options.Width/Height
	// (SetSize) and scrollOffsets change per render; Render refreshes
	// cachedEngine's copies of those directly rather than invalidating the
	// cache. Like every other Document field, this assumes the existing
	// single-goroutine-mutates-Document contract (see CLAUDE.md's "no
	// locking in the interactive layer" invariant) — it is not safe to call
	// Render concurrently on the same Document, but nothing in Document ever
	// has been.
	//
	// cachedRules memoizes the resolved stylesheet rule set (r.rules plus
	// any <style>-element rules, via cachedEngine.DocumentRules) as of the
	// tree ParseDocument first saw or the most recent rules recompute.
	// Unlike cachedEngine, this *can* go stale: the generic tree-mutation
	// API (Element.AppendChild/InsertBefore/RemoveChild/ReplaceChild,
	// Document.SetInnerHTML) has no tag-name restriction, so a caller can
	// attach or detach a <style> element after the first Render. Every one
	// of those mutation sites calls markRulesStale on the subtree it
	// touches, which sets rulesStale whenever that subtree contains a
	// <style> element; Render recomputes cachedRules from the current tree
	// whenever rulesStale is set, instead of reusing a stale snapshot. This
	// only covers a <style> element being attached/detached — editing an
	// already-attached <style> element's text content in place (there is no
	// dedicated API for that; see markRulesStale) is not detected. Inline
	// style="" attributes need no such tracking at all: they're read fresh
	// off the live node by cssengine.Cascade.Resolve on every single
	// RenderNode call, never folded into cachedRules.
	cachedEngine *render.Engine
	cachedRules  []cssengine.Rule
	rulesStale   bool
}

// markRulesStale sets d.rulesStale if n or any of its descendants is a
// <style> element — called from every tree-structural mutation entry point
// (Element.AppendChild/InsertBefore/RemoveChild/ReplaceChild,
// Document.SetInnerHTML) on whichever subtree the mutation attaches or
// detaches, so cachedRules's doc comment invariant holds: a <style> element
// entering or leaving the tree always causes the next Render to recompute
// cachedRules instead of silently reusing a stale rule set. A no-op if d is
// nil (an Element constructed outside a Document) or n is nil.
func markRulesStale(d *Document, n *html.Node) {
	if d == nil || n == nil || d.rulesStale {
		return
	}
	if hasStyleDescendant(n) {
		d.rulesStale = true
	}
}

// hasStyleDescendant reports whether n itself, or any node in its subtree,
// is a <style> element — markRulesStale's test for whether a mutated
// subtree can affect the resolved stylesheet rule set.
func hasStyleDescendant(n *html.Node) bool {
	if n.Type == html.ElementNode && strings.EqualFold(n.Data, "style") {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if hasStyleDescendant(c) {
			return true
		}
	}
	return false
}

// ParseDocument parses htmlStr and returns a Document backed by the
// resulting tree. opts configures rendering the same way it does for New.
func ParseDocument(htmlStr string, opts Options) (*Document, error) {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, fmt.Errorf("htmlterm: %w", err)
	}
	return &Document{doc: doc, opts: opts, scrollOffsets: map[*html.Node]int{}, scrollOffsetsX: map[*html.Node]int{}}, nil
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
		SelectionStartAttr:  selectionStartAttr,
		SelectionEndAttr:    selectionEndAttr,
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
	} else if d.rulesStale {
		d.cachedRules = d.cachedEngine.DocumentRules(d.doc)
	}
	d.rulesStale = false
	result := d.cachedEngine.RenderNode(d.doc, render.Request{
		Width:          d.opts.Width,
		Height:         d.opts.Height,
		Rules:          d.cachedRules,
		ScrollOffsets:  d.scrollOffsets,
		ScrollOffsetsX: d.scrollOffsetsX,
	})
	d.positions = convertPositions(result.Positions)
	d.scrollOffsets = result.ScrollOffsets
	d.scrollViewport = result.ScrollViewport
	d.scrollOffsetsX = result.ScrollOffsetsX
	d.scrollViewportX = result.ScrollViewportX
	d.contentOffsets = result.ContentOffsets
	d.contentOffsetsX = result.ContentOffsetsX
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

// ContentOffsetX is ContentOffset's horizontal counterpart: the column shift
// from el's own Rect.Col (its full CSS border box — see Rect's doc comment)
// right to el's first actual content column, as of the most recent Render
// call — e.g. for placing a cursor inside a bordered, padded <textarea> on
// the right visual column rather than assuming content starts at Rect.Col.
// ok is false if el is nil, Render hasn't been called yet, or el has no
// recorded offset (no border/padding/margin columns left of its content, or
// not a block-level element at all — see contentOffsetsX's doc comment).
func (d *Document) ContentOffsetX(el *Element) (int, bool) {
	if el == nil {
		return 0, false
	}
	offset, ok := d.contentOffsetsX[el.node]
	return offset, ok
}

// selectionState is a text-entry element's live caret/selection position —
// rune offsets into its current "value", with start <= end always enforced
// by setSelection (never by the zero value directly, which is only ever
// read through selection's default-filling below). direction mirrors real
// DOM's selectionDirection: "forward" (end is the moving edge), "backward"
// (start is), or "none" (a fresh collapsed caret, or a range set without a
// direction). This is live property state, not a reflected HTML attribute —
// see docs/proposals/CARET_SELECTION.md's "Data model" section for why that
// matches real spec's own split rather than introducing a new one.
type selectionState struct {
	start, end int
	direction  string
}

// selection returns n's current selection state, defaulting to a collapsed
// caret at the end of its value with direction "none" — the well-defined
// fallback for a text entry that has never had setSelection called on it,
// matching this package's pre-existing (and still default) append-at-end
// behavior exactly. The stored state (if any) is re-clamped against n's
// current value length on every read, so an external mutation of "value"
// (e.g. a direct Element.SetAttribute("value", ...) call bypassing SetValue)
// can never leave a stale offset pointing past the end of a shorter value.
func (d *Document) selection(n *html.Node) selectionState {
	valueLen := utf8.RuneCountInString(nodeAttr(n, "value"))
	s, ok := d.selections[n]
	if !ok {
		return selectionState{start: valueLen, end: valueLen, direction: "none"}
	}
	s.start = clampInt(s.start, 0, valueLen)
	s.end = clampInt(s.end, 0, valueLen)
	if s.start > s.end {
		s.start, s.end = s.end, s.start
	}
	return s
}

// setSelection records n's selection state, clamping start/end to n's
// current value length (in runes) and swapping them if start > end — real
// setSelectionRange silently reorders an inverted range rather than
// erroring. direction is normalized to "none" unless it's exactly "forward"
// or "backward", mirroring setSelectionRange's own handling of an
// unrecognized third argument.
func (d *Document) setSelection(n *html.Node, start, end int, direction string) {
	if n == nil {
		return
	}
	valueLen := utf8.RuneCountInString(nodeAttr(n, "value"))
	start = clampInt(start, 0, valueLen)
	end = clampInt(end, 0, valueLen)
	if start > end {
		start, end = end, start
	}
	if direction != "forward" && direction != "backward" {
		direction = "none"
	}
	if d.selections == nil {
		d.selections = make(map[*html.Node]selectionState)
	}
	d.selections[n] = selectionState{start: start, end: end, direction: direction}
	syncSelectionAttrs(n, start, end)
}

// syncSelectionAttrs mirrors a text entry's [start, end) selection range
// onto its own selectionStartAttr/selectionEndAttr marker attributes (see
// their doc comments) — the only channel internal/render has for reading
// Document's selection state, the same reserved-attribute pattern
// focus/blur already use for focusAttr. Removes both attributes entirely
// when collapsed (start == end): a collapsed caret has nothing for
// internal/render's ::selection highlight to render, and doing so also
// means selectionRange's "has" check (formcontrol.go) can treat mere
// presence of both attributes as proof of a well-formed, non-empty range
// without comparing the two values itself.
func syncSelectionAttrs(n *html.Node, start, end int) {
	if start == end {
		removeAttr(n, selectionStartAttr)
		removeAttr(n, selectionEndAttr)
		return
	}
	setAttr(n, selectionStartAttr, strconv.Itoa(start))
	setAttr(n, selectionEndAttr, strconv.Itoa(end))
}

// clearSelection removes any explicitly recorded selection state for n, so
// the next selection call falls back to its default (collapsed at end) —
// called by SetValue, mirroring real spec's "setting .value collapses
// selection to the end" behavior for programmatic value assignment. Also
// clears the mirrored selectionStartAttr/selectionEndAttr marker
// attributes (syncSelectionAttrs), if any, for the same reason.
func (d *Document) clearSelection(n *html.Node) {
	delete(d.selections, n)
	removeAttr(n, selectionStartAttr)
	removeAttr(n, selectionEndAttr)
}

// clampInt clamps v to [lo, hi].
func clampInt(v, lo, hi int) int {
	return min(max(v, lo), hi)
}

// scrollTop is Element.ScrollTop's implementation: el's current vertical
// scroll offset (in lines), and whether el was a scroll container
// (overflow:scroll|auto with a resolved height) as of the most recent Render
// call — mirroring the DOM's element.scrollTop. ok is false if el is nil or
// isn't a scroll container.
func (d *Document) scrollTop(el *Element) (int, bool) {
	if el == nil {
		return 0, false
	}
	offset, ok := d.scrollOffsets[el.node]
	return offset, ok
}

// setScrollTop is Element.SetScrollTop's implementation: sets el's vertical
// scroll offset directly (e.g. to jump to the top of a pane, or restore a
// previously saved position). The value is clamped to the valid range on the
// next Render call, the same way DispatchWheel/DispatchKey-driven scrolling
// is; it has no effect if el isn't (or hasn't yet been rendered as) a scroll
// container.
func (d *Document) setScrollTop(el *Element, offset int) {
	if el == nil {
		return
	}
	d.scrollOffsets[el.node] = offset
}

// scrollLeft is Element.ScrollLeft's implementation, scrollTop's horizontal
// counterpart: reports el's current horizontal scroll offset and whether el
// was (as of the most recent Render call) an overflow-x:scroll|auto
// container with an explicit width.
func (d *Document) scrollLeft(el *Element) (int, bool) {
	if el == nil {
		return 0, false
	}
	offset, ok := d.scrollOffsetsX[el.node]
	return offset, ok
}

// setScrollLeft is Element.SetScrollLeft's implementation, setScrollTop's
// horizontal counterpart: sets el's horizontal scroll offset directly. The
// value is clamped to the valid range on the next Render call, the same way
// DispatchWheel/DispatchKey-driven scrolling is; it has no effect if el isn't
// (or hasn't yet been rendered as) a horizontal scroll container.
func (d *Document) setScrollLeft(el *Element, offset int) {
	if el == nil {
		return
	}
	d.scrollOffsetsX[el.node] = offset
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

// nearestScrollableX is nearestScrollable's horizontal counterpart: the
// nearest ancestor of n (inclusive) that was an overflow-x:scroll|auto
// container with an explicit width as of the most recent Render call. Kept
// as its own ancestor walk (not a shared axis-parameterized helper) since
// the nearest horizontally-scrollable ancestor and nearest
// vertically-scrollable ancestor can legitimately differ for a nested pane.
func (d *Document) nearestScrollableX(n *html.Node) *html.Node {
	chain := ancestorChain(n)
	for i := len(chain) - 1; i >= 0; i-- {
		if _, ok := d.scrollOffsetsX[chain[i]]; ok {
			return chain[i]
		}
	}
	return nil
}

// tryScrollCapClick reports whether (row, col) lands on scrollable's
// ::scrollbar-cap-start or ::scrollbar-cap-end cell — see render.Viewport's
// GutterCol/GutterWidth/CapStart/CapEnd doc comment — and if so, scrolls it
// by one line (mirroring DispatchKey's own ArrowUp/ArrowDown step) and
// reports true. scrollable's own Rect anchors GutterCol/TopOffset, the same
// Rect-plus-Viewport composition scrollIntoView/ScrollVisible already use.
// Unclamped, like every other d.scrollOffsets mutation site — clamped on
// the next Render call. No "click" Event is dispatched for a cap hit: a cap
// is rendering chrome, not real element content, so this intentionally
// mirrors DispatchWheel's direct-mutation shape rather than DispatchClick's
// own event-dispatch path.
func (d *Document) tryScrollCapClick(scrollable *html.Node, row, col int) bool {
	vp, ok := d.scrollViewport[scrollable]
	if !ok || vp.GutterWidth == 0 {
		return false
	}
	rect, ok := d.positions[scrollable]
	if !ok {
		return false
	}
	colStart := rect.Col + vp.GutterCol
	if col < colStart || col > colStart+vp.GutterWidth-1 {
		return false
	}
	top := rect.Row + vp.TopOffset
	bottom := top + vp.Height - 1
	switch {
	case vp.CapStart && row == top:
		d.scrollOffsets[scrollable]--
		return true
	case vp.CapEnd && row == bottom:
		d.scrollOffsets[scrollable]++
		return true
	}
	return false
}

// tryScrollCapClickX is tryScrollCapClick's horizontal counterpart: reports
// whether (row, col) lands on scrollable's ::scrollbar-cap-start-x
// (leftmost content column) or ::scrollbar-cap-end-x (rightmost content
// column) cell — see render.ViewportX's doc comment — and if so, scrolls it
// by one column and reports true. Same unclamped-write/no-Event-dispatch
// shape as tryScrollCapClick.
func (d *Document) tryScrollCapClickX(scrollable *html.Node, row, col int) bool {
	vp, ok := d.scrollViewportX[scrollable]
	if !ok || vp.GutterHeight == 0 {
		return false
	}
	rect, ok := d.positions[scrollable]
	if !ok {
		return false
	}
	rowStart := rect.Row + vp.GutterRow
	if row < rowStart || row > rowStart+vp.GutterHeight-1 {
		return false
	}
	left := rect.Col + vp.LeftOffset
	right := left + vp.Width - 1
	switch {
	case vp.CapStart && col == left:
		d.scrollOffsetsX[scrollable]--
		return true
	case vp.CapEnd && col == right:
		d.scrollOffsetsX[scrollable]++
		return true
	}
	return false
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
// action runs. A click on a focusable text entry also focuses it and
// positions its caret at the clicked rune (see focusAndPositionCaret) —
// mirroring a real browser's mousedown-driven focus-plus-caret-placement,
// folded into "click" here since htmlterm only synthesizes one click event
// kind (see COMPATIBILITY.md); Shift held extends the existing selection to
// the click point instead of collapsing it, matching real Shift+Click. This
// runs before the "click" event dispatches below, same as real
// mousedown-before-click ordering, and unconditionally — a listener calling
// PreventDefault on "click" doesn't un-focus a field any more than it does
// in a real browser. Any open <select> dropdown other than one target is
// itself inside (see closeSelectsExcept) is closed first, unconditionally —
// a click anywhere else, including on a disabled element or entirely
// outside every element's Rect, dismisses it, matching a real dropdown's
// click-outside behavior. A click landing on a scrollable ancestor's
// ::scrollbar-cap-start/::scrollbar-cap-end cell (see tryScrollCapClick), or
// a horizontally-scrollable ancestor's ::scrollbar-cap-start-x/
// ::scrollbar-cap-end-x cell (see tryScrollCapClickX), is handled before any
// of that — it scrolls by one line/column and returns, the same
// no-event-dispatch shape DispatchWheel already has, since a cap is
// rendering chrome, not real element content. Returns false if no element
// was hit.
func (d *Document) DispatchClick(row, col int, mods Modifiers) bool {
	target := d.elementAt(row, col)
	d.closeSelectsExcept(target)
	if scrollable := d.nearestScrollable(target); scrollable != nil && d.tryScrollCapClick(scrollable, row, col) {
		return true
	}
	if scrollableX := d.nearestScrollableX(target); scrollableX != nil && d.tryScrollCapClickX(scrollableX, row, col) {
		return true
	}
	if target == nil {
		return false
	}
	if nodeHasAttr(target, "disabled") {
		return true
	}
	if isTextEntry(target) && d.isFocusable(target) {
		d.focusAndPositionCaret(target, row, col, mods.Shift)
	}
	ev := d.dispatch(target, "click", "", mods)
	if ev.DefaultPrevented() {
		return true
	}
	d.applyCheckToggle(target)
	d.applySelectClick(target)
	if isSubmitControl(target) {
		if form := nearestForm(target); form != nil {
			d.dispatch(form, "submit", "", Modifiers{})
		}
	}
	return true
}

// focusAndPositionCaret focuses target (a text entry hit by a click) and
// sets its caret to the rune position (row, col) corresponds to
// (caretIndexFromClick) — DispatchClick's click-to-caret default action;
// see its doc comment and docs/proposals/CARET_SELECTION.md. With
// shiftExtend, the click extends the selection from its current anchor
// (anchorFocus) to the new position instead of collapsing to it, matching
// real Shift+Click. The anchor is read before d.focus runs, though nothing
// about focus() itself would change it (d.selections is keyed by node, not
// by focus state) — this is just the more obviously-correct order to read
// from.
func (d *Document) focusAndPositionCaret(target *html.Node, row, col int, shiftExtend bool) {
	idx := d.caretIndexFromClick(target, row, col)
	anchor, _ := anchorFocus(d.selection(target))
	d.focus(&Element{node: target, doc: d})
	if shiftExtend {
		d.setSelection(target, anchor, idx, directionOf(anchor, idx))
		return
	}
	d.setSelection(target, idx, idx, "none")
}

// caretIndexFromClick computes the rune offset into target's value that
// (row, col) corresponds to — the reverse of the same column model tui's
// focusCursorPos uses to place the terminal's own hardware cursor: column
// offset from the box's left edge (a text-like <input> renders its value
// plain, with no bracket or other prefix to account for — see
// docs/proposals/CARET_SELECTION.md), and, for a <textarea>, row *and*
// column offsets from its first content cell (d.contentOffsets/
// d.contentOffsetsX) rather than its full border box — the UA stylesheet
// gives every <textarea> a border and one column of padding on each side,
// so Rect.Col is two columns left of where its text actually starts. The
// column-to-offset step goes through render.RuneIndexForColumn rather than
// plain subtraction, since a rune is not always one cell wide (CJK, emoji).
// Returns 0 if target has no recorded Rect (e.g. Render hasn't run since
// target was attached).
func (d *Document) caretIndexFromClick(target *html.Node, row, col int) int {
	rect, ok := d.positions[target]
	if !ok {
		return 0
	}
	value := nodeAttr(target, "value")
	if strings.ToLower(target.Data) != "textarea" {
		return clampInt(render.RuneIndexForColumn(value, col-rect.Col), 0, utf8.RuneCountInString(value))
	}
	lines := strings.Split(value, "\n")
	offset := d.contentOffsets[target]
	offsetX := d.contentOffsetsX[target]
	lineIdx := clampInt(row-rect.Row-offset, 0, len(lines)-1)
	lineCol := clampInt(render.RuneIndexForColumn(lines[lineIdx], col-rect.Col-offsetX),
		0, utf8.RuneCountInString(lines[lineIdx]))
	idx := lineCol
	for i := 0; i < lineIdx; i++ {
		idx += utf8.RuneCountInString(lines[i]) + 1 // +1 for that line's own "\n"
	}
	return idx
}

// wheelScrollLines is how many lines one wheel notch scrolls — matching
// typical terminal/browser wheel step size. decodeSGRMouse (input.go)
// reports one unit per notch; this is where that gets turned into a line
// count.
const wheelScrollLines = 3

// DispatchWheel hit-tests (row, col) against the position map from the most
// recent Render call, then scrolls the nearest scrollable ancestor on each
// axis independently: deltaY wheel notches vertically against the nearest
// overflow-y:scroll|auto ancestor with a resolved height (see
// nearestScrollable), and deltaX wheel notches horizontally against the
// nearest overflow-x:scroll|auto ancestor with an explicit width (see
// nearestScrollableX) — these can legitimately be two different elements
// for a nested pane. Either axis is a no-op if its own delta is 0 or no
// matching scrollable ancestor exists; Returns true if at least one axis
// actually scrolled. New offsets are unclamped here; they're clamped to
// their valid range on the next Render call (see block.go's overflow
// gates), the same "Document holds the possibly-stale value, Renderer
// clamps it next frame" pattern Rect's staleness already follows.
func (d *Document) DispatchWheel(row, col, deltaX, deltaY int) bool {
	target := d.elementAt(row, col)
	if target == nil {
		return false
	}
	scrolled := false
	if deltaY != 0 {
		if scrollable := d.nearestScrollable(target); scrollable != nil {
			d.scrollOffsets[scrollable] += deltaY * wheelScrollLines
			scrolled = true
		}
	}
	if deltaX != 0 {
		if scrollable := d.nearestScrollableX(target); scrollable != nil {
			d.scrollOffsetsX[scrollable] += deltaX * wheelScrollLines
			scrolled = true
		}
	}
	return scrolled
}

// DispatchResize dispatches a "resize" event targeting DocumentElement (the
// document's root node) — the mechanism a host driving terminal resizes
// (tui's Loop, on a SIGWINCH-derived tcell.EventResize) uses after calling
// SetSize, since htmlterm has no re-layout concept of its own beyond what
// SetSize already did; a listener reacts to the new size via Size/Rect.
// There is no default action to prevent.
func (d *Document) DispatchResize() {
	d.dispatch(d.doc, "resize", "", Modifiers{})
}

// dispatchEvent is Element.DispatchEvent's actual logic — the same
// Document-holds-the-logic split Focus/Blur/DispatchClick already use for
// their own Element wrappers. Checks ev.dispatching itself, before calling
// runDispatch, so a blocked reentrant call (the same *Event already
// mid-dispatch elsewhere) returns false unconditionally rather than
// evaluating Cancelable/DefaultPrevented against ev's stale, pre-existing
// state (see docs/proposals/CUSTOM_EVENTS.md's "Fixing the reentrancy
// return value").
func (d *Document) dispatchEvent(el *Element, ev *Event) bool {
	if ev.dispatching {
		return false
	}
	d.runDispatch(ev, el.node, ev.Bubbles)
	return !(ev.Cancelable && ev.DefaultPrevented())
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

// isEditable reports whether n is a text entry whose value the built-in
// default actions are allowed to modify: a text entry (isTextEntry) that is
// neither readonly nor disabled. This is the "user edit" gate only —
// matching real HTML, where readonly blocks typing/pasting/cutting but not
// focus, caret movement, selection, or programmatic assignment through
// Element.SetValue, and where a readonly control still submits with its
// form. A disabled control can't normally be focused at all (isFocusable),
// but a host can disable one that already is, so it's checked here too
// rather than assumed unreachable.
func isEditable(n *html.Node) bool {
	return isTextEntry(n) && !nodeHasAttr(n, "readonly") && !nodeHasAttr(n, "disabled")
}

// snapshotValueAtFocus records n's current value as the baseline a later
// commitChange call compares against, if n is a text entry — called when n
// becomes focused. A no-op for non-text-entry elements (e.g. <select>, which
// tracks and fires its own "change" independently — see select.go).
func (d *Document) snapshotValueAtFocus(n *html.Node) {
	if !isTextEntry(n) {
		return
	}
	if d.valueAtFocus == nil {
		d.valueAtFocus = make(map[*html.Node]string)
	}
	d.valueAtFocus[n] = nodeAttr(n, "value")
}

// commitChange dispatches "change" on n if it's a text entry whose value
// differs from the baseline snapshotValueAtFocus recorded, then updates (or,
// if clearBaseline is true, removes) that baseline — matching real DOM's
// "fire only once per distinct commit" behavior for a text field's "change"
// event, unlike "input" which fires on every mutating keystroke. Called
// wherever a text entry's value can be considered committed: losing focus
// (blur, or focus moving to another element) and Enter on a single-line
// field (DispatchKey's submit branch). clearBaseline is true for the former
// (no further comparisons are possible once nothing is focused there) and
// false for the latter (the field stays focused, so typing can resume and a
// later blur/Enter should compare against the just-committed value, not the
// original focus-time one).
func (d *Document) commitChange(n *html.Node, clearBaseline bool) {
	if !isTextEntry(n) {
		return
	}
	v := nodeAttr(n, "value")
	if d.valueAtFocus[n] != v {
		d.dispatch(n, "change", "", Modifiers{})
	}
	if clearBaseline {
		delete(d.valueAtFocus, n)
	} else {
		d.valueAtFocus[n] = v
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
// focus to the next focusable element (FocusNext), or the previous one
// (FocusPrev) with Shift held; "Backspace"/"Delete" on a focused editable
// text entry (isEditable — readonly and disabled controls take no edits,
// though they still focus, select, and move their caret normally) delete its current selection, or the one rune before/
// after the caret if collapsed (see deleteAt); "ArrowLeft"/"ArrowRight" move
// or (with Shift) extend the caret by one rune, collapsing an existing
// selection to its near edge first if Shift isn't held (see
// moveCaretHorizontal); "Home"/"End" move or extend to the start/end of the
// caret's current "\n"-delimited line — for a single-line <input>, its only
// line is the whole value (see moveCaretToLineEdge); Ctrl+"a"/"A" selects
// the entire value; a lone space (" ") toggles a focused checkbox/radio
// (applyCheckToggle); "Enter" on a submit control or a focused text entry
// fires "change" (if the value differs from its value at focus time — see
// commitChange) and dispatches "submit" on the nearest ancestor <form> —
// matching HTML's implicit-submit-on-Enter behavior for a single-line text
// field; any other single-rune key replaces a focused text entry's current
// selection (or inserts at the caret if collapsed — see replaceSelection).
// Every text-entry value mutation above (Backspace/Delete, a <textarea>'s
// Enter-newline, and typed-character insertion) also dispatches "input" on
// the target, once per keystroke, distinct from "change" which only fires
// on commit (Enter, or losing focus — see Document.focus/blur); a readonly
// or disabled field dispatches neither, since nothing about its value
// changed; pure caret
// movement (Arrow*/Home/End/Ctrl+A) dispatches neither, matching real DOM
// (no value mutation, nothing to report). See
// docs/proposals/CARET_SELECTION.md for the full caret/selection design.
// key follows the convention described in docs/INTERACTIVE.md: a single
// printable rune as a UTF-8 string ("a", "5", " "), or a named key from a
// fixed vocabulary ("Enter", "Backspace", "Delete", "Tab", "Escape", "Home",
// "End", "ArrowUp"/"Down"/"Left"/"Right"). mods records which modifier keys
// were held, mirroring a real KeyboardEvent's ctrlKey/shiftKey/altKey/
// metaKey — copied onto the dispatched Event, and now consulted by the
// caret-movement default actions above (Shift to extend a selection, Ctrl
// for select-all). The host owns all raw-terminal-byte-to-key-name/modifier
// translation; htmlterm never reads a terminal itself. Returns false if
// nothing is focused.
func (d *Document) DispatchKey(key string, mods Modifiers) bool {
	if d.focused == nil || key == "" {
		return false
	}
	target := d.focused
	ev := d.dispatch(target, "keydown", key, mods)
	if ev.DefaultPrevented() {
		return true
	}
	switch {
	case key == "Tab":
		// Shift+Tab walks the tab order backwards, same as every real UA.
		// A host whose terminal reports it as its own key name instead of
		// Tab-with-Shift (tcell's legacy KeyBacktab, say) is expected to
		// translate that into ("Tab", Modifiers{Shift: true}) — see
		// tui/tcell_loop.go's keyName.
		if mods.Shift {
			d.FocusPrev()
		} else {
			d.FocusNext()
		}
	case key == "Backspace" && isEditable(target):
		d.deleteAt(target, key, mods, false)
	case key == "Delete" && isEditable(target):
		d.deleteAt(target, key, mods, true)
	case mods.Ctrl && (key == "a" || key == "A") && isTextEntry(target):
		v := nodeAttr(target, "value")
		d.setSelection(target, 0, utf8.RuneCountInString(v), "forward")
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
		// members) and submit controls, not <textarea>. A readonly (or
		// disabled) textarea swallows Enter entirely rather than falling
		// through to the implicit-submit branch below — a <textarea> never
		// implicit-submits, editable or not.
		if isEditable(target) {
			d.replaceSelection(target, key, "\n", mods)
		}
	case key == "Enter" && (isSubmitControl(target) || isTextEntry(target)):
		d.commitChange(target, false)
		if form := nearestForm(target); form != nil {
			d.dispatch(form, "submit", "", Modifiers{})
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
	case (key == "ArrowLeft" || key == "ArrowRight") && isTextEntry(target):
		d.moveCaretHorizontal(target, key == "ArrowRight", mods.Shift)
	case (key == "Home" || key == "End") && isTextEntry(target):
		d.moveCaretToLineEdge(target, key == "End", mods.Shift)
	case key == "ArrowLeft" || key == "ArrowRight":
		// Reserved landing spot for horizontal scrolling on anything that
		// isn't a text entry — text entries claim ArrowLeft/ArrowRight for
		// caret movement above (see moveCaretHorizontal).
		if scrollable := d.nearestScrollableX(target); scrollable != nil {
			step := 1
			if key == "ArrowLeft" {
				step = -1
			}
			d.scrollOffsetsX[scrollable] += step
		}
	default:
		if r, size := utf8.DecodeRuneInString(key); size == len(key) && r != utf8.RuneError && isEditable(target) {
			d.replaceSelection(target, key, key, mods)
		}
	}
	return true
}

// deleteAt implements Backspace's (forward=false) and Delete's (forward=
// true) default action for a focused text entry: deletes the current
// selection if non-collapsed (via deleteSelectionRange), otherwise the one
// rune immediately before or after the caret. Dispatches "input" only if
// something was actually deleted — an empty value's Backspace, or a caret
// already at the end's Delete, changes nothing and fires nothing, matching
// real DOM (no "input" event for a no-op edit).
func (d *Document) deleteAt(target *html.Node, key string, mods Modifiers, forward bool) {
	sel := d.selection(target)
	if sel.start != sel.end {
		d.deleteSelectionRange(target, sel)
		d.dispatch(target, "input", key, mods)
		return
	}
	v := []rune(nodeAttr(target, "value"))
	var newValue []rune
	var caret int
	switch {
	case !forward && sel.start > 0:
		newValue = append(append([]rune{}, v[:sel.start-1]...), v[sel.start:]...)
		caret = sel.start - 1
	case forward && sel.start < len(v):
		newValue = append(append([]rune{}, v[:sel.start]...), v[sel.start+1:]...)
		caret = sel.start
	default:
		return
	}
	setAttr(target, "value", string(newValue))
	d.setSelection(target, caret, caret, "none")
	d.dispatch(target, "input", key, mods)
}

// deleteSelectionRange removes sel's [start, end) span from target's value
// and collapses the caret to sel.start — the shared "delete whatever's
// currently selected" step used by deleteAt (Backspace/Delete over a
// non-collapsed selection) and DispatchCut's range-cut path (see below).
// Does not itself dispatch "input" — callers do that once, after deciding
// whether anything actually changed.
func (d *Document) deleteSelectionRange(target *html.Node, sel selectionState) {
	v := []rune(nodeAttr(target, "value"))
	newValue := append(append([]rune{}, v[:sel.start]...), v[sel.end:]...)
	setAttr(target, "value", string(newValue))
	d.setSelection(target, sel.start, sel.start, "none")
}

// stripNewlines removes every CR/LF from s — HTML's "strip newlines from the
// value" sanitization, which applies to every single-line text control
// (everything isTextEntry accepts except <textarea>, which is the only
// multi-line one). Without it a pasted or programmatically assigned
// multi-line string would put a literal "\n" inside an inline <input>, which
// this renderer honors as a real line break and which therefore tears the
// surrounding line apart.
func stripNewlines(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return s // overwhelmingly the common case; avoid the rewrite entirely
	}
	return strings.NewReplacer("\r\n", "", "\r", "", "\n", "").Replace(s)
}

// sanitizeEntryValue applies stripNewlines to text unless n is a <textarea>,
// the one text entry whose value is legitimately multi-line — the shared
// gate every path that writes a text entry's value from caller-supplied
// content goes through (replaceSelection, covering both DispatchPaste and
// typed characters; Element.SetValue). Element.SetAttribute("value", ...)
// deliberately doesn't: it's the raw-attribute escape hatch, matching real
// DOM, where sanitization is a property of the value *setter*, not of the
// content attribute.
func sanitizeEntryValue(n *html.Node, text string) string {
	if strings.EqualFold(n.Data, "textarea") {
		return text
	}
	return stripNewlines(text)
}

// replaceSelection implements typed-character insertion and <textarea>'s
// Enter-newline: replaces target's current selection (or, if collapsed,
// inserts at the caret) with text, then collapses the caret to just after
// the inserted text — matching real spec, where typing over a selection
// always collapses afterward rather than leaving a new selection — and
// dispatches "input". text is newline-sanitized for every single-line
// control (sanitizeEntryValue), so a multi-line paste into an <input>
// arrives as one line rather than breaking the layout.
func (d *Document) replaceSelection(target *html.Node, key, text string, mods Modifiers) {
	text = sanitizeEntryValue(target, text)
	sel := d.selection(target)
	v := []rune(nodeAttr(target, "value"))
	newValue := string(v[:sel.start]) + text + string(v[sel.end:])
	setAttr(target, "value", newValue)
	caret := sel.start + utf8.RuneCountInString(text)
	d.setSelection(target, caret, caret, "none")
	d.dispatch(target, "input", key, mods)
}

// anchorFocus decomposes sel into its anchor (the fixed edge a Shift-extend
// keeps in place) and focus (the edge that moves) — the inverse of the
// (start, end, direction) triple setSelection normalizes into. A collapsed
// selection, or one with direction "none", has no distinguished anchor yet,
// so both are just the caret position (sel.start == sel.end already in that
// case) — exactly the state a fresh Shift+Arrow/Home/End should extend
// from.
func anchorFocus(sel selectionState) (anchor, focus int) {
	if sel.direction == "backward" {
		return sel.end, sel.start
	}
	return sel.start, sel.end
}

// directionOf reports the selectionDirection a raw (anchor, focus) pair
// normalizes to once setSelection sorts them into (start, end): "forward"
// if focus is at or past anchor, "backward" if focus is before it, "none"
// if they're equal (a Shift-extend that lands back where it started reads
// as no active direction, not a phantom forward selection — matching real
// spec's own equivalent).
func directionOf(anchor, focus int) string {
	switch {
	case focus > anchor:
		return "forward"
	case focus < anchor:
		return "backward"
	default:
		return "none"
	}
}

// moveCaretHorizontal implements ArrowLeft's (forward=false)/ArrowRight's
// (forward=true) default action for a focused text entry. With extend
// false (no Shift) and an existing selection, this collapses to whichever
// edge that direction points at — matching real browsers, where an
// unmodified arrow out of a selection lands at its near edge rather than
// moving relative to the caret — otherwise it moves a collapsed caret one
// rune, clamped by setSelection to [0, len(value)]. With extend true
// (Shift held), it moves the "focus" edge (see anchorFocus) by one rune,
// keeping the anchor fixed, growing or shrinking the selection.
func (d *Document) moveCaretHorizontal(target *html.Node, forward, extend bool) {
	sel := d.selection(target)
	if !extend {
		if sel.start != sel.end {
			pos := sel.start
			if forward {
				pos = sel.end
			}
			d.setSelection(target, pos, pos, "none")
			return
		}
		pos := sel.start
		if forward {
			pos++
		} else {
			pos--
		}
		d.setSelection(target, pos, pos, "none")
		return
	}
	anchor, focus := anchorFocus(sel)
	if forward {
		focus++
	} else {
		focus--
	}
	d.setSelection(target, anchor, focus, directionOf(anchor, focus))
}

// lineBounds returns the rune-offset [start, end) of the "\n"-delimited
// line containing position pos within value — moveCaretToLineEdge's
// line-relative movement for a multi-line <textarea>. A single-line value
// (no "\n" at all) has exactly one line spanning the whole string, so
// start=0 and end=utf8.RuneCountInString(value) regardless of pos, matching
// Home/End's plain "start/end of value" behavior for a single-line
// <input>. pos is clamped to [0, len(value in runes)].
func lineBounds(value string, pos int) (start, end int) {
	runes := []rune(value)
	pos = clampInt(pos, 0, len(runes))
	start = 0
	for i := 0; i < pos; i++ {
		if runes[i] == '\n' {
			start = i + 1
		}
	}
	end = len(runes)
	for i := pos; i < len(runes); i++ {
		if runes[i] == '\n' {
			end = i
			break
		}
	}
	return start, end
}

// moveCaretToLineEdge implements Home's (toEnd=false)/End's (toEnd=true)
// default action for a focused text entry: moves to the start/end of the
// "\n"-delimited line containing the caret's current "focus" edge (see
// anchorFocus, lineBounds) — for a single-line <input>, that's simply the
// start/end of the whole value, since it has exactly one line. extend
// mirrors moveCaretHorizontal's Shift handling.
func (d *Document) moveCaretToLineEdge(target *html.Node, toEnd, extend bool) {
	sel := d.selection(target)
	anchor, focus := anchorFocus(sel)
	lineStart, lineEnd := lineBounds(nodeAttr(target, "value"), focus)
	newFocus := lineStart
	if toEnd {
		newFocus = lineEnd
	}
	if !extend {
		d.setSelection(target, newFocus, newFocus, "none")
		return
	}
	d.setSelection(target, anchor, newFocus, directionOf(anchor, newFocus))
}

// DispatchPaste dispatches a "paste" event on the currently focused element,
// with Event.ClipboardData set to text — the host's job to supply (see
// tui.Loop's bracketed-paste handling for where a real terminal's pasted
// text comes from); Document has no clipboard access of its own. Unless a
// listener calls Event.PreventDefault, the default action replaces a
// focused editable text-like <input>/<textarea>'s current selection with
// ev.ClipboardData (a listener may have rewritten it, mirroring a real
// paste handler calling clipboardData.setData first) via replaceSelection —
// a readonly or disabled field takes no insertion (isEditable), matching a
// real browser, where a paste handler still fires but the field is
// unchanged —
// inserting at the caret if the selection is collapsed, which is exactly
// today's append-at-end behavior for any field that has never had
// SetSelectionRange called on it (the untouched default is a caret
// collapsed at the end of value) — and dispatches "input" on it afterward,
// mirroring a real paste's per-commit "input" event. A no-op default action
// for any other focused element (a listener still sees the event; there's
// just nothing built-in to paste into). Returns false if nothing is
// focused.
func (d *Document) DispatchPaste(text string) bool {
	if d.focused == nil {
		return false
	}
	target := d.focused
	ev := d.newEvent(target, "paste", "", Modifiers{})
	ev.ClipboardData = text
	d.runDispatch(ev, target, ev.Bubbles)
	if ev.DefaultPrevented() {
		return true
	}
	if isEditable(target) {
		d.replaceSelection(target, "", ev.ClipboardData, Modifiers{})
	}
	return true
}

// DispatchCut dispatches a "cut" event on the currently focused element,
// with Event.ClipboardData pre-populated from its current selection, if a
// focused text-like <input>/<textarea> has a non-collapsed one — real cut
// semantics, scoped to the selected range, now that one exists (see
// docs/proposals/CARET_SELECTION.md) — or, if the selection is collapsed,
// from the field's whole value, preserving this package's original
// (pre-selection) "cut acts on the entire field" behavior for the common
// case of a caller that never calls SetSelectionRange. A listener may read
// or rewrite ev.ClipboardData, the same way DispatchPaste's works in
// reverse. Unless a listener calls Event.PreventDefault, the default action
// removes exactly what was placed on the clipboard — the selected range via
// deleteSelectionRange, or the whole value in the collapsed case — and
// dispatches "input". A readonly or disabled field still populates
// ClipboardData (so a host can put it on the real clipboard) but loses
// nothing, matching a browser, where cutting from a non-editable field
// degrades to a copy. Returns the final ClipboardData (post-listener) and
// true if something was focused to cut from, so a host (e.g. tui.Loop) can
// hand that text to the real system clipboard — Document itself has no OS
// clipboard access. ok is false, with an empty string, if nothing is
// focused.
func (d *Document) DispatchCut() (text string, ok bool) {
	if d.focused == nil {
		return "", false
	}
	target := d.focused
	var clip string
	var sel selectionState
	if isTextEntry(target) {
		sel = d.selection(target)
		v := []rune(nodeAttr(target, "value"))
		if sel.start != sel.end {
			clip = string(v[sel.start:sel.end])
		} else {
			clip = string(v)
		}
	}
	ev := d.newEvent(target, "cut", "", Modifiers{})
	ev.ClipboardData = clip
	d.runDispatch(ev, target, ev.Bubbles)
	if !ev.DefaultPrevented() && isEditable(target) {
		// Re-read sel here rather than reusing the copy captured above: a
		// listener may have mutated target's value (or selection) during
		// dispatch (e.g. calling SetValue), and d.selection always
		// reclamps against the *current* value length — reusing the
		// pre-dispatch snapshot instead could slice past a since-shortened
		// value.
		sel = d.selection(target)
		if sel.start != sel.end {
			d.deleteSelectionRange(target, sel)
		} else {
			setAttr(target, "value", "")
			d.setSelection(target, 0, 0, "none")
		}
		d.dispatch(target, "input", "", Modifiers{})
	}
	return ev.ClipboardData, true
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
// for the additional scroll-container and tabindex cases this alone doesn't
// cover.
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

// tabIndexOf reports n's tabindex attribute, parsed as a base-10 integer.
// ok is false if the attribute is absent or its value doesn't parse as an
// integer — HTML treats an invalid tabindex the same as a missing one,
// rather than raising an error.
func tabIndexOf(n *html.Node) (int, bool) {
	for _, a := range n.Attr {
		if a.Key == "tabindex" {
			v, err := strconv.Atoi(strings.TrimSpace(a.Val))
			if err != nil {
				return 0, false
			}
			return v, true
		}
	}
	return 0, false
}

// isFocusable reports whether n is a tab-stoppable element: a form control
// (isFormFocusable), any element carrying a valid tabindex attribute
// (mirroring real DOM: tabindex makes an otherwise non-focusable element
// like <div>/<a> focusable, regardless of tag), or — mirroring real
// browsers' keyboard-accessible scroll containers — an overflow:scroll|auto
// element with a resolved height (a key in d.scrollOffsets as of the most
// recent Render call; see nearestScrollable) that has no focusable
// descendant of its own. The scroll-container case exists so a scrollable
// region with no button/input inside it is still Tab-reachable for
// keyboard-driven scrolling (DispatchKey's
// PageUp/PageDown/ArrowUp/ArrowDown/ArrowLeft/ArrowRight); a container that
// already has a focusable descendant is reached through that descendant
// instead, so it isn't also made its own redundant tab stop. Checks both
// scrollOffsets and scrollOffsetsX, so a horizontally-only scrollable
// element (overflow-x:scroll|auto with no vertical counterpart) is just as
// reachable as a vertically-scrollable one. Always false for the
// scroll-container case before the first Render (both maps are empty then,
// same staleness as Rect/ScrollVisible).
//
// Note this only decides focusability, not sequential (Tab-key) order — a
// negative tabindex is focusable here but excluded from focusableList's
// output; see inTabOrder.
func (d *Document) isFocusable(n *html.Node) bool {
	if isFormFocusable(n) {
		return true
	}
	if n.Type != html.ElementNode || nodeHasAttr(n, "disabled") {
		return false
	}
	if _, ok := tabIndexOf(n); ok {
		return true
	}
	_, scrollsY := d.scrollOffsets[n]
	_, scrollsX := d.scrollOffsetsX[n]
	if !scrollsY && !scrollsX {
		return false
	}
	return !d.hasFocusableDescendant(n)
}

// inTabOrder reports whether n, already known focusable (isFocusable),
// participates in sequential Tab navigation. Only a negative tabindex opts
// an otherwise-focusable element out — matching real DOM's tabindex="-1"
// semantics: reachable via Focus()/click, skipped by Tab.
func inTabOrder(n *html.Node) bool {
	v, ok := tabIndexOf(n)
	return !ok || v >= 0
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

// focusableList returns every element in sequential (Tab-key) focus order:
// elements with a positive tabindex first, ascending by value (ties broken
// by document order), followed by every other focusable element (no
// tabindex, tabindex="0", or a native/scroll-container tab stop) in document
// order — matching real DOM's tabindex-ordering algorithm. Elements that are
// focusable but excluded from Tab order (tabindex="-1", see inTabOrder) are
// omitted entirely.
func (d *Document) focusableList() []*html.Node {
	var out []*html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if d.isFocusable(n) && inTabOrder(n) {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(d.doc)

	sort.SliceStable(out, func(i, j int) bool {
		vi, oki := tabIndexOf(out[i])
		vj, okj := tabIndexOf(out[j])
		pi := oki && vi > 0
		pj := okj && vj > 0
		if pi != pj {
			return pi // positive-tabindex elements sort before everything else
		}
		if pi && pj && vi != vj {
			return vi < vj
		}
		return false // stable: preserves document order within each bucket/value
	})
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
		d.commitChange(prev, true)
		d.dispatch(prev, "blur", "", Modifiers{})
	}
	d.snapshotValueAtFocus(el.node)
	d.scrollIntoView(el.node)
	d.scrollIntoViewX(el.node)
	d.dispatch(el.node, "focus", "", Modifiers{})
	return true
}

// scrollIntoView adjusts every scrollable ancestor of n (see
// nearestScrollable, walked here via ancestorChain since more than one
// nested scrollable ancestor may need adjusting) so n's previous-frame Rect
// falls within that ancestor's own previous-frame visible range — mirroring
// a real browser auto-scrolling a newly focused control into view. One-frame
// stale by construction (n's/the ancestor's Rects are only as fresh as the
// last Render call, the same staleness Rect itself already documents), and a
// no-op before the first Render (d.positions is nil). Vertical-axis only —
// see scrollIntoViewX for the horizontal counterpart, called alongside this
// one from focus().
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

// scrollIntoViewX is scrollIntoView's horizontal-axis transpose (Row/Height/
// scrollOffsets/scrollViewport/TopOffset become Col/Width/scrollOffsetsX/
// scrollViewportX/LeftOffset) — see its doc comment for the shared rationale
// and staleness caveat.
func (d *Document) scrollIntoViewX(n *html.Node) {
	elRect, ok := d.positions[n]
	if !ok {
		return
	}
	for _, anc := range ancestorChain(n) {
		offset, isScrollable := d.scrollOffsetsX[anc]
		if !isScrollable {
			continue
		}
		ancRect, ok := d.positions[anc]
		if !ok {
			continue
		}
		vp := d.scrollViewportX[anc]
		contentLeft := ancRect.Col + vp.LeftOffset
		relLeft := elRect.Col - contentLeft
		relRight := relLeft + elRect.Width - 1
		switch {
		case relLeft < 0:
			offset += relLeft
		case relRight >= vp.Width:
			offset += relRight - vp.Width + 1
		default:
			continue
		}
		if offset < 0 {
			offset = 0
		}
		d.scrollOffsetsX[anc] = offset
	}
}

// scrollVisible is Element.ScrollVisible's implementation: reports whether
// el's Rect, as of the most recent Render call, currently falls at least
// partly within the visible content range of every scrollable ancestor it
// has. Rect itself is never clipped or hidden for a scrolled-off element
// (matching a real scrolled-off DOM element's getBoundingClientRect() — see
// Rect's doc comment), so a host placing its own UI (e.g. Loop's terminal
// cursor, via focusCursorPos) on top of an element's Rect needs this to know
// whether that position is actually visible right now, rather than
// off-screen inside a container that has since scrolled past it (e.g. via
// DispatchWheel/DispatchKey scrolling a pane out from under a focused
// control it contains). True for an element with no scrollable ancestor, or
// before the first Render. Checks both scroll axes: an element off-screen on
// either the vertical or horizontal range of any scrollable ancestor is not
// visible.
func (d *Document) scrollVisible(el *Element) bool {
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
		ancRect, ok := d.positions[anc]
		if !ok {
			continue
		}
		if vp, isScrollable := d.scrollViewport[anc]; isScrollable {
			contentTop := ancRect.Row + vp.TopOffset
			relTop := rect.Row - contentTop
			relBottom := relTop + rect.Height - 1
			if relBottom < 0 || relTop >= vp.Height {
				return false
			}
		}
		if vp, isScrollable := d.scrollViewportX[anc]; isScrollable {
			contentLeft := ancRect.Col + vp.LeftOffset
			relLeft := rect.Col - contentLeft
			relRight := relLeft + rect.Width - 1
			if relRight < 0 || relLeft >= vp.Width {
				return false
			}
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
	d.commitChange(prev, true)
	d.dispatch(prev, "blur", "", Modifiers{})
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

// setInnerHTML is Element.SetInnerHTML's implementation: parses htmlStr as
// an HTML fragment (parsed in el's own
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
// mutation for content that doesn't — see docs/INTERACTIVE.md's ImportHTML note,
// which this supersedes).
//
// A <style> element in the fragment (or removed along with el's previous
// children) does take effect: it marks Document's cachedRules stale (see
// markRulesStale), so the next Render recomputes the resolved stylesheet
// rule set rather than reusing a snapshot from before the replacement. That
// said, page-level CSS still belongs in Options.CSS/Stylesheets, set once at
// ParseDocument time — SetInnerHTML is meant for markup, not styling; a
// <style>-bearing fragment is supported, not recommended.
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
// comments on Document). The two per-node maps that *aren't* rebuilt —
// selections and valueAtFocus — are swept explicitly by pruneDetachedState,
// alongside the focus clearing above.
func (d *Document) setInnerHTML(el *Element, htmlStr string) error {
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
		markRulesStale(d, c)
		c = next
	}
	for _, n := range nodes {
		el.node.AppendChild(n)
		markRulesStale(d, n)
	}
	d.pruneDetachedState()
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
	d.pruneDetachedState()
}

// pruneDetachedState drops every piece of per-node state Document holds for
// nodes that are no longer reachable from the document root — the common
// cleanup every mutation that can cut a subtree out of the tree
// (SetInnerHTML, SetPreRendered, Element.RemoveChild, Element.ReplaceChild)
// needs.
//
// Focus is cleared silently (no "blur" dispatched — the element is gone, not
// blurred) rather than left dangling on a detached node. d.selections and
// d.valueAtFocus are swept the same way: unlike scrollOffsets/scrollViewport/
// contentOffsets, which Render rebuilds wholesale every frame and which
// therefore drop stale nodes for free, those two are keyed by node and only
// ever written on demand — so without this sweep, a long-running Loop that
// repeatedly refreshes a container via SetInnerHTML (the documented pattern
// for content that changes shape) would accumulate one entry per text entry
// it ever rendered, for the Document's whole lifetime. Both maps hold at
// most a handful of live entries, so the sweep is cheap even though it's a
// full scan.
//
// Event listeners are deliberately NOT swept: dropping a subtree without
// RemoveEventListener leaks its listeners here exactly as it does in a real
// DOM, and that's the documented behavior (see setInnerHTML).
func (d *Document) pruneDetachedState() {
	if d.focused != nil && !isDescendant(d.doc, d.focused) {
		d.focused = nil
	}
	for n := range d.selections {
		if !isDescendant(d.doc, n) {
			delete(d.selections, n)
		}
	}
	for n := range d.valueAtFocus {
		if n != d.focused && !isDescendant(d.doc, n) {
			delete(d.valueAtFocus, n)
		}
	}
}

// isDescendant reports whether n is root or a descendant of root, by walking
// up n's parent chain — used by pruneDetachedState to detect nodes that just
// got cut out of the tree.
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

// CreateComment returns a new comment node carrying text, detached from the
// tree — mirroring the DOM's Document.createComment. Attach it with
// Element.AppendChild or Element.InsertBefore. A comment node never
// produces visible output once attached — internal/render's dispatch
// explicitly skips html.CommentNode wherever it appears, the same way a
// browser never renders comment content either.
func (d *Document) CreateComment(text string) *Element {
	return &Element{node: &html.Node{Type: html.CommentNode, Data: text}, doc: d}
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
	walkMatchingSubtree(d.doc, sel, true, visit)
}

// walkMatchingSubtree is walkMatching's shared implementation, generalized
// to start from any root (the document node itself for Document's
// whole-document search, or an arbitrary element node for Element's
// subtree-scoped QuerySelector/QuerySelectorAll). includeSelf controls
// whether root itself is tested against sel — false for Element's scoped
// search, matching the DOM's own rule that querySelector(All) never matches
// the context node itself, only its descendants.
func walkMatchingSubtree(root *html.Node, sel string, includeSelf bool, visit func(n *html.Node) bool) {
	group := cssengine.ParseSelectorGroup(sel)
	var walk func(n *html.Node, testSelf bool) bool
	walk = func(n *html.Node, testSelf bool) bool {
		if testSelf && n.Type == html.ElementNode {
			if group.Match(n, focusAttr, "") {
				if !visit(n) {
					return false
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if !walk(c, true) {
				return false
			}
		}
		return true
	}
	walk(root, includeSelf)
}

// classNameSelector turns a whitespace-separated list of class names (the
// argument shape getElementsByClassName accepts per spec) into the
// equivalent compound class selector (".a.b"), or "" if cls has no tokens.
func classNameSelector(cls string) string {
	fields := strings.Fields(cls)
	if len(fields) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range fields {
		b.WriteByte('.')
		b.WriteString(f)
	}
	return b.String()
}

// GetElementsByClassName returns every element in the document whose class
// attribute includes every token in cls (cls may itself be a
// whitespace-separated list of multiple class names) — mirroring the DOM's
// Document.getElementsByClassName().
func (d *Document) GetElementsByClassName(cls string) []*Element {
	sel := classNameSelector(cls)
	if sel == "" {
		return nil
	}
	return d.QuerySelectorAll(sel)
}

// GetElementsByTagName returns every element in the document with the
// given tag name, in document order — mirroring the DOM's
// Document.getElementsByTagName().
func (d *Document) GetElementsByTagName(tag string) []*Element {
	return d.QuerySelectorAll(tag)
}

// GetElementsByName returns every element in the document whose name
// attribute equals name — mirroring the DOM's Document.getElementsByName().
func (d *Document) GetElementsByName(name string) []*Element {
	return d.QuerySelectorAll(fmt.Sprintf("[name=%q]", name))
}

// Body returns the document's <body> element, or nil if there is none —
// mirroring the DOM's Document.body.
func (d *Document) Body() *Element {
	return d.QuerySelector("body")
}

// Head returns the document's <head> element, or nil if there is none —
// mirroring the DOM's Document.head.
func (d *Document) Head() *Element {
	return d.QuerySelector("head")
}

// Title returns the text content of the document's <title> element, or ""
// if there is none — mirroring the DOM's Document.title (getter only; the
// DOM's title setter has no equivalent here — set the <title> element's
// text content directly instead).
func (d *Document) Title() string {
	el := d.QuerySelector("title")
	if el == nil {
		return ""
	}
	return el.TextContent()
}

// Forms returns every <form> element in the document, in document order —
// mirroring the DOM's Document.forms (an HTMLCollection there; a plain
// slice here, same as QuerySelectorAll).
func (d *Document) Forms() []*Element {
	return d.QuerySelectorAll("form")
}

// Images returns every <img> element in the document, in document order —
// mirroring the DOM's Document.images.
func (d *Document) Images() []*Element {
	return d.QuerySelectorAll("img")
}

// Links returns every <a>/<area> element carrying an href attribute, in
// document order — mirroring the DOM's Document.links.
func (d *Document) Links() []*Element {
	return d.QuerySelectorAll("a[href], area[href]")
}

// Scripts returns every <script> element in the document, in document
// order — mirroring the DOM's Document.scripts. Present for structural
// completeness only: <script> content is never executed or rendered (see
// COMPATIBILITY.md).
func (d *Document) Scripts() []*Element {
	return d.QuerySelectorAll("script")
}

// StyleSheets returns every <style> element in the document, in document
// order — mirroring the DOM's Document.styleSheets (a StyleSheetList of
// parsed CSSOM objects there; the raw <style> elements here, since this
// package has no CSSOM to expose).
func (d *Document) StyleSheets() []*Element {
	return d.QuerySelectorAll("style")
}
