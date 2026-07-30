package render

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/colorprofile"
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
// its value, falling back to its placeholder.
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
		return "[" + val + "]"
	}
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

// renderInput renders an <input>'s synthesized content, applying acc
// uniformly — except, for a text-like input with a non-collapsed, focused
// selection (selectionRange), the [start, end) rune range within its value
// (not counting the synthesized "["/"]" brackets), which instead renders
// under resolveSelectionStyle's ::selection style. Checkbox/radio/submit/
// button/reset/hidden inputs have no notion of a text selection, so they
// keep rendering as a single acc.render(inputDisplayText(n), p) call, same
// as before this existed.
func (r *Engine) renderInput(n *html.Node, elemDecls map[string]string, acc inlineStyle, p colorprofile.Profile) string {
	typ := strings.ToLower(nodeAttr(n, "type"))
	if typ == "" {
		typ = "text"
	}
	switch typ {
	case "hidden", "checkbox", "radio", "submit", "button", "reset":
		return acc.render(inputDisplayText(n), p)
	}
	start, end, has := r.selectionRange(n)
	if !has {
		return acc.render(inputDisplayText(n), p)
	}
	val := nodeAttr(n, "value")
	if val == "" {
		val = nodeAttr(n, "placeholder")
	}
	runes := []rune(val)
	start = min(max(start, 0), len(runes))
	end = min(max(end, 0), len(runes))
	selStyle := r.resolveSelectionStyle(n, elemDecls)
	return acc.render("["+string(runes[:start]), p) +
		selStyle.render(string(runes[start:end]), p) +
		acc.render(string(runes[end:])+"]", p)
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
