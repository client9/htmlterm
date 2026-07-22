# Scrolling and Multi-Pane Layout: Design Notes

## Motivation

`htmlterm` has grown from a one-shot renderer into a small interactive TUI
framework (`Document`/`Element`, native Go DOM-style events, `Loop`,
timers, resize handling — see `INTERACTIVE.md`, `RENDERING.md`,
`REPAINT.md`). The natural next step is general scrollable application
layouts: any number of panes, arranged however (nested block elements with
explicit widths/heights, or a `display: flex` row/column — see CSS.md's
"Flexbox" section), where any pane's content may exceed its box and need
to scroll independently. A dashboard with a fixed header, a scrollable
body, and a fixed footer is one instance of that shape — an illustrative
example used throughout this doc, not the scope boundary. The design here
must not assume a specific pane count or arrangement.

Two CSS mechanisms are relevant: `overflow: scroll`/`auto` (making a
pane's content scrollable) and `display: flex` (arranging panes
declaratively). **Both are real, wanted features**, and both are now
implemented — this doc covers scrolling (Sections 1/3); flexbox's own
design and supported subset live in CSS.md's "Flexbox" section instead,
not here.

## Status

Section 1 (scrolling), Section 1's scrollbar gutter/indicator, and Section 3
(focus scroll-into-view) are all **implemented** — see
`Document.ScrollTop`/`SetScrollTop`/`DispatchWheel`, `DispatchKey`'s
`PageUp`/`PageDown`/`ArrowUp`/`ArrowDown` cases, and `Focus`'s
scroll-into-view call (all in `document.go`), plus the `overflow-x`/
`overflow-y` gates in `block.go`. Wheel-event decoding, originally
hand-rolled in `input.go` (since replaced — see INTERACTIVE.md's terminal
I/O section), now comes from `tcell.EventMouse`'s `WheelUp`/`WheelDown`
buttons (`tcell_loop.go`); `Document.DispatchWheel`'s own signature and
behavior are unchanged either way. All of the "explicit non-goals for v1"
listed under Section 1
below remain out of scope. Flexbox (`display: flex`/`inline-flex`) has
since landed too, as a deliberately small subset aimed at simple
single-row/single-column layouts — `internal/render/flex.go`; see CSS.md's
"Flexbox" section for exactly what's supported and what isn't yet
(`flex-wrap`, `align-content`, `align-self`, `order`, `row-reverse`/
`column-reverse`, and applied `flex-shrink`).

The **scrollbar gutter/indicator**, designed in detail below (see "Scrollbar
gutter and indicator"), shipped as designed: the `ScrollbarGutterWidth`
default constant and the gutter-reservation gate in `renderBlockContentBox`,
and the track/thumb rendering plus thumb-math formula in
`appendScrollbarColumn` (all in `block.go`) — see also CSS.md's `overflow`
section for the shipped `overflow-y: scroll` vs. `auto` behavior split. The
gutter's width and the track/thumb glyphs/colors, originally listed below as
an explicit non-goal ("CSS-configurable glyphs/colors for the track/thumb
characters"), have since shipped too, as `::scrollbar`/`::scrollbar-track`/
`::scrollbar-thumb` plus a `scrollbar-style: block|shaded|classic|ascii` shorthand
that presets those three pseudo-elements' glyphs/colors in one declaration
— see "Scrollbar pseudo-elements" below for that design and CSS.md's own
"Scrollbar pseudo-elements" section for the user-facing reference. Clickable
arrow-cap buttons at the two ends of the track — `::scrollbar-cap-start`/
`::scrollbar-cap-end`, on by default and included in every `scrollbar-style`
preset — have since shipped too; see "Scrollbar cap buttons" below. The rest
of the "explicit non-goals for the scrollbar" listed
below remain out of scope (`tabindex`/`autofocus` handling, named separately
under Section 3, likewise remain unimplemented).

## Why scrolling was sequenced before flexbox

This was the rationale at the time scrolling was designed and built, before
flexbox existed; kept here as history, not as a description of flexbox's
actual final design (see CSS.md's "Flexbox" section and
`internal/render/flex.go` for what was actually shipped — notably, it did
not generalize `sizeColumns`; item 3 below anticipated an approach the
implementation didn't end up taking).

1. **Scrolling is needed regardless of pane arrangement.** Any element
   whose content exceeds its box needs `overflow:auto`, whether that box's
   height came from an explicit CSS `height`, a percentage, or a flex
   resolution. It is not specific to flex layouts and is fully
   applicable to today's block-only layout model.
2. **Flexbox's output is designed to feed the mechanism scrolling
   introduces.** A flex-track algorithm's actual result is a resolved
   height (or width) per child — which only becomes useful once there's a
   height/overflow mechanism downstream that can clip/scroll content to
   that resolved size. Building scrolling first gave flexbox a concrete,
   already-working consumer to target instead of a hypothetical one.
3. **Risk/scope.** Scrolling extends existing seams incrementally:
   `block.go`'s already-existing overflow gate, `document.go`'s
   already-existing position-map pattern, `input.go`'s already-existing
   mouse decoder, `event.go`'s already-existing dispatch/`ancestorChain`.
   Flexbox was anticipated to require generalizing `table.go`'s
   column-only, string-based `sizeColumns` algorithm into a two-axis,
   box/token-based layout engine — a materially larger, riskier design
   surface with no existing two-axis precedent in the codebase at the
   time.
4. **General layouts aren't blocked in the meantime.** Nested block
   elements with explicit widths/heights already support arbitrary
   multi-pane arrangements manually; flexbox makes sizing declarative
   later, it doesn't unlock pane arrangement itself.

## What already exists (confirmed by reading the code, not assumed)

- `overflow` today (`block.go:358-370` for width-based line truncation,
  `block.go:397-421` for height truncation; `box.go:53-61`'s `forceHeight`
  for the root-level `Options.Height` analog) is a **static, stateless
  line-truncation gate** — `hidden`/`clip` slice `lines[:heightLines]` from
  index 0. There is no scroll-offset state anywhere, and `scroll`/`auto`
  aren't even distinguished from `visible` — only `hidden`/`clip` are
  checked, so any other value (including `scroll`/`auto`) falls through as
  if `visible`. No scrollbar concept exists. This gate applies uniformly to
  any block-level element regardless of how its height was set, so it's
  the correct foundation for scrolling any pane in any layout.
- Position tracking is incremental and first-class: `wordWrapTokens`
  (`wraptoken.go:354` call site in `block.go`) resolves line-wrapping and
  returns a `map[*html.Node]Rect` in the same pass; `mergePositions`
  (`wraptoken.go:52`) shifts and merges a child's position map into its
  parent's as composition proceeds bottom-up. `Document.Rect(el)`
  (`document.go`) is a lookup into the map `Document.Render()`
  (`document.go:47-55`) refreshes on every call.
- The closest existing multi-track sizing algorithm is `table.go`'s
  `sizeColumns` (`table.go:214-305`) — fixed/percent/flexible sizing via
  `effectiveMinMax`, iterative capped-expand distributing leftover space,
  greedy shrink-widest — but single-axis, columns-only, and living in
  table's string-based cell rendering (`table_render.go`), not the
  box/token model. Flexbox's own weighted-`flex-grow` distribution
  (`internal/render/flex.go`) ended up as fresh box/token-model code rather
  than a generalization of this algorithm.
- `spliceColumns` (`textutil.go:340`) — the line-compositing primitive
  `RENDERING.md`'s "Popups/z-order" section designed for overlaying one
  box's lines over another's at a known row/col range — is implemented and
  unit-tested (`TestSpliceColumns`) but **unwired**. It was considered as the
  mechanism for a scrollbar thumb but rejected for that purpose — see
  "Scrollbar gutter and indicator" under Section 1 for why (it overwrites
  whatever was already on that line, which risks silently clobbering real
  content on any line using the box's full width). It remains available for
  a genuine floating-overlay use case (a real popup/dropdown, per
  `RENDERING.md`), just not this one.
- `expandShorthand` (`css.go:184`) already expands `margin`/`padding`/
  `background`/`list-style` shorthands into their longhand properties at
  parse time (for both stylesheet rules and inline `style=` — see
  `parseCSS`/`parseInlineDecls`), with the normal per-property cascade
  (`resolveDecls`) then letting a more-specific/later longhand override just
  that one expanded value. This is the existing, proven mechanism the
  scrollbar design below reuses for `overflow` → `overflow-x`/`overflow-y`,
  rather than inventing a new runtime fallback.
- The focus system (`document.go:315-427` for `isFocusable`/
  `focusableList`/`Focus`/`Blur`/`FocusNext`/`FocusPrev`; cursor placement
  in `tcell_loop.go`'s `focusCursorPos`, since migrated off the original
  `loop.go`) is solid and mostly complete — the
  `data-htmlterm-focus` marker attribute drives `:focus`
  (`selector.go`'s `matchPseudo`), and `focusCursorPos` places the real
  terminal cursor on the focused element after each repaint. Its one real
  gap relative to browser behavior is **scroll-into-view**: nothing
  currently moves a scroll offset when focus lands on an element outside
  the visible range of a scrolled ancestor — this matters for any
  focusable control inside any scrollable pane, not just in a
  header/footer layout.
- Event dispatch (`event.go`) already has everything a new scroll-related
  input needs to plug into: `Document.dispatch(target, typ, key)` runs
  capture→target→bubble honoring `PreventDefault`; `ancestorChain(n)`
  (`event.go:120-129`) walks root→target, exactly what routing wheel input
  to the nearest scrollable ancestor needs. `elementAt` (`document.go`,
  used by `DispatchClick`) hit-tests the position map, deepest-node-wins on
  overlap. No changes to `dispatch` itself are needed for scrolling.

## Section 1 — `overflow: scroll`/`auto` and real scrolling (this pass)

### State

Add `scrollOffsets map[*html.Node]int` to `Document` (`document.go:22-30`),
keyed by `*html.Node` the same way `positions`/`listeners` already are —
not by `Element`, a throwaway handle reconstructed on every lookup.
Applies to any element with `overflow:scroll|auto` and a resolved height,
however that height was set (an explicit `height` today — flexbox landed
without ever resolving an explicit main-axis height itself, so this
remains the only source; see CSS.md's "Flexbox" section — no coupling to
how the box was sized). Scope
v1 to **vertical offset only** — the motivating cases, and the existing
clip code, are vertical; horizontal overflow is already handled by
wrapping in the common case.

### Rendering

Extend the existing gate at `block.go:397-421`. Today:

```go
if len(lines) > heightLines && (ov == "hidden" || ov == "clip") {
    lines = lines[:heightLines]
}
```

Add a branch for `ov == "scroll" || ov == "auto"` that instead does
`lines = lines[offset : offset+heightLines]`, with `offset` clamped to
`[0, max(0, len(lines)-heightLines)]` before slicing.

This surfaces two plumbing gaps that don't exist today:

- `Document.Render()` (`document.go:47-55`) constructs a throwaway
  `Renderer` via `New(d.opts)` with no back-reference to `Document`, so
  the renderer needs a way to read the current `scrollOffsets` map for
  this render pass (e.g. a field set on the `Renderer` right after
  construction, mirroring how `d.opts` is already passed through).
- Because clamping can change the effective offset (content shrank on
  resize or mutation), the *clamped* value must be written back into
  `Document.scrollOffsets` after `Render()` returns — the same
  "renderer computes, `Document` is the authoritative source of truth for
  the next call" pattern `d.positions` already follows at
  `document.go:53`.

### Position tracking under scroll

A scrolled container's clip must also shift its descendants' `Rect.Row` by
`-offset`, mirroring how `mergePositions` (`wraptoken.go:52`) already
shifts a child's position map by a placement offset during composition.

Out-of-viewport `Rect`s should be **kept, not deleted**, with an
out-of-range `Row` — matching real `getBoundingClientRect()` behavior for
a scrolled-off element in a browser. `elementAt` needs no change: a real
terminal click coordinate can never land on a `Row` outside the
container's own displayed range, so out-of-view entries are naturally
inert for hit-testing without special-casing.

This must work for arbitrarily nested scrollable regions (a scrollable
pane inside another scrollable pane) with no special-case code per nesting
depth, since the whole document re-renders every frame regardless — an
outer container's scroll offset naturally reshuffles an inner scrollable
child's rows for free, the same way any other layout change already does.

### Input plumbing

- Originally: extend `decodeSGRMouse` (`input.go`) to keep wheel reports
  (SGR mouse `Cb` bits 64/65) instead of discarding them, returning a new
  `wheelEvent` kind carrying a `delta`. `input.go` was later replaced by
  `tcell.Screen`'s own decoding (see INTERACTIVE.md) — wheel events now
  arrive as `tcell.EventMouse` with `WheelUp`/`WheelDown` set, translated
  to a `delta` of `∓1` in `tcell_loop.go`. Either way, the shape below is
  unchanged.
- Add `Document.DispatchWheel(row, col, delta int) bool`: hit-test via
  `elementAt`, then walk `ancestorChain(target)` from the hit-tested
  element to the nearest ancestor with a `scrollOffsets` entry (deepest
  wins, matching the tie-break `elementAt` already uses), adjust and clamp
  that offset. Routes correctly regardless of how many panes exist or how
  they're nested.
- Add `PageUp`/`PageDown`/arrow-at-boundary as new default actions inside
  `DispatchKey` (`document.go:256`), gated on `PreventDefault` the same way
  `Tab`/`Backspace` already are, reusing the same ancestor-walk helper —
  but resolved from the *focused* element's nearest scrollable ancestor,
  since keyboard scrolling (unlike wheel) has no click coordinate to
  hit-test from.

### Scrollbar gutter and indicator

Status: designed here in detail, not yet implemented (see "Status" above).

**Per-axis property, via the existing shorthand-expansion mechanism, not a
new fallback.** Add an `"overflow"` case to `expandShorthand` (`css.go:184`)
that expands the shorthand into `overflow-x`/`overflow-y` longhands — one
token sets both axes to the same value, two tokens set `overflow-x` then
`overflow-y` respectively, matching real CSS's own `overflow` shorthand
grammar. This reuses the exact mechanism `margin`/`padding` already go
through (expand at parse time, let `resolveDecls`' normal per-property
cascade let a more-specific/later longhand override just that one axis) —
no new runtime "check the longhand, else fall back to the shorthand" logic
needed anywhere. A stylesheet or inline `style=` that only ever sets plain
`overflow` (every existing call site, today) is completely unaffected:
`overflow-x` and `overflow-y` both end up equal to it, same as before this
change existed.

`block.go`'s two existing `decls["overflow"]` reads split along the axis
they actually gate: the width-truncation check (currently ~`block.go:358-359`,
gating `hidden`/`clip` truncation under `hasExplicitWidth`) reads
`overflow-x`; the height gate and the scroll-viewport recording (currently
~`block.go:400-440` and `block.go:554`) read `overflow-y`. Both were already
conceptually per-axis; they just shared one property name because nothing
before this needed to tell them apart.

**Convention — the per-value behavior `overflow-y` selects:**

| `overflow-y` value | Behavior |
|---|---|
| `hidden` / `clip` | Static truncation only (today's existing behavior) — no scroll offset state, no gutter. |
| `auto` | Real scrolling (offset state, wheel/keys/focus-into-view) — **no gutter, no indicator**. Exactly what Section 1 already ships, unchanged. |
| `scroll` | Real scrolling **plus** an always-reserved gutter column and a drawn track/thumb indicator, *regardless* of whether the content actually overflows. |
| `visible` / unset | No clipping, as today. |

Both `scroll` and `auto` still require a resolved height to do anything
(same precondition scrolling itself already has — "Applies to any element
with `overflow:scroll|auto` and a resolved height" in Section 1's "State"
above); `overflow-y: scroll` with no `height` is a no-op, same as `auto`
already is today.

**Why `auto` gets no indicator, deliberately, not as a shortcut.** Whether a
gutter column is needed depends on whether the content overflows vertically
— but that depends on how the content wraps, which depends on the width,
which is exactly what reserving a gutter column would change. Real CSS
engines hit this same circularity (it's why `scrollbar-gutter: stable`
exists as an escape hatch — auto-reserved scrollbar space is a known source
of layout jank even in real browsers). `overflow-y: scroll`'s CSS semantics
are already unconditional (a scrollbar shows whether or not it's needed),
so implementing exactly that case sidesteps the circularity entirely — one
wrap pass, at a width that already accounts for the gutter, exactly like
`auto` does today. Properly solving "reserve a gutter only when content
turns out to overflow" for `auto` would need a second wrap pass (this
codebase already has a measure-then-layout precedent for a *different*
purpose — `tokensNaturalWidth` wraps once at a sentinel width purely to
measure — but reusing that shape here isn't attempted in this pass) and is
called out below as a named non-goal, not fixed by this design.

**Mechanism: reserve a column, then append to it — never overlay/splice.**
`heightLines` is already resolved (`block.go:268`) well before `innerW` is
computed (`block.go:289`), so — unlike the "does this need scrolling at
all" question — "does this axis want a gutter" is known up front, with no
wrap pass needed first: `wantsGutter := heightLines > 0 && ovY == "scroll"`.
When true, subtract a small fixed `scrollbarGutterWidth` (1 column) from the
width passed into `clampCellPadding` at `block.go:289`, the same place
padding/border-character width is already subtracted — the gutter is just
one more thing that narrows the content area before wrapping, not a
retrofit after. If there isn't at least 1 column left for real content once
the gutter is subtracted, the gutter is silently dropped for that render
(content wins) rather than collapsing content to 0 width — a small, named
edge case, not a crash.

The indicator itself is drawn by *appending* one character to each already
rendered, uniform-width line — inside the existing `case "scroll", "auto":`
branch of the height switch (`block.go:411`), right where `offset`/
`maxOffset`/`heightLines` are already local variables, gated on
`ovY == "scroll"` specifically (not `auto`). Appending, never overwriting
(`spliceColumns`), makes content-clobbering structurally impossible — this
is the direct fix for the overlay risk discussed before writing this down.
This happens *before* padding-right/borders are applied around the box, so
the gutter sits inside the border alongside padding-right, exactly where a
real scrollbar occupies part of the padding box. Because the appended
column is on the trailing edge, it needs no `colShift` position-bookkeeping
adjustment (`block.go`'s final `mergePositions` call) — only left-side
insertions (padding-left, left border char, margin-left) ever need that.

**Thumb math**, standard proportional scrollbar formula, using the same
`offset`/`heightLines`/total-line-count already available at the append
point:

```
thumbSize  = clamp(round(heightLines * heightLines / totalLines), 1, heightLines)
thumbStart = round(offset * (heightLines - thumbSize) / maxOffset)   // 0 if maxOffset == 0
```

When `totalLines <= heightLines` (nothing to actually scroll — `maxOffset ==
0`), this naturally reduces to `thumbSize == heightLines`, i.e. the thumb
fills the whole track — matching a real scrollbar's own convention for "you
can see everything already," and requiring no special-case branch.

Default glyphs — `"│"` (track) and `"█"` (thumb) — and the default 1-column
gutter width are now CSS-configurable via `::scrollbar`/`::scrollbar-track`/
`::scrollbar-thumb`; see "Scrollbar pseudo-elements" immediately below for
the mechanism and CSS.md's own "Scrollbar pseudo-elements" section for the
user-facing reference.

### Scrollbar pseudo-elements: `::scrollbar`/`::scrollbar-track`/`::scrollbar-thumb`

**Reuses the existing `::before`/`::after`/`::marker` pseudo-element
machinery, not a new mechanism.** `cssengine`'s pseudo-element handling was
already generic over the pseudo-element name: `cascade.PseudoElement(n,
which)` (`cascade.go`) matches any rule whose selector ends in `::which`
against a real node `n`, with full cascade/specificity/`!important`
resolution, and `specificity()` (`selector.go`) already scores any non-empty
`pseudoElem` the same way regardless of its name. The only actual gate was
`parseSimpleSelector`'s pseudo-element name whitelist (`selector.go`, the
`case "before", "after", "marker":` switch) — widened to also accept
`"scrollbar"`, `"scrollbar-track"`, and `"scrollbar-thumb"`. Everything else
below is new call sites in `internal/render`, not new `cssengine` logic.

**Matched against the scrollable element itself**, exactly like `::before`
is matched against the element it decorates — `.log-pane::scrollbar-thumb {
… }` scopes to one pane; a bare `::scrollbar-thumb { … }` (implicit
universal selector, same as bare `::before` already works) applies to every
scrollable element. This is a deliberate divergence from real CSS's
`::-webkit-scrollbar-thumb` (element-and-vendor-prefix-scoped only) toward
something closer to `::selection`'s scoping model — htmlterm has no
cross-browser vendor-prefix convention to converge on, so there's no reason
to imitate one.

**`::scrollbar { width }`** (`Engine.scrollbarGutterWidth`, `block.go`)
resolves a `width` declaration (same `ch`/bare-integer forms as any other
[Size Value](../CSS.md#size-values); percentages are treated as unset —
there's no meaningful "percentage of the gutter" concept) in place of the
`ScrollbarGutterWidth` default constant, at the same point in
`renderBlockContentBox` that already computes `gutterWidth` before wrapping
(the "Mechanism: reserve a column, then append to it" section above still
applies unchanged — reservation still happens up front, still narrows
`innerW` before `wordWrapTokens` runs, just by a resolved width instead of a
hardcoded `1`). `ScrollbarGutterWidth` (`htmlterm.ScrollbarGutterWidth`)
remains exported and keeps its original meaning — the *default* gutter
width — for pre-render callers (`Document.SetPreRendered`); a caller who
also sets a custom `::scrollbar { width }` on the live pane is responsible
for accounting for that override in its own pre-render pass, same as it
already has to account for any other CSS that affects the live pane's
content width.

**`::scrollbar-track`/`::scrollbar-thumb`** (`Engine.resolveScrollbarStyle`,
`block.go`) resolve `content` (via the same `parseCSSContentString` helper
`::before`/`::after` use, so it accepts a quoted-string literal but falls
back to the built-in default glyph when unset — `"none"`/`"normal"` and the
`attr()`/`counter()`/quote tokens are all accepted syntactically but have no
practical use here) plus `color`/`background-color`/`font-weight` (via
`extractInlineStyle`, the same struct/rendering path ordinary inline text
styling already uses) into a `scrollbarStyle{char, style}`. `block.go`'s
existing `appendScrollbarColumn` call site resolves both once per gutter
(not once per line) and passes them down; the function's own per-line loop
picks `track` or `thumb` exactly as it always did, and now renders
`style.render(strings.Repeat(char, gutterWidth), profile)` instead of a bare
hardcoded character — the repeat is what makes a `width > 1` gutter show the
same glyph across every reserved column rather than requiring a
column-by-column pattern (no such per-column pattern concept exists; out of
scope, not just unimplemented — `content` is one glyph, not a bitmap).

**`scrollbar-style: block|shaded|classic|ascii`** (`scrollbarPresets`,
`Engine.resolveScrollbarStyle`, `block.go`) is a shorthand set on the
scrollable element itself, not on `::scrollbar-track`/`::scrollbar-thumb` —
it can't be, since a preset needs to supply *both* pseudo-elements' baseline
from one declaration on the one real element both pseudo-elements are
matched against. This is a different shape from `margin`/`padding`/
`overflow`'s existing shorthand-expansion mechanism (`expandShorthand`,
`css.go`), which expands into longhands on the *same* selector target at
parse time — `scrollbar-style` instead has to be read from the scrollable
element's own resolved decls (`renderBlockContentBox` already has this as
its `decls` parameter, threaded into `resolveScrollbarStyle` as `elemDecls`)
and merged against a *different* selector target's cascade result
(`r.pseudoElemDecls(n, which)`) at render time. The two are merged
property-by-property — preset supplies a baseline map, the real
`::scrollbar-track`/`::scrollbar-thumb` rule's own resolved decls are
copied on top (`maps.Copy`) — so `scrollbar-style: classic` plus a lone
`::scrollbar-thumb { color: red }` combine (classic's `content`/
`background-color` baseline, plus this rule's `color` on top) rather than
one replacing the other outright. An unset or unrecognized
`scrollbar-style` value falls back to `defaultScrollbarStyle` ("block"),
which is defined to reproduce this feature's own pre-`scrollbar-style`
behavior exactly — dropping the UA stylesheet's original
`::scrollbar-track { content: "│"; }`/`::scrollbar-thumb { content: "█"; }`
rules (now redundant: `block`'s Go-level preset defaults supply the same
values) was necessary specifically so an *unset* real `::scrollbar-track`/
`::scrollbar-thumb` rule reads back as genuinely empty from
`pseudoElemDecls` — if the old UA rules had stayed, they'd unconditionally
win the merge (real cascade rules always beat a Go-level fallback the way
this is wired) and no non-`block` preset's own `content` would ever be
visible.

`classic`'s track/thumb colors (`#444444`/`#aaaaaa`, a neutral gray pair
evoking the flat, non-glyph classic Mac/Windows scrollbar look — plain
`" "` content, distinguished only by `background-color`, not by a glyph)
are not independently CSS-configurable through the `scrollbar-style`
keyword itself — the design choice here is that `scrollbar-style` names a
small, fixed set of *complete* presets (mirroring `border-style`'s own
`solid`/`rounded`/`thick`/`double` preset model in spirit), not a
parameterized shorthand; a different palette on `classic` is one
`::scrollbar-thumb { background-color: … }` rule away, using the
property-level override the merge already supports.

### Scrollbar cap buttons: `::scrollbar-cap-start` / `::scrollbar-cap-end`

Arrow-button cells at the two ends of the track (real GUI/terminal
scrollbars commonly draw `▲`/`▼` here). Three decisions, confirmed with the
user before implementing:

- **Clickable**, not just decorative — a click on the cap's cell scrolls one
  line, matching `DispatchKey`'s own `ArrowUp`/`ArrowDown` step.
- **New pseudo-elements, opt-out (revised from an initial opt-in design)** —
  first shipped requiring an explicit `content` rule (mirroring `::before`/
  `::after`'s own "no content, no injection" contract) with no
  `scrollbarPresets` involvement at all; the user then asked for the
  opposite — caps on by default, with basic glyphs added to the UA-visible
  defaults, and each `scrollbar-style` preset given its own cap glyphs too.
  `scrollbarPresets` (the same table `resolveScrollbarStyle` already reads
  for track/thumb) gained `capStart`/`capEnd` entries per preset, and
  `resolveScrollbarCap` was rewritten to do the identical
  preset-baseline-plus-override merge `resolveScrollbarStyle` already does —
  so `overflow-y: scroll` alone now gets cap buttons, styled per whichever
  `scrollbar-style` is active. The per-element opt-*out* escape hatch is
  `content: none` on `::scrollbar-cap-start`/`::scrollbar-cap-end` — that
  still works exactly as designed the first time (`parseCSSContentString`
  already treats `none`/`normal` as empty, so no extra code was needed for
  this once the merge existed), it's just no longer the *only* way to get no
  cap (a too-short gutter still drops both automatically too — see
  "Rendering" below).
- **Named `-start`/`-end`, not `-top`/`-bottom`** — matches WebKit's own
  `::-webkit-scrollbar-button:start`/`:end` precedent, and stays meaningful
  if a horizontal scrollbar is ever added (this axis has no "top" to alias).

**Resolution** (`Engine.resolveScrollbarCap`, `block.go`) mirrors
`resolveScrollbarStyle`: look up `scrollbarPresets[elemDecls["scrollbar-style"]]`
(falling back to `defaultScrollbarStyle`, "block") for a baseline `capStart`/
`capEnd` map, copy `r.pseudoElemDecls(n, which)` on top per-property, then
compute `content` via `parseCSSContentString`. `ok` is `false` only when
that resolves empty — which happens either because an explicit rule set
`content: none`/`"normal"` (parseCSSContentString's own existing handling,
unchanged) or, in principle, a `scrollbar-style` preset that supplied no cap
content at all (none currently do — every preset defines both ends).

**Rendering** (`appendScrollbarColumn`, `block.go`) draws a cap by claiming
row 0 (`capStart`) and/or the last row (`capEnd`) verbatim instead of
computed track/thumb, and runs the existing thumb-size/thumb-position
formula against the *interior* track — `heightLines` minus however many caps
are actually active — so the thumb never overlaps a cap. If there isn't at
least 1 interior row left once active caps are subtracted
(`heightLines-activeCaps < 1`), **both** caps are dropped for that render,
not just the one that doesn't fit — the same "silently drop the added
chrome, keep the rest correct" precedent the gutter reservation itself
already established (see "Scrollbar gutter and indicator" above). The
function now returns which caps it actually drew (`bool, bool`), since that
can differ from which caps were *requested* (`hasCapStart`/`hasCapEnd`) in
exactly that no-room case — the caller needs the drawn value, not the
requested one, for the click hit-testing geometry below.

**Click hit-testing** needed a route from render-time geometry back to
`document.go`, which never previously needed to know the gutter's *column*
range (only `Height`/`TopOffset`, for `PageUp`/`PageDown` step size and
scroll-into-view math). `Viewport` (`engine.go`) gained `GutterCol`,
`GutterWidth`, `CapStart`, `CapEnd`:

```go
type Viewport struct {
	Height      int
	TopOffset   int
	GutterCol   int
	GutterWidth int
	CapStart    bool
	CapEnd      bool
}
```

`GutterCol` is relative to the element's own `Rect.Col`, the same relationship
`TopOffset` already has to `Rect.Row` — computed at the existing
`r.liveScrollViewport[n] = ...` assignment (`block.go`, right where
`colShift := pl + runeLen(bl.char) + ml` is already computed for
`mergePositions`) as `colShift + innerW`: `colShift` is the offset to where
this box's own *content* starts (margin-left included, matching `Rect`'s own
documented "margin baked in, not subtracted back out" imprecision — the
same one every other click/wheel hit-test on a scrollable pane already
lives with), and the gutter sits immediately after content. `CapStart`/
`CapEnd` are the *drawn* booleans `appendScrollbarColumn` returns, not
whether a rule was set — so a cap dropped for lack of room correctly reads
back as not clickable.

`document.go`'s new `tryScrollCapClick(scrollable *html.Node, row, col int) bool`
combines `d.scrollViewport[scrollable]` with `d.positions[scrollable]`
exactly the way `scrollIntoView`/`ScrollVisible` already combine `Viewport`
and `Rect` — `rect.Col + vp.GutterCol` for the column range, `rect.Row +
vp.TopOffset` / `+ vp.Height - 1` for the two candidate rows — and on a hit,
mutates `d.scrollOffsets[scrollable]` by ∓1 directly, unclamped (clamped on
the next `Render()`, the same pattern every other `scrollOffsets` mutation
site already follows). `DispatchClick` calls this *before* falling through
to its own `elementAt`-based dispatch, but after `closeSelectsExcept(target)`
so an open `<select>` popup still closes on any click, cap included. A cap
hit does **not** dispatch a `"click"` `Event` on the scrollable element —
deliberately mirrors `DispatchWheel`'s own direct-mutation, no-event shape
rather than `DispatchClick`'s normal target-dispatch path, since a cap is
rendering chrome, not real element content, and dispatching a real click
there risked interfering with a listener or a submit/checkbox default
action attached to the scrollable element itself.

**Not implemented / deliberately out of scope, same reasoning as
`::marker`'s own property list:** `font-style`, `text-decoration`, and
`opacity` on `::scrollbar-track`/`::scrollbar-thumb` — `extractInlineStyle`
technically supports them since it's the same shared struct/function
`::marker` and ordinary inline text use, so they're not blocked by any
missing plumbing, just not called out in CSS.md's supported-properties list
for this pseudo-element pair (matching `::marker`'s own documented subset)
and not covered by tests here. A user relying on them is relying on
undocumented behavior that happens to work, not a supported contract.

### Rejected alternative (scrollbar)

Overlaying the indicator onto the already-rendered, full-width content via
`spliceColumns` (`textutil.go:340`), instead of reserving a column up front.
Rejected: it overwrites whatever character was already at that column on
each line, and the content most likely to need a scrollbar in the first
place — logs, code, `pre` blocks, wide tables — is exactly the content most
likely to already be using the box's full width on every line, making
silent right-edge corruption the common case, not a rare one. It would also
conflate with `text-overflow`'s existing ellipsis truncation (now two
unrelated reasons a line's last character could look cut). `spliceColumns`
remains genuinely useful for an actual floating overlay (a real popup, per
`RENDERING.md`) — just not for this.

### Explicit non-goals for the scrollbar

- Horizontal indicator (`overflow-x: scroll`'s gutter/track/thumb) — no
  horizontal scroll offset exists to indicate in the first place (see
  Section 1's own non-goals).
- A gutter/indicator for `overflow-y: auto` that only appears when content
  actually overflows — would need the two-pass wrap this design deliberately
  avoids; `auto` stays exactly as already shipped (functional, silent).
- `font-style`/`text-decoration`/`opacity` on `::scrollbar-track`/
  `::scrollbar-thumb` — see "Scrollbar pseudo-elements" above.
- Horizontal scrollbar-gutter reflow interaction with `text-overflow` at
  both edges of a doubly-clipped nested region (already a named Section 1
  non-goal; unchanged here).

### Rejected alternative

A virtual/windowed layout that only lays out the visible slice of tokens.
Rejected because `wordWrapTokens` needs the whole content to resolve wrap
points, and partial layout would break re-render-on-resize and position
tracking for below-the-fold content. Full-render-then-slice — generalizing
what `overflow:hidden` already does, just with a nonzero offset — is
simpler, consistent with the existing "always fully re-render" model, and
means nested scrollable regions need no special-case code.

### Explicit non-goals for v1

- Horizontal scroll offset.
- "Sticky to bottom" auto-follow for growing content (e.g. a log tail) —
  numeric clamping survives content mutation, but nothing auto-follows;
  that would be a separate opt-in behavior.
- Doubly-clipped nested regions' `text-overflow` marker interaction at
  both edges.

## Section 3 — Focus: scroll-into-view is the one real gap

`Focus`/`FocusNext`/`FocusPrev` (`document.go:352`, `392`, `412`), after
setting `d.focused`, should walk `ancestorChain(el.node)` and for each
ancestor with a `scrollOffsets` entry, compare the focused element's
previous-frame `Rect` (from `d.positions`) against that ancestor's own
previous-frame `Rect`, and shift+clamp the offset if the focused element
falls outside the ancestor's visible range. This is one-frame-stale by
construction — consistent with `Rect`'s existing documented staleness —
and a no-op before the first `Render()` (`d.positions == nil`), an
acceptable, honestly-documented v1 gap. Applies uniformly to any focusable
control inside any scrollable pane, including nested ones.

`focusableList()` (`document.go:332`) should **not** filter by current
clip/visibility state. Real browsers keep scrolled-out elements in tab
order and auto-scroll them into view on focus, which is exactly what the
fix above does; filtering would be the rejected alternative, and it
contradicts the project's stated "mirror real DOM semantics" principle —
see `INTERACTIVE.md`'s design goal and the project's own confirmed
takeaway that this pays off in practice (every feature added under that
constraint so far has been both easy to design and easy to explain).

**Keyboard-accessible scroll containers (implemented):** `Document.isFocusable`
also treats a scroll container (a key in `d.scrollOffsets`) as a tab stop when
it has no focusable descendant of its own (`hasFocusableDescendant`) —
mirroring real browsers making an otherwise-keyboard-unreachable scrollable
region focusable, so a pane with no button/input inside it (mouse-wheel-only
otherwise) is still reachable via Tab, letting `DispatchKey`'s
`PageUp`/`PageDown`/`ArrowUp`/`ArrowDown` scroll it directly once focused. A
container that already has a focusable descendant (e.g. a pane with a button
in it) is reached through that descendant instead and isn't also made its own
redundant stop.

Two smaller, honestly-scoped gaps, named but not designed here:

- No explicit `tabindex` *attribute* support — a plain non-scrollable,
  non-form element can't be opted into tab order via `tabindex="0"`, and
  `focusableList` never reorders for a positive `tabindex`. (Distinct from
  the scroll-container case above, which is implicit/automatic and unrelated
  to the `tabindex` attribute.)
- No `autofocus` attribute handling at first render. `Document.Focus(el)`
  already fully covers programmatic "set focus"; the one plausible
  addition is honoring `autofocus` at `ParseDocument`/first `Render` time
  — a single `isFocusable` check plus one `Focus()` call — for hosts that
  want initial focus solved declaratively rather than only imperatively.

## Layering note

Section 1 and Section 3 touch `document.go` (new state + rendering hook +
focus scroll-into-view), `block.go` (the overflow gate), `wraptoken.go`
(position shifting), the terminal I/O layer (wheel decoding — originally
`input.go`, now `tcell.Screen`/`tcell_loop.go`, see INTERACTIVE.md), and
`event.go`/`document.go` (new dispatch entry point + key defaults) — no
changes anticipated to `Renderer`'s public `Render(htmlStr) (string, error)`
contract, consistent with the one-directional layering already
established (`Loop → Document → Renderer.Render`, see `INTERACTIVE.md`).
Flexbox landed within that same boundary: `internal/render/flex.go` is new
box/token-model layout code plus dispatch wiring in `render.go`/`inline.go`,
and did not touch `Loop` or the event/focus layer beyond what Section 1
already added.

The scrollbar pseudo-elements landed within the same boundary too:
`internal/cssengine/selector.go` (pseudo-element name whitelist),
`internal/render/engine.go` (dropping the now-redundant UA
`::scrollbar-track`/`::scrollbar-thumb` content rules), and
`internal/render/block.go` (`scrollbarPresets`/`scrollbarGutterWidth`/
`resolveScrollbarStyle`/`appendScrollbarColumn`, and the two call sites that
use them) are the only touched files — no changes to `cascade.go`'s
`PseudoElement` (already generic over the pseudo-element name), `document.go`,
`event.go`, or `Renderer`'s public contract. `scrollbar-style` itself needed
no `cssengine` changes at all — it's read as a plain, already-parsed
property off the scrollable element's own resolved decls, not a new
selector or shorthand-expansion form.

The scrollbar cap buttons crossed one more boundary than any prior piece of
this feature: `document.go` (`DispatchClick`'s new early branch plus the new
`tryScrollCapClick` helper), because click-to-scroll is a `document`-layer
concern (`Document.scrollOffsets`/`scrollViewport`, hit-testing), not a
`render`-layer one — `render.Viewport`'s four new fields are exactly the
data contract that crossing needed, mirroring how `Height`/`TopOffset`
already crossed that same boundary for `PageUp`/`PageDown` and
scroll-into-view. Otherwise the same files as before: `selector.go` (two
more whitelisted pseudo-element names) and `block.go`
(`resolveScrollbarCap`, `appendScrollbarColumn`'s extended signature, and
the `Viewport` construction site). No change to `cascade.go`'s
`PseudoElement`, `event.go`, or `Renderer`'s public contract.
