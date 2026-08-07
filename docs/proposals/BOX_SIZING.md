# box-sizing: real property support, border-box via the UA stylesheet

Status: **implemented**, for ordinary block boxes, flex containers, and a
flex item's cross axis. Table cell width remains unimplemented; see
"Table cells" below and `COMPATIBILITY.md`. This document is kept as the
design record, updated in place to match what shipped and to correct one
claim ("not a breaking change") that turned out to be wrong for the
`height` axis specifically, plus two implementation bugs a later code
review caught on the `width` axis. See "Corrections found during
implementation" near the end before trusting anything above it about
breaking-change scope or row-direction box-sizing conversion.

## Problem

`box-sizing` is currently parsed and thrown away (`grep -rn box-sizing
internal/render` matches nothing outside comments and docs). Regardless of
what an author sets, the two axes behave differently, and neither reads the
property:

- **`width`** always behaves like `border-box`: a declared width is the
  box's total border-box size, and padding/border are subtracted out of it
  to find the content width (`renderBlockContentBox`, `block.go:404`).
- **`height`** always behaves like `content-box`: a declared height is the
  content size, and padding/border rows are added on top
  (`ARCHITECTURE.md`'s "Key invariants", `COMPATIBILITY.md`'s width/height
  asymmetry entry).

`COMPATIBILITY.md`'s current `box-sizing` bullet also mis-describes this: it
says padding/border are "subtracted from a declared `width`/`height`... not
added on top", which is true for `width` but contradicts the width/height
asymmetry entry a few paragraphs above it, which correctly says `height`
adds padding/border on top. That bullet gets rewritten as part of this work,
not just left as a stale description of the old behavior.

The real gap is narrower than "wrong default": it's that `box-sizing` has no
effect at all, so there is no way to opt into the *other* model on either
axis. `width` can't be told to use `content-box`; `height` can't be told to
use `border-box`.

## Decision

Implement `box-sizing` as a real, cascade-resolved property with both
values, `content-box` and `border-box`, and put

```css
*, ::before, ::after { box-sizing: border-box; }
```

in `internal/render/defaultstylesheet.go`, so `border-box` is the *default
a document actually sees*, even though `content-box` stays the property's
real initial value underneath it.

This is the same mechanism, at the same layer, that essentially every
real-world stylesheet already uses to get exactly this default. Bootstrap's
Reboot: "The `box-sizing` is globally set on every element... to
`border-box`. This ensures that the declared width of element is never
exceeded due to padding or border." `sindresorhus/modern-normalize`: the
identical `*, ::before, ::after { box-sizing: border-box; }` rule, with the
same rationale. htmlterm doing this in its own UA stylesheet isn't a
deviation from spec; it's moving a near-universal author convention down
one cascade layer, using cascade machinery (`ARCHITECTURE.md`'s documented
order: UA stylesheet → `Options.CSS` → `Options.Stylesheets` → `<style>` →
inline `style=`) that already exists and needs no new code to support it.
Anyone who wants literal spec-default behavior gets it the same way they'd
get it in a browser without a reset: `box-sizing: content-box` (or
`initial`, or `unset`, both already supported CSS-wide keywords) on
whatever they want it for.

**This is not a breaking change to default output, on the `width` axis.**
The `border-box` formula this implements has to equal today's unconditional
width formula exactly (see "What this requires" below), and the UA
stylesheet now selects that formula for everyone who never touches
`box-sizing`. Every existing worked width/padding/border example in
`CSS.md` and `COMPATIBILITY.md` stays numerically correct with no changes
needed on that axis.

**It is a breaking change on the `height` axis, discovered only once this
was implemented and the test suite run against it; see "Corrections found
during implementation" for the full account.** `box-sizing` has to apply
uniformly to both axes to mean what it means in real CSS, but `height`'s
own unconditional behavior, before this proposal, was already
content-box-shaped, the opposite of `width`'s. There is no default value
that leaves both axes' existing output unchanged, because that existing
output was never a real `content-box` or `border-box` box model to begin
with, on either axis; it was an unlabeled asymmetric mix this proposal
exists to replace. Defaulting to `border-box` preserves `width` and changes
`height` for anyone combining an explicit `height` with padding or a
border. That is exactly what adopting a Bootstrap-style reset changes on a
real page too, and was accepted as the intended outcome, not a regression
to avoid.

**Flex items are out of scope for this pass**, kept on today's outer-size
model regardless of `box-sizing`. `COMPATIBILITY.md`'s `flex-basis`/`width`
entry already documents why: main-axis distribution, wrapping, and the
anti-corruption clip (the "over-shrunk flex item" entry) all depend on
`flex-basis`/`width` sizing an item's whole outer box, which is what
`border-box` already means. Since the UA default is now `border-box`
everywhere, this isn't even a visible gap for a flex item that inherits the
default; it only matters if someone explicitly sets `box-sizing:
content-box` on a flex item and expects it to take effect, which it won't.
`box-sizing` becomes inert on a flex item's main axis, the same category
`border-width` already sits in, rather than being half-implemented there.
Revisit only alongside a rework of flex sizing around Flexbox §9.9's
chosen-flex-fraction machinery, an already-open gap (`COMPATIBILITY.md`'s
`flex: 1` entry).

## What this requires

### 1. Read `box-sizing`

Two values matter: `content-box` and `border-box`. Values arrive lowercased
already, per `keywordcase.go`'s cascade-time folding, so no
case-insensitive comparison is needed at the read site. An unset or
unrecognized value falls back to `content-box`, matching real CSS's initial
value; in practice nearly every element will instead resolve `border-box`
through the new UA rule, same as a real browser under a Bootstrap-style
reset.

### 2. Add the UA rule

`*, ::before, ::after { box-sizing: border-box; }` in
`defaultstylesheet.go`. Both the universal selector and `::before`/`::after`
pseudo-elements are already supported by `internal/cssengine`'s selector
matching (`defaultstylesheet.go:55-58` already has `q::before`,
`img::before`, and others), so this is an ordinary rule addition, not new
selector-engine work. Confirm during implementation that `*` alone doesn't
already implicitly reach `::before`/`::after` here (it doesn't in real CSS,
which is exactly why Bootstrap/modern-normalize spell out all three
selectors rather than relying on `*` to cover generated content); write the
explicit three-selector form regardless, to match the well-tested
real-world precedent instead of relying on an assumption about this
engine's own pseudo-element matching.

### 3. Branch the one block-level call site

`renderBlockContentBox` (`block.go:402-415`) is where a declared `width`
currently gets turned into a content width:

```go
if totalW, constrained := resolveWidthConstraints(decls, availWidth, availWidth); constrained {
    inner := totalW - ml - textcell.Width(bl.char) - pl - pr - textcell.Width(br.char) - mr
    if inner < 1 {
        inner = 1
    }
    hBorderWidth = textcell.Width(bl.char) + pl + inner + pr + textcell.Width(br.char)
    hasExplicitWidth = true
}
```

This formula becomes the `border-box` branch, unchanged. `content-box`
takes `totalW` as the content width directly:

```go
inner := totalW
if inner < 1 {
    inner = 1
}
hBorderWidth = textcell.Width(bl.char) + pl + inner + pr + textcell.Width(br.char)
```

`hBorderWidth` (the box's border-box total, excluding margin) can now
exceed `availWidth - ml - mr` for a `content-box` element, which is the
overflow case handled in section 5. Since `border-box` is the default via
the new UA rule, this is the entire change for ordinary block boxes: one
branch at one site, reachable only when an author explicitly opts into
`content-box`.

### 4. Table cells

This is a compatibility gap against spec, not a case spec exempts: CSS Box
Sizing Module Level 3 applies `box-sizing` to "all elements that accept
width or height," internal table elements included, and every mainstream
browser honors it on `<td>`/`<th>` the same as on a `<div>`. htmlterm's
cells just don't read the property.

`table.go`'s `sizeColumns` and the column-width algorithms in
`table_separate.go`/`table_collapse.go` resolve a cell's declared `width`
through a mechanism that already doesn't match either real `box-sizing`
value on its own: `border-box`-shaped for padding (subtracted from the
declared width) but `content-box`-shaped for border characters (added on
top as separate overhead tracked outside that width entirely, consistent
with `docs/TABLES.md`'s border unification model). Giving cells real
`box-sizing` means replacing that mixed model with an actual
`content-box`/`border-box` pair first — rewriting `sizeColumns`'s
fixed-column branch and threading border overhead through it consistently
— not adding a branch alongside the existing formula the way the
block-level case allowed.

### 5. Overflow policy for `content-box` + `width: 100%` + padding or border

Under real content-box semantics this combination legitimately produces a
border-box total wider than the containing block, the classic browser
box-model gotcha the UA-level `border-box` reset (section 2) exists to
avoid by default. Because it's now genuinely opt-in, this only has to be
solved for someone who deliberately reaches for `content-box`, not for
every document.

**As implemented, no new code was needed for this at all.** The `content-box`
branch simply lets `hBorderWidth` exceed `availWidth - ml - mr`; every
downstream step (wrapping, alignment, padding, border-drawing) already
operates on `hBorderWidth`/`innerW` as plain numbers with no assumption
that they fit inside the caller's available width, so the box's lines come
out longer than the space it was offered and get embedded as-is by its
parent, exactly reusing the precedent `COMPATIBILITY.md` already states for
unbreakable content: "a plain bordered block whose unbreakable content
exceeds its `width` also paints past its own border." Nested blocks stack
vertically, so one painting wider than intended doesn't corrupt a sibling's
columns the way a flex item would, which is what makes this safe to leave
as an emergent property rather than something requiring an explicit clip
rule. The "does the outermost frame clip at `Options.Width`" question this
section originally raised turned out not to need a separate answer: the
outermost frame is just another caller embedding a child's lines, subject
to the same existing behavior.

### 6. Border-box `height`

Real CSS's `border-box` value already applies uniformly to both axes, so
this proposal's `border-box` value has to as well: it subtracts top/bottom
border rules and vertical padding from the declared height instead of
adding them on top, with the same `< 1` floor the width path uses. This is
net-new code (nothing lets height do this today), factored as a standalone
`borderBoxContentHeight(decls, h)` helper (`block.go`) rather than inlined,
because it turned out to have two callers, not one — see the flex
correction below. `content-box` height needs no change: that's already
today's unconditional behavior, now reachable only when `box-sizing:
content-box` is set.

### 7. Flex items and flex containers

**A flex item's main-axis size** (the `width`/`height` override
`renderFlexItemBoxSized` injects to drive flex distribution) is exempt from
`box-sizing`: it always behaves as an outer, border-box-shaped size,
regardless of what the item's own `box-sizing` resolves to. This is not
"no sizing change" in the sense of no code at all, contrary to what this
section originally said before implementation: the injection site has to
actively neutralize the item's real `box-sizing` for that one synthesized
value, or a `content-box` item's flex-resolved size would be
reinterpreted as a *content* size and grow past its allotment. See
"Corrections found during implementation" for why this needed real code,
found via a failing test, not just a documentation note.

**A flex container's own outer box is not exempt** and follows
`box-sizing` the same as any ordinary block box, on both axes. This wasn't
explicitly called out in the original version of this section, which only
discussed items; it needed its own fix during implementation once flex
containers turned out to have two separate box-sizing-sensitive numbers of
their own (their outer painted box, and their internally-resolved content
height for laying out children), both of which needed the same
`borderBoxContentHeight` conversion `renderBlockContentBox` uses. See the
correction below.

## Corrections found during implementation

Two things in the sections above turned out to be wrong or incomplete once
this was actually built and run against the existing test suite. Recorded
here rather than silently edited away, since both are the kind of mistake
worth not repeating.

**"Not a breaking change" was true for `width` and false for `height`.**
The original version of this document's "Decision" section claimed the
whole proposal was non-breaking, reasoning about `width` in isolation and
not checking `height` against the same claim. Running the full test suite
after implementing the `height` side (`go test ./...`) surfaced 6 failing
cases, all a declared `height` plus `border-style`/padding coming out
shorter than before: `<div style="height:50%;border-style:solid">x</div>`
on an 8-row viewport used to render 4 content rows + 2 border rows = 6
total; it now renders 2 + 2 = 4, because the new border-box-height math,
which is what the UA default now selects, subtracts the 2 border rows from
the resolved 4 instead of adding them on top. This was confirmed correct
and intended (see the "It is a breaking change on the `height` axis"
paragraph above) rather than fixed around; the 6 tests were updated to the
new expected output.

**Flex containers needed the same border-box-height conversion items'
synthesized sizes get, or nested grown containers overshoot their
allotment.** `renderFlexItemBoxSized` injects a flex item's resolved
main-axis size as a raw border-box total when the item resolves to
`border-box` (the common, default case), trusting that whichever renderer
reads `decls["height"]` next will subtract the item's own chrome, the way
`renderBlockContentBox`'s new border-box branch does. That assumption
silently broke for a flex item that is *itself* a nested flex container
(a `display:flex` item with `flex-grow`, a common pattern for the "fills
the screen, one panel grows" layout `CSS.md`'s Flexbox section documents):
such an item renders through `renderFlexContentBox`, not
`renderBlockContentBox`, and `renderFlexContentBox`'s own
`resolveFlexContainerHeight` read `decls["height"]` with no `box-sizing`
awareness at all, so the injected value was never converted anywhere, and
the container's own border/padding rows were added on top of it in
addition to being implicitly assumed already subtracted. `go test ./...`
caught this as 4 failing `TestFlexNestedContainerMainSize` cases, each
overshooting its allotted height by exactly its own chrome. The fix:
`resolveFlexContainerHeight` and `resolveFlexContainerHeightFloor`
(`flex.go`) now call the same `borderBoxContentHeight` helper
`renderBlockContentBox` does, whenever the container resolves to
`border-box`, which is what section 7 above describes as "a flex
container's own outer box is not exempt."

Both corrections were caught by running `go test ./...` immediately after
each implementation step, not by reasoning alone. Nothing else in this
document's original scope changed; table cells (section 4 above) were the
one piece deliberately deferred going in, and stayed deferred.

**A code review of this feature, after it shipped, found two more bugs, both
on the `width` axis this time and both caught by reasoning about the code
rather than a failing test, since neither had a regression test at the
time.**

**A content-box flex item's own horizontal margin was subtracted twice.**
`renderFlexItemBoxSized`'s width override pre-subtracted
`blockHorizontalChrome`, which folds in horizontal margin along with border
and padding, to convert a flex-resolved outer size into the content width
`block.go`'s content-box formula expects. But that formula already
subtracts margin from the declared width on its own, independently of
box-sizing (see section 3's "content-box takes `totalW` as the content
width directly", and `ARCHITECTURE.md`'s block border box model
invariant). A content-box item with `margin-left` or `margin-right` lost
that margin's width a second time, painting narrower than its flex-basis
by exactly its own horizontal margin. The fix: the override's own
pre-subtraction now uses `blockHorizontalBorderPadding`, border and padding
only, leaving margin to the one subtraction `block.go` already does. See
`TestBoxSizingFlexItemMarginNotDoubleSubtracted`.

**Row-direction min-width/max-width never got the box-sizing conversion
their column-direction counterparts (min-height/max-height) already had.**
`mainAxisHeightBound` converts a content-box min-height/max-height/height
value to the outer size flex layout compares it against, and
`flexGrowHeightCeiling`, `flexMainAxisHeightFloor`, and
`clampFlexMainHeight` all call it. The row-direction equivalents,
`flexGrowCeiling`, `flexMainAxisFloor`, and `clampFlexWidth`, had no such
conversion at all: they compared a raw content-box min-width/max-width
value directly against an item's outer main-axis size. A bordered
content-box item stopped growing, or floored while shrinking, its own
border and padding short of its declared bound; `max-width: 4` on a
bordered item capped it at 4 total columns instead of 4 content columns
plus its border. The fix: `mainAxisWidthBound`, `mainAxisHeightBound`'s new
row-direction counterpart, wired into all three functions. It adds
`blockHorizontalChrome`, not `blockVerticalChrome`, since a flex item's
width-axis size bakes in horizontal margin the way every other width does
in this engine, where a column-direction item's height never bakes in
vertical margin; the two conversions differ by exactly that term. See
`mainAxisWidthBound`, `TestBoxSizingFlexMaxWidthClamps`, and
`TestMainAxisWidthBoundMirrorsHeightAxis`.

Both fixes are covered by regression tests now. The same review also
prompted two simplifications with no behavior change: `block.go`'s and
`flex.go`'s near-identical content-box/border-box width formulas were
factored into a shared `borderBoxContentWidth` helper, `borderBoxContentHeight`'s
width-axis counterpart; and `mainAxisHeightBoundFunc`/`mainAxisWidthBoundFunc`
resolve an item's chrome once per call for a caller like `clampFlexWidth` or
`clampFlexMainHeight` that converts more than one bound for the same item,
rather than paying for `resolveBoxBorders` again for each one.

## Non-goals

- No change to `border-width`/`border-*-width`; still a no-op. The
  box-sizing math uses each border side's *rendered glyph width* (0 or 1,
  depending on `border-style`), the same quantity `resolveBoxBorders`
  already exposes, not a numeric border-width.
- No independent `box-sizing` handling for `inline-block` in this pass.
  `COMPATIBILITY.md` already documents that an `inline-block` element's own
  border and padding aren't drawn at all; that gap is unaffected by this
  proposal and stays as-is.
- No change to absolute/fixed positioning's shrink-to-fit gap
  (`COMPATIBILITY.md`'s positioned-layout entry); scope here is normal
  block flow and table cells only.
- No hardcoded default in Go. The `border-box` default lives entirely in
  `defaultstylesheet.go` as an ordinary CSS rule, overridable by any author
  CSS the normal way; nothing in `internal/render`'s Go code should special-
  case "no `box-sizing` set" as meaning `border-box`.

## Docs to update

Per `ARCHITECTURE.md`'s rule that a deviation gets recorded in two matching
places:

- `COMPATIBILITY.md`: rewrite the `box-sizing` bullet (currently wrong about
  `height`, and about to become stale about `width` too) to state that both
  values are supported, that the UA stylesheet defaults every element to
  `border-box` the way Bootstrap's/modern-normalize's own reset does, and
  that `content-box` (the real spec initial value) is available by
  overriding it; plus a line in the flexbox gaps section noting
  `box-sizing` is inert on a flex item's main axis.
- `CSS.md`: add a real `box-sizing` entry, and mention the UA default rule
  in the default-stylesheet documentation alongside the existing
  `q::before`/`img::before`/etc. entries.
- `ARCHITECTURE.md`'s "Key invariants" width/height asymmetry bullet needs a
  `box-sizing` qualifier: it currently states an unconditional rule; after
  this change it's the behavior under the *default* `border-box` value
  specifically, with `content-box` as an explicit escape hatch.
- `docs/TABLES.md`: mention that a cell's `box-sizing` isn't read at all
  (section 4 above), the same gap `CSS.md`'s Cell Sizing section and
  `COMPATIBILITY.md` already state, so a reader following `docs/TABLES.md`
  for the full table-styling reference finds the caveat there too rather
  than only in the other two docs. Its own worked examples don't change,
  since nothing about existing cell rendering is different.

## Summary

| Concern | Status |
|---|---|
| Default `box-sizing` seen by a document | Implemented: `border-box`, via `*, ::before, ::after { box-sizing: border-box; }` in `defaultstylesheet.go` — the same mechanism Bootstrap Reboot/modern-normalize use |
| Real spec initial value | Implemented: `content-box`, reachable via an explicit override on any selector |
| `isBorderBox`/`borderBoxContentHeight` helpers | Implemented in `block.go`, shared by `renderBlockContentBox` and flex.go's `resolveFlexContainerHeight`/`resolveFlexContainerHeightFloor` |
| Ordinary block-level boxes, both axes | Implemented: width branch in `renderBlockContentBox` (block.go:~440), height/min-height/max-height via `toContentLines`/`borderBoxContentHeight` |
| Flex containers' own outer box, both axes | Implemented: width branch in `renderFlexContentBox` (flex.go), height via the same `resolveFlexContainerHeight`/`Floor` fix — added during implementation, not in the original scope; see "Corrections" |
| Flex items' main-axis size | Implemented as exempt: `renderFlexItemBoxSized` neutralizes the item's real `box-sizing` for its synthesized override, always treating it as outer/border-box-shaped; covered by `TestBoxSizingFlexItemMainAxisExempt` |
| Table cells | **Not implemented — a compatibility gap versus spec, not a spec exception.** Real CSS applies `box-sizing` to table cells like any other box. Cell width resolution here is a separate, older algorithm (`sizeColumns`/`table_separate.go`/`table_collapse.go`) that already matches neither real value on its own: border-box-shaped for padding, content-box-shaped (purely additive) for border characters |
| `width: 100%` + padding/border overflow | No new code needed: `hBorderWidth` exceeding the caller's available width already flows through existing overflow-painting behavior with no special-casing required |
| Compatibility impact | **Not breaking on `width`** (border-box formula unchanged from before). **Breaking on `height`** for anyone combining an explicit `height` with padding or a border and no `box-sizing: content-box` override — confirmed as the intended outcome; see "Corrections found during implementation" |
| Tests | `box_sizing_test.go` (new); 6 pre-existing tests across `flex_test.go`, `htmlterm_test.go`, `layout_test.go` updated to the new `height` default |
