package render

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/colorprofile"
	"github.com/client9/htmlterm/internal/textcell"
	"golang.org/x/net/html"
)

// nodeHasAttr reports whether key is present on n, regardless of its value —
// needed for boolean attributes like "checked", where nodeAttr's "" return
// can't distinguish absent from present-but-empty (e.g. checked="").
func nodeHasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

// inputDisplayText synthesizes an <input>'s visual content from its
// attributes rather than children (it has none) — see docs/INTERACTIVE.md's
// "render actual form controls" section. checkbox/radio show a glyph
// reflecting the checked attribute; submit/button/reset show a bracketed
// label (falling back to a type-appropriate default when value is unset);
// hidden renders nothing; every other type (the text-like default) shows
// its value, falling back to its placeholder, plain — no brackets. Unlike
// <button> (a real element with children, so the UA stylesheet's
// button::before/::after can bracket it like any other content), a
// text-like <input> gets no boundary glyph at all: it has no children for
// pseudo-elements to wrap, and — more importantly — a real browser doesn't
// bracket its text fields either, it just sizes them (see renderInput's
// width handling) and leaves any visible edge to the author's own
// background-color/border. Callers needing a fixed-width field pad this
// return value themselves (renderInput), since this function has no notion
// of the resolved box width.
func inputDisplayText(n *html.Node) string {
	typ := strings.ToLower(nodeAttr(n, "type"))
	if typ == "" {
		typ = "text"
	}
	switch typ {
	case "hidden":
		return ""
	case "checkbox":
		if nodeHasAttr(n, "checked") {
			return "☑"
		}
		return "☐"
	case "radio":
		if nodeHasAttr(n, "checked") {
			return "●"
		}
		return "○"
	case "submit", "button", "reset":
		label := nodeAttr(n, "value")
		if label == "" {
			switch typ {
			case "submit":
				label = "Submit"
			case "reset":
				label = "Reset"
			default:
				label = "Button"
			}
		}
		return "[ " + label + " ]"
	default:
		val := nodeAttr(n, "value")
		if val == "" {
			val = nodeAttr(n, "placeholder")
		}
		return val
	}
}

// inputSizeAttr reads a text-like <input>'s "size" attribute — the number of
// visible character cells real HTML sizes such a field to — defaulting to
// 20 (matching browsers' own default) when absent or not a valid positive
// integer. This is renderInput's naturalWidth for resolveWidthConstraints,
// the same width-resolution shared with <progress>/<meter>'s bar: an
// explicit CSS width/min-width/max-width on the input still overrides it.
func inputSizeAttr(n *html.Node) int {
	v := nodeAttr(n, "size")
	if v == "" {
		return 20
	}
	size, err := strconv.Atoi(v)
	if err != nil || size <= 0 {
		return 20
	}
	return size
}

// padInputText right-pads s with spaces to width, so an empty or short
// value still fills the field's resolved box — matching a real browser,
// where a text field's empty tail is part of its visible box rather than
// just wherever the value's own characters happen to end. A value at or
// past width is returned unchanged: it overflows uncut rather than being
// truncated or scrolled into view the way a real browser would keep the
// caret visible — an accepted approximation, see COMPATIBILITY.md.
//
// s is measured in terminal columns (textcell.Width), not runes: a value of
// CJK or emoji characters occupies two cells apiece, so padding by rune count
// would render the field wider than its resolved width and break the
// column-alignment invariant every other box in this package maintains.
func padInputText(s string, width int) string {
	n := textcell.Width(s)
	if n >= width {
		return s
	}
	return s + strings.Repeat(" ", width-n)
}

// selectionRange reports the [start, end) rune range into a focused text
// entry's value that should render under the ::selection highlight — see
// docs/proposals/CARET_SELECTION.md. has is false unless n currently
// carries the focus marker (real UAs don't paint a selection highlight on
// an unfocused field) and both selectionStartAttr/selectionEndAttr are
// present. The two attribute values are reclamped against n's *current*
// value length here, the same way Document.selection reclamps on every
// read — they normally can't be stale (Document.setSelection/clearSelection
// keep them in sync with d.selections), but a direct
// Element.SetAttribute("value", ...) call bypasses SetValue's
// clearSelection and can leave them pointing past a since-shortened value;
// reclamping here keeps this function's notion of the selection consistent
// with Element.SelectionStart()/SelectionEnd() (which reclamp the same way)
// rather than trusting the raw attributes verbatim. has is false if,
// after reclamping, the range is malformed or collapsed (start >= end) —
// including the collapsed-by-reclamping case, not just an originally
// collapsed one.
func (r *Engine) selectionRange(n *html.Node) (start, end int, has bool) {
	if !nodeHasAttr(n, r.focusAttr) {
		return 0, 0, false
	}
	sv := nodeAttr(n, r.selectionStartAttr)
	ev := nodeAttr(n, r.selectionEndAttr)
	if sv == "" || ev == "" {
		return 0, 0, false
	}
	s, errS := strconv.Atoi(sv)
	e, errE := strconv.Atoi(ev)
	if errS != nil || errE != nil {
		return 0, 0, false
	}
	valueLen := utf8.RuneCountInString(nodeAttr(n, "value"))
	s = min(max(s, 0), valueLen)
	e = min(max(e, 0), valueLen)
	if s >= e {
		return 0, 0, false
	}
	return s, e, true
}

// resolveSelectionStyle resolves n's effective ::selection style: any
// author color/background-color declaration wins, the same
// resolve-then-render shape resolveScrollbarStyle uses for
// ::scrollbar-track/::scrollbar-thumb. With neither set, it falls back to
// reverse video (inlineStyle.reverse) — the UA default every terminal
// editor already uses for a text selection, since there's no
// terminal-neutral equivalent of a browser's platform selection color to
// hardcode instead. elemDecls is n's own resolved declarations, read only
// for var() substitution (see pseudoElemDecls).
func (r *Engine) resolveSelectionStyle(n *html.Node, elemDecls map[string]string) inlineStyle {
	decls := r.pseudoElemDecls(n, "selection", customPropSubset(elemDecls))
	style := extractInlineStyle(decls)
	if style.fg == nil && style.bg == nil {
		style.reverse = true
	}
	return style
}

// synthesizedControlText renders the elements whose visual content is
// synthesized from their attributes rather than laid out from child nodes —
// <input> (which has no children at all), <select>'s closed state (built from
// its <option> labels, not from rendering them), and <progress>/<meter>'s bar.
// It reports false for every other element, leaving the caller to render n's
// children the ordinary way.
//
// This is the one copy of that dispatch. It's reached from all four layout
// paths a control can turn up in — root-level (render.go), nested inline
// (inline.go), block-level, and as a flex item (both via block.go's content
// step) — and the fourth is why it was extracted: a flex item is blockified,
// so it went through renderBlockContentBox, which walked the element's
// children and found none. Every <input> in a `display: flex` row rendered as
// nothing at all.
//
// <progress>/<meter> are deliberately not wrapped in acc.render, unlike
// input/select: each glyph run is already individually styled from its own
// ::progress-*/::meter-* declarations, and one uniform outer style would
// flatten that (e.g. <meter>'s region coloring).
func (r *Engine) synthesizedControlText(n *html.Node, decls map[string]string, acc inlineStyle, availWidth int) (string, bool) {
	switch n.Data {
	case "input":
		// renderInput applies the element's own resolved style (e.g. a :focus
		// background-color) to the synthesized text itself, and splices in the
		// ::selection highlight over any focused non-collapsed selection (see
		// docs/proposals/CARET_SELECTION.md).
		return r.renderInput(n, decls, acc, availWidth, r.profile), true
	case "select":
		return acc.render(selectDisplayText(n), r.profile), true
	case "progress", "meter":
		innerWidth, _ := resolveWidthConstraints(decls, availWidth, formControlBarDefaultWidth)
		if innerWidth < 0 {
			innerWidth = 0
		}
		if n.Data == "progress" {
			bar, value := r.resolveProgressStyle(n, decls)
			return progressDisplayText(n, innerWidth, bar, value, r.profile), true
		}
		bar, optimum, suboptimum, evenLessGood := r.resolveMeterStyle(n, decls)
		return meterDisplayText(n, innerWidth, bar, optimum, suboptimum, evenLessGood, r.profile), true
	}
	return "", false
}

// renderInput renders an <input>'s synthesized content, applying acc
// uniformly — except, for a text-like input with a non-collapsed, focused
// selection (selectionRange), the [start, end) rune range within its value,
// which instead renders under resolveSelectionStyle's ::selection style.
// Checkbox/radio/submit/button/reset/hidden inputs have no notion of a text
// selection and no resolved width of their own (they size to their glyph/
// label like <button>), so they keep rendering as a single
// acc.render(inputDisplayText(n), p) call, same as before this existed. A
// text-like input's own width instead comes from resolveWidthConstraints
// (author CSS width/min-width/max-width, falling back to inputSizeAttr's
// "size"-attribute-or-20 default) — the same mechanism <progress>/<meter>
// use for their bar — and its display text is padded (padInputText) to
// fill that width, since without the old "["/"]" brackets there's nothing
// else marking the field's resolved size.
func (r *Engine) renderInput(n *html.Node, elemDecls map[string]string, acc inlineStyle, availWidth int, p colorprofile.Profile) string {
	typ := strings.ToLower(nodeAttr(n, "type"))
	if typ == "" {
		typ = "text"
	}
	switch typ {
	case "hidden", "checkbox", "radio", "submit", "button", "reset":
		return acc.render(inputDisplayText(n), p)
	}
	innerWidth, _ := resolveWidthConstraints(elemDecls, availWidth, inputSizeAttr(n))
	if innerWidth < 0 {
		innerWidth = 0
	}
	start, end, has := r.selectionRange(n)
	if !has {
		return acc.render(padInputText(inputDisplayText(n), innerWidth), p)
	}
	val := nodeAttr(n, "value")
	if val == "" {
		val = nodeAttr(n, "placeholder")
	}
	runes := []rune(val)
	start = min(max(start, 0), len(runes))
	end = min(max(end, 0), len(runes))
	selStyle := r.resolveSelectionStyle(n, elemDecls)
	// Same column (not rune) measure padInputText uses on the unselected
	// path above — the two must agree, or a field changes width the moment
	// a selection appears in it.
	pad := max(innerWidth-textcell.Width(string(runes)), 0)
	return acc.render(string(runes[:start]), p) +
		selStyle.render(string(runes[start:end]), p) +
		acc.render(string(runes[end:])+strings.Repeat(" ", pad), p)
}

// parseFloatAttr reads n's key attribute as a float, falling back to def
// when the attribute is absent or unparseable — the same "bad numeric
// attribute silently falls back" behavior real HTML's IDL attribute
// reflection uses for <progress>/<meter>'s value/max/min/low/high/optimum,
// none of which have a distinct "invalid" rendering state of their own (see
// docs/proposals/PROGRESS_METER.md).
func parseFloatAttr(n *html.Node, key string, def float64) float64 {
	v, err := strconv.ParseFloat(nodeAttr(n, key), 64)
	if err != nil {
		return def
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// progressFraction reads a <progress>'s value/max attributes and returns
// its completion fraction in [0,1]. indeterminate is true iff the value
// attribute is absent — per spec, that is the *only* indeterminate trigger;
// an explicitly present but unparseable value is just clamped like any
// other bad numeric attribute (parseFloatAttr's def=0), not a second
// indeterminate case. max defaults to 1; an unparseable or non-positive max
// also falls back to 1, matching real UAs' "candidate maximum value" step.
func progressFraction(n *html.Node) (fraction float64, indeterminate bool) {
	if !nodeHasAttr(n, "value") {
		return 0, true
	}
	value := parseFloatAttr(n, "value", 0)
	max := parseFloatAttr(n, "max", 1)
	if max <= 0 {
		max = 1
	}
	return clampFloat(value, 0, max) / max, false
}

// meterFraction reads a <meter>'s value/min/max/low/high/optimum attributes
// (defaults 0/0/1/min/max/midpoint respectively) and returns its fraction
// in [0,1] plus which of the three WebKit-precedented regions the value
// falls in ("optimum"/"suboptimum"/"even-less-good"), resolved via the
// boundary-clamping order real UAs use: max is forced up to min if given as
// less than min, value/low/high are each clamped into the resulting
// [min,max] (high additionally floored at low), then optimum is clamped
// into [min,max] before the 3-case region algorithm runs (see
// docs/proposals/PROGRESS_METER.md for why this is 3 distinct cases, not one
// formula). A zero-width range (max==min after clamping) reports fraction
// 1.0 if value is at or above min, else 0.0 — real UAs converge on some sane
// behavior for this degenerate case rather than dividing by zero.
func meterFraction(n *html.Node) (fraction float64, region string) {
	min := parseFloatAttr(n, "min", 0)
	max := parseFloatAttr(n, "max", 1)
	if max < min {
		max = min
	}
	value := clampFloat(parseFloatAttr(n, "value", 0), min, max)
	low := clampFloat(parseFloatAttr(n, "low", min), min, max)
	high := clampFloat(parseFloatAttr(n, "high", max), low, max)
	optimum := clampFloat(parseFloatAttr(n, "optimum", (min+max)/2), min, max)

	region = meterRegion(optimum, low, high, value)
	if max == min {
		if value >= min {
			return 1.0, region
		}
		return 0.0, region
	}
	return (value - min) / (max - min), region
}

// meterRegion implements <meter>'s 3-case region algorithm: which of
// optimum/suboptimum/even-less-good value falls in, given optimum's
// position relative to [low,high].
func meterRegion(optimum, low, high, value float64) string {
	switch {
	case optimum >= low && optimum <= high:
		// optimum inside [low,high]: that range alone is optimum, and both
		// outer ranges are suboptimum — no even-less-good region exists.
		if value >= low && value <= high {
			return "optimum"
		}
		return "suboptimum"
	case optimum < low:
		// lower is better: [min,low] optimum, [low,high] suboptimum,
		// (high,max] even-less-good.
		switch {
		case value <= low:
			return "optimum"
		case value <= high:
			return "suboptimum"
		default:
			return "even-less-good"
		}
	default: // optimum > high
		// higher is better: [high,max] optimum, [low,high] suboptimum,
		// [min,low) even-less-good.
		switch {
		case value >= high:
			return "optimum"
		case value >= low:
			return "suboptimum"
		default:
			return "even-less-good"
		}
	}
}

// progressDisplayText synthesizes a <progress>'s visual content: innerWidth
// columns of bar.char, with the leading fraction*innerWidth columns (rounded
// to the nearest whole column, matching this codebase's existing
// fractional-to-integer convention — see estimateColumnWidths) replaced by
// value.char, each portion styled independently via its own resolved
// ::progress-bar/::progress-value declarations. An indeterminate <progress>
// (no value attribute) has no fraction to size a fill from, so real UAs'
// animated barber-pole becomes a static full-width bar.char run here — the
// one deliberate static-not-animated deviation from spec this feature has
// (no animation support exists at all).
func progressDisplayText(n *html.Node, innerWidth int, bar, value scrollbarStyle, profile colorprofile.Profile) string {
	fraction, indeterminate := progressFraction(n)
	if indeterminate {
		return bar.style.render(strings.Repeat(bar.char, innerWidth), profile)
	}
	filled := max(0, min(innerWidth, int(math.Round(fraction*float64(innerWidth)))))
	return value.style.render(strings.Repeat(value.char, filled), profile) +
		bar.style.render(strings.Repeat(bar.char, innerWidth-filled), profile)
}

// meterDisplayText is progressDisplayText's <meter> counterpart: the same
// filled/unfilled bar, but the filled portion's glyph/style is chosen from
// optimum/suboptimum/evenLessGood according to meterFraction's resolved
// region rather than always being one fixed "value" style.
func meterDisplayText(n *html.Node, innerWidth int, bar, optimum, suboptimum, evenLessGood scrollbarStyle, profile colorprofile.Profile) string {
	fraction, region := meterFraction(n)
	value := optimum
	switch region {
	case "suboptimum":
		value = suboptimum
	case "even-less-good":
		value = evenLessGood
	}
	filled := max(0, min(innerWidth, int(math.Round(fraction*float64(innerWidth)))))
	return value.style.render(strings.Repeat(value.char, filled), profile) +
		bar.style.render(strings.Repeat(bar.char, innerWidth-filled), profile)
}

// selectOptionNodes returns n's <option> descendants usable for layout and
// navigation, in document order: direct <option> children, plus (one level
// deep) the <option> children of any direct <optgroup> child — real HTML
// only ever nests <option> one level inside <optgroup>, never deeper. See
// docs/SELECT.md.
func selectOptionNodes(n *html.Node) []*html.Node {
	var out []*html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
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

// selectOptionLabel returns opt's visible label: its label attribute if set,
// else its concatenated descendant text content.
func selectOptionLabel(opt *html.Node) string {
	if label := nodeAttr(opt, "label"); label != "" {
		return label
	}
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(c *html.Node) {
		if c.Type == html.TextNode {
			sb.WriteString(c.Data)
		}
		for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
			walk(cc)
		}
	}
	walk(opt)
	return strings.TrimSpace(sb.String())
}

// selectedOption returns n's currently selected <option> — the first one
// carrying a selected attribute, else the first option (matching HTML's
// default-selection rule) — or nil if n has no options.
func selectedOption(n *html.Node) *html.Node {
	options := selectOptionNodes(n)
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

// selectDisplayText synthesizes a closed <select>'s visual content — the
// selected option's label, bracketed with a disclosure indicator. Mirrors
// inputDisplayText's role for <input>. The open dropdown itself is a
// separate compositing step — see compositeOpenSelects in select_popup.go.
func selectDisplayText(n *html.Node) string {
	label := ""
	if sel := selectedOption(n); sel != nil {
		label = selectOptionLabel(sel)
	}
	return "[ " + label + " ▾]"
}
