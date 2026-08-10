# Rendering Architecture V2: Positioned Boxes

## Status

**Implemented in full.** See the migration plan below; all 7 steps are done,
and docs/INTERACTIVE.md's own "Status" line reads "RENDERING.md landed (all 9
steps)". `box` (box.go), `wrapToken`/`wordWrapTokens` (wraptoken.go), and the
`select_popup.go` popup-compositing mechanism described below all exist in
the code as designed. `cappedWriter` is gone, referenced only in comments
explaining what replaced it. docs/INTERACTIVE.md's Phase 2 ("hit-test map via
zero-width markers") was superseded by this document as planned; see
"Migration plan" step 7 below for what shipped instead, namely
`Document.Rect(el)` reading the position map that composition produces
directly. The rest of this document is kept as design history and rationale,
not a live plan. Read it for the "why", not as a to-do list.

## Motivation

Three separate mechanisms exist today only because position and size aren't
first-class data:

- `cappedWriter`'s deferred-newline buffering (`WriteAtLeastNewlines`,
  writer.go) exists to get CSS margin-collapse right without knowing ahead of
  time how much margin is coming.
- The NBSP-for-unbreakable-content trick (table_render.go's `nbsp` constant)
  exists because content gets re-embedded into strings that might still be
  reflowed after the fact.
- The OSC zero-width marker mechanism designed for INTERACTIVE.md's Phase 2
  exists because there's no other way to recover *where* a node ended up,
  short of smuggling a sentinel through the string and scanning for it
  afterward.

If boxes carry their own size and position as they're composed, all three
collapse into one mechanism instead of three different ones. It also makes
something the current model cannot express at all, regardless of cleverness:
overlapping content such as a `<select>` dropdown or a tooltip. A linear
string has no way to represent two things occupying the same screen area.

## Current architecture — precise findings

These are the concrete seams the rewrite exploits, confirmed by reading the
code rather than paraphrased from memory:

> **Reading note (post-rewrite):** the findings below describe the code *as it
> was before* this rewrite, so their file:line references are long stale.
> `wordWrapANSI`, the wrap entry point that findings 3 and 4 and the sizing
> section are written against, no longer exists. It became a one-line shim
> over `wordWrapTokens` when the token model landed, and was deleted once its
> last production caller went away. Its ANSI-carry documentation now lives on
> `wordWrapTokens` itself (wraptoken.go). Read `wordWrapANSI` below as "the
> wrapping function", which is `wordWrapTokens` today.

1. **The "lines as a slice" representation already exists, it's just
   ephemeral.** `applyLineEdges`, `alignLines`, `padLinesToWidth`, and
   `applyBlockBorders` (all block.go) each independently do
   `strings.Split(content, "\n")` → operate per-line → `strings.Join(...)`.
   `renderBlockContent` itself does the same ad hoc for height-padding and
   text-indent. Every one of these reconstructs and re-flattens a `[]string`
   on every call. The rewrite's core mechanical move is to stop
   reconstructing it repeatedly and thread one shared type through instead.

2. **`renderBlockContent` (block.go:218-453) already distinguishes
   content-box from border-box, just not by type.** Order of operations,
   confirmed from the full body: border-chars/margin/padding parsed →
   `clampCellPadding` (box.go) resolves final `pl`, `pr`, `innerW` →
   `renderInlineAcc` produces raw content → word-wrap
   (`wordWrapANSI(rawContent, innerW, breakMode)`) → overflow/text-overflow →
   text-align → **height/min-height/max-height padding applied to `content`,
   which stabilizes line count** → text-indent → padding baked in via
   `applyLineEdges` → left/right borders baked in via `applyBlockBorders` →
   **top/bottom border-rule lines prepended and appended after that** →
   margins baked in via `applyLineEdges` again. Border-box height (rule lines)
   is deliberately added *after* the content-box height is fixed. That is real
   CSS box model semantics, already implicit in the code's ordering.

3. **Every current word-wrap call site operates on already-fully-flattened
   text**, not on any kind of structured token stream:
   `render.go:29` (root-level transparent content), `list.go:136` and
   `list.go:149` (an `<li>`'s first and subsequent lines, wrapped *twice*,
   once implicitly via whatever produced the flat string, once explicitly
   here), `table_render.go:523` (one cell's flat text, forced `break-word`),
   `block.go:318` (a block's inline content). All five call sites receive a
   plain `string` from `renderInline`/`renderInlineAcc`/`plainInlineText`
   *before* wrapping ever runs. **This means introducing an atomic
   "pre-rendered sub-box, don't split this" token requires restructuring the
   flattening step at all five call sites. It is not a `wordWrapANSI`
   signature change alone.**

4. **`wordWrapANSI` (textutil.go:344-419) is a greedy word-fill over
   `splitANSITokens`'s whitespace-delimited tokens.** Each token is measured
   once via `ansiVisibleLen` and treated as an opaque atomic unit: it is
   either placed whole on the current line, forces a line flush if it doesn't
   fit, or under `break-word` gets hard-split via `splitAtVisualWidthCarry` if
   it's wider than the whole line. The fill/break decision loop only ever
   consults each token's *width*, never its content structure. That is good
   news. Generalizing the token type, by adding a "this is a pre-rendered
   sub-box" case alongside plain text, mostly widens the token type; the fill
   loop itself barely changes.

5. **`table_render.go` flattens cell content early, wraps late, and
   interleaves borders with content in the same pass.** `renderTable`
   collects each cell's inline content into flat `cellText string` before
   column widths are even known → `buildTableColumns`/`sizeColumns`
   determine `widths []int` → only then does `fillTableCellLines` word-wrap
   each cell into `tableCell.lines []string`, already a slice. But
   `renderTableRow` composes columns and draws border glyphs in the *same*
   string-building loop, immediately serialized. There is no intermediate
   "positioned grid" stage even though `lines []string` almost gets there.

6. **`list.go` renders each `<li>` via one `renderInline` call, fully
   flattened, then re-wraps the resulting string a second time** at a
   narrower first-line width to account for the bullet or number prefix, and
   combines prefix and wrapped body via plain string concatenation, never as
   structured `(prefix, body)` data.

7. **`box.go`'s primitives (`parsePaddingLen`, `resolveMarginSide`,
   `splitAutoMargins`, `clampCellPadding`) are pure `int`-in/`int`-out** with
   no string or line awareness at all. They are reusable as-is, with no
   changes needed anywhere in this rewrite.

## New architecture

### Core type

```go
// box is rendered content that hasn't been assigned an absolute position
// yet — position is assigned by whichever caller embeds it into a parent.
type box struct {
    lines []string // one entry per output row, ANSI-styled, no trailing \n
    width int       // visible column width, uniform across lines
}
```

The line-oriented helpers (`applyLineEdges`, `alignLines`, `padLinesToWidth`,
`applyBlockBorders`, `drawBlockHBorder`) change to operate on `box`/`[]string`
directly, dropping their internal Split/Join round-trip. `renderBlockContent`
keeps its *exact current sequence of operations*, see finding #2 above. Only
the type threaded through changes, from `content string` to a `box`.

### Position tracking

A side map, built as boxes are composed. When a parent embeds a child's box
into its own `lines`, it knows the child's local offset: however many lines
are already in the parent's own accumulator, and whatever column offset its
own margin, border, and padding already established. Those are the same
`ml`/`bl.char` width/`pl` values `renderBlockContent` already computes, per
finding #2, just no longer used only to bake spaces into a string. Record
`layout[child.node] = Rect{row: parentLineCountSoFar, col: knownColOffset,
width: child.width, height: len(child.lines)}`, relative to the parent's own
origin. As that parent's own box gets embedded one level up, shift all its
recorded entries by *its* offset in the grandparent, and merge. This
resolves to absolute coordinates once the walk reaches the document root. It
is propagated incrementally, one level at a time, not deferred to some later
pass.

Margin collapse between siblings becomes `max(a.marginBottom, b.marginTop)`,
arithmetic performed once when the parent composes its children, rather than
a runtime buffering trick.

**Rect semantics — border-box, not margin-box.** `Document.Rect(el)` and the
`layout`/position map generally must report the CSS *border box* (content
plus padding plus border), excluding margin. That matches
`getBoundingClientRect()` and is needed for accurate hit-map testing. This
matters because the two margin axes are asymmetric today. Vertical margin
(`margin-top`/`margin-bottom`) is already injected externally by the parent as
blank lines around the child's own returned content, never baked into the
child's own string, so it is already border-box-clean. Horizontal margin
(`margin-left`/`margin-right`) is baked directly into every line of the
child's own returned box via the same `applyLineEdges`-style mechanism used
for padding, just outside the border char. Once baked in, those space
characters are indistinguishable from content unless something remembers how
wide they were.

No new type is needed to fix this. `renderBlockContent` already computes
`ml`/`mr`/`marginTop`/`marginBottom` as local variables at exactly the point
it finalizes the box, per finding #2's confirmed operation order. Whichever
step builds the position map — the token-based `wordWrapTokens`'s `Rect`
output, and `Document.Rect` itself — must inset by those already-known
values: `Rect{row: origin.row + marginTop, col: origin.col + ml, width:
box.width - ml - mr, height: len(box.lines) - marginTop - marginBottom}`,
not the box's full bounds. `renderBlockContent` keeps returning the
margin-inclusive box exactly as it does today, preserving the string-shim's
byte-identical behavior. Only the position-map builder needs to know the
inset.

### Token-based inline wrapping

Generalize the wrap unit from "whitespace-delimited text token"
(`splitANSITokens`'s output today) to a token that can also be an atomic
pre-rendered sub-box:

```go
type wrapToken struct {
    text  string      // set for a plain text token
    box   *box        // set for a nested block-in-inline / inline-block child
    node  *html.Node  // originating node, for position tracking
    width int          // ansiVisibleLen(text), or box.width
}

func wordWrapTokens(tokens []wrapToken, width int, breakMode string, firstLineWidth int) (result box, positions map[*html.Node]Rect)
```

Per finding #4, the fill/break decision loop in `wordWrapANSI` barely
changes, since it already only consults a token's width. What changes is the
**tokenization step upstream**, at all five call sites from finding #3.
`renderInlineAcc` must *collect* `[]wrapToken` as it walks children — text
runs become text tokens, and block-in-inline and inline-block children become
box tokens via their own recursive box-producing render call — instead of
writing into a `cappedWriter` as it goes, then call `wordWrapTokens` once at
the end. `list.go` and the table-cell extraction path need the equivalent
change. One simplification for a box token that's *multiple lines tall*:
force a line break before and after it. There is no flowing text around a
tall embedded object, matching the earlier design discussion's stated scope.

This also resolves the historical worry about block-in-inline elements not
having a knowable position until their surrounding text's wrap is decided.
In the new model, wrapping and position assignment happen in the *same*
step, in the same function call. `wordWrapTokens` decides line breaks and
hands back positions for every token, including box tokens, as one
integrated result rather than something reconstructed afterward.

### Fate of `cappedWriter`

- Margin-collapse buffering (`WriteAtLeastNewlines`) is replaced entirely by
  the integer arithmetic described above. No writer is needed for this.
- `maxBlankLines` capping — the messy/untrusted-HTML feature, unrelated to
  margin collapse, see INTERACTIVE.md's output-model section — becomes a
  single filter pass over the *final* flattened `[]string` of the whole
  document's lines, collapsing runs of blank entries down to `maxBlanks`. It
  is a plain slice filter, not a stateful runtime machine.
- `EnterPre`/`ExitPre`'s job, exempting pre-formatted regions from that
  capping, needs *some* residual tracking. One option is to tag a box's lines
  as "pre" so the final filter skips over that range. **Open question, not
  resolved here** — flagged below.

### Popups / z-order

Once `Rect`s are real rather than reconstructed, compositing a popup is:
render its box separately, then at final serialization splice its `lines`
over the base box's lines at the known row/col range. That is sufficient for
opaque rectangular overlays like a `<select>` dropdown, with no per-cell
blending needed, matching the granularity decision from the prior design
discussion: line-level compositing, not a full per-cell grid.

**Done.** The splicing primitive is `spliceColumns(line string, col, width
int, replacement string) string` (textutil.go, alongside
`splitAtVisualWidthCarry`), not `splitAtVisualWidth`. That function only
chops a string into sequential fixed-width chunks starting from column 0.
This doc's own migration plan corrected that claim before implementation; see
"Corrections to RENDERING.md" in the plan this repo used. Overwriting an
*interior* column range needed a real primitive. It walks the line once,
tracking `ansiCarry` state, to (1) copy the untouched prefix, closing
whatever span is open right before the splice so it doesn't leak into
`replacement`; (2) skip the overwritten `[col, col+width)` region without
emitting it, still tracking carry state through it; (3) reopen whatever
span was active at that point, if any, before the resuming suffix, so a
style span that happened to cover the entire cut region doesn't leave the
suffix orphaned. That span's own eventual reset, past the cut, still closes
it correctly. `replacement` is inserted verbatim; the caller is responsible
for it already being exactly `width` visible columns wide.

**Wired up** (post-migration): `select_popup.go`'s `compositeOpenSelects` and
`compositeSelectPopup` are the composition-level caller. They are invoked
from `Engine.RenderNode` after `capBlankRuns`/`forceHeight`, so they operate
on the exact final `lines`/`positions` about to be emitted. For every
`<select>` currently carrying the reserved `selectOpenAttr` marker — see
`document`'s `toggleSelectOpen`, threaded through the same way `focusAttr` is,
via `Options.SelectOpenAttr`/`Engine.selectOpenAttr` — the popup's own
lines, one per `<option>`, reverse-video wrapped, are spliced beneath the
select's own `Rect` via `spliceColumns`, and a synthetic `Rect` is recorded
for each `<option>` at its spliced row and col. That is what lets
`elementAt`/`DispatchClick` hit-test individual options with no dedicated
hit-testing code of their own. If there isn't already enough room below the
select for every option, the popup either grows the document with extra
blank rows (natural or automatic height) or clips to whatever room remains
(an `Options.Height`-fixed viewport, so as not to exceed the caller's
requested size); see `compositeOpenSelects`'s `canGrow` parameter. When
clipping is forced, option rows are dropped first, then the bottom
border and padding, and the top border and padding last, so a clipped popup
never renders headless.

**CSS-styled popups**: the popup's rows are no longer unconditionally
reverse-video wrapped. `overlay_box.go`'s `resolveOverlayBoxStyle` resolves
the `<select>`'s own cascaded declarations, via `Engine.resolveDecls`, the
same cascade every other element uses, into an `overlayBoxStyle`: border
(`resolveBoxBorders`), padding (`parsePaddingLen`), `margin-left`/`margin-top`
(`resolveMarginSide`/`parseMargin`; `margin-right`/`margin-bottom` have no
effect on a floating overlay with nothing following it to push against),
width (`resolveWidthConstraints`), and a base `inlineStyle`
(`extractInlineStyle`) for background-color and color. `drawOverlayFrame`
splices the border and padding rows around the option rows the same way
`renderBlockContentBox` composes a real block box's border and padding, just
by hand rather than through the full box-tree pipeline. The popup still has
no node of its own in the box tree, so it can't be routed through that
pipeline directly. See this section's opening paragraph: it's still a
line-splice overlay, not a real box.

Each `<option>` row resolves its own `background-color` and `color` via
`resolveOptionRowStyle` in `select_popup.go`, falling back to the
`<select>`'s own base style for whichever of foreground or background it
doesn't set itself. The `▸`-highlighted arrow-key row is exposed to CSS as
`option:hover`, a repurposing of the `:hover` pseudo-class via the same
synthetic-attribute mechanism `:focus` already uses: `cssengine.Cascade`'s
`FocusAttr` gets a sibling `HoverAttr`, wired to `Engine.selectHighlightAttr`
in `render/cascade.go`'s `cascade()`. `option[selected]` needs no engine
change at all, since attribute selectors already match generically. Only
when nothing in that chain — `<select>`, `<option>`, `option:hover` — sets a
color does a row fall back to the historical hardcoded
`"\x1b[7m"`/`"\x1b[27m"` reverse-video wrap. `TestSelectPopupComposition`
pins that this fallback still renders byte-identically to before CSS support
existed.

Adding border and padding shifts each option's synthetic `Rect` by the border
and padding columns and rows. The `Rect` recorded for hit-testing is the
content sub-rectangle only, excluding the border and padding, so a click that
lands on the popup's own chrome rather than an option's text is not
misattributed to that option. Per-option border, padding, and width are not
supported. Every row shares the popup's own content width, matching a real
`<select>`'s option list, which is uniform width regardless of a browser's
own OS-native rendering of individual rows.

`overlay_box.go`'s primitives (`overlayBoxStyle`, `resolveOverlayBoxStyle`,
`drawOverlayFrame`) are deliberately not select-specific. They're written so
a future overlay — a tooltip, a context menu, see `docs/INTERACTIVE.md`'s
"Popups / z-order beyond `<select>`" gap — can resolve and draw its own
bordered, padded box against its own trigger node the same way, with only its
own per-row or per-item content styling staying bespoke.

### `position: relative`

This is the second consumer of this section's splice-based compositing
pattern (`position.go`), and a simpler one than `<select>`'s popup. The
element never leaves normal flow, so no new layout pass is needed at all, and
`renderBlockContentBox`'s layout math is untouched. `Engine.RenderNode`
(engine.go) runs `applyRelativeOffsets` right after `forceHeight` and
`capBlankRuns`, so row indices are final, and right before
`compositeOpenSelects`, so a `position: relative` `<select>` anchors its
popup off its shifted position rather than its original layout slot. For each
element with `position: relative` and a non-zero resolved `top` or `left`
offset, it reads the element's own rectangle out of the composed `lines`
via `visibleWindowCarry` — textutil.go, the exact inverse of
`spliceColumns`, previously used only for `overflow-x: scroll`'s viewport
window — blanks the original rectangle, splices the extracted content back
in at the offset location via `spliceColumns`, and shifts the element's own
and every descendant's recorded `Rect` by the same offset. That last step is
a flat-map subtree walk, not `mergePositions`' merge-two-maps shape, but the
same underlying shift arithmetic. As a result `document.elementAt` and
`DispatchClick` hit the shifted position with no changes needed in the
`document` package at all.

Two things were deliberately out of scope for `relative` itself, and both
landed with the `absolute`/`fixed` work described next: growing the canvas
when a shift has nowhere to land (clamped instead, unchanged for `relative`),
and `z-index` ordering when multiple relatively-positioned elements' shifted
content overlaps.

### `position: absolute` / `position: fixed`

Unlike `relative`, an `absolute` or `fixed` element is removed from normal
flow entirely. It reserves no layout space at all, like `display: none`, and
is positioned against a containing block: the document viewport for `fixed`,
the nearest ancestor with `position` ≠ `static` for `absolute`, falling back
to the viewport if there is none. This needed the two things `relative`
didn't: out-of-flow removal at every layout call site, and a real
containing-block lookup. Both are implemented in
`internal/render/outofflow.go`.

**Static upfront collection.** `collectOutOfFlow` (outofflow.go) walks the
whole DOM once, before `renderRootTokens` runs, finding every element whose
resolved `position` is `absolute` or `fixed`. Results are stored as both a
`map[*html.Node]bool` for fast membership checks and a preorder
`[]*html.Node`, so ancestors always come before descendants. This is
deliberately a static pre-pass rather than incremental discovery during
layout, the way `compositeOpenSelects`'s own walk discovers open `<select>`s.
An out-of-flow element can itself contain further out-of-flow descendants,
and one whole-document walk finds all of them regardless of nesting depth,
with no fixed-point re-walk needed. The preorder property is reused twice
later: for Phase A's dependency order, and Phase B's z-index tiebreak.

**Out-of-flow removal.** This works exactly like `display: none`, but three
call sites needed the check rather than one central gate, mirroring the
existing scattered `display == "none"` checks in the same three files for the
same reason: `render.go`'s `renderRootDisplayTokens`, the central dispatch
for root-level elements; `inline.go`'s `renderInlineAcc` per-child walk, the
recursive counterpart, covering every depth including table-cell content, see
RENDERING.md finding #5; and `flex.go`'s `collectFlexItems`, since a flex
container's direct children are collected via a separate path that bypasses
`renderInlineAcc`'s loop entirely. **Accepted, documented gap**: `absolute`
or `fixed` set directly on a `<tr>`, `<td>`, `<th>`, or `<li>` itself is not
removed from flow, because list.go's own item loop and table_render.go's
row/cell-grid walk bypass both gates above. Content *nested inside* one still
is, via the `inline.go` gate.

**Phase A — geometry and render, dependency order.** `applyOutOfFlow`
processes `e.outOfFlowOrder` shallowest-first, guaranteeing any out-of-flow
containing-block ancestor is resolved before a descendant that depends on
it. For each node it does the following.

Resolve the containing block Rect: the whole document for `fixed`;
`nearestPositionedAncestor`'s result for `absolute`, looked up in either the
normal `positions` map or this phase's own running `resolvedRects`, so an
out-of-flow ancestor already processed earlier in this same pass works too,
with the whole document as fallback.

Render the element's own box via `renderBlockContentBox(n, decls,
cbRect.Width)`. That is the same generic "render an arbitrary block-display
node's box in isolation" function already called from three unrelated sites
(render.go, flex.go, inline.go), reused here unchanged, since the spec
blockifies an out-of-flow element regardless of its own `display` anyway.

Run `applyRelativeOffsets` on that box's own local `lines` and
`subPositions`, not just once at the top level. This element's whole subtree
was skipped during normal layout, see "Out-of-flow removal" above, so a
`position: relative` descendant nested inside it would otherwise never appear
in any `positions` map `applyRelativeOffsets` gets to see.

Resolve `top`/`right`/`bottom`/`left` via the new `resolveAbsoluteAxis`
against the *real* containing block dimensions. This is a strictly better
result than `relative`'s document-wide percentage approximation, now that a
real containing block Rect exists.

Then clamp the result via `clampOutOfFlowRect` — Col to `[0,
e.width-box.width]`, Row to `>= 0`, and, only when `canGrow` is false, Height
clipped so it fits the document's current line count — **before** recording
it in `resolvedRects`. Clamping here in Phase A, rather than deferring it to
Phase B's paint step, matters specifically for a descendant that resolves
this element as its own containing block. It must see the *same* bounds this
element actually paints at, not a pre-clamp position it may never occupy. An
earlier version of this code clamped only in Phase B, which let a nested
out-of-flow descendant of a clamped ancestor compute its own position against
stale, never-painted coordinates, visibly detached from where its container
actually ended up.

**Shrink-to-fit and centering** landed later, in `renderOutOfFlowBox`. With no
declared `width`, and not both offsets on the inline axis set, the box is
measured at its own content width via flex.go's existing `measureNaturalWidth`
and then rendered for real at that width. Two passes rather than one recursive
shrink-to-fit render, so blocks *inside* the box still fill the result instead
of each shrinking independently, which is what CSS's own shrink-to-fit
specifies. On top of that, auto margins on both sides of an axis, with no
offset set, center the box (`resolveAbsoluteAxis`). `renderBlockContentBox`
splits auto margins itself for an explicit-width box and bakes them into its
lines, so `renderOutOfFlowBox` drops them before rendering; otherwise the box
would be offset twice, and its recorded `Rect` would span the whole containing
block rather than the box actually painted. This is what a modal `<dialog>`
rides on: it is an ordinary `position: fixed` box, given that position by the
UA stylesheet's own `dialog:modal` rule, with no compositing code of its own.
See `docs/DIALOG.md`.

**Deliberate simplifications, not oversights**: with neither `top`/`bottom` nor
`left`/`right` nor an auto-margin pair set on an axis, the element defaults to
the containing block's top-left corner, not spec's "static position", roughly
where the element would have flowed. With *both* offsets on an axis set, it
stretches to the containing block rather than between those two edges.

**Phase B — paint, z-index order.** The Phase A results, each already
carrying its final clamped `Rect`, are sorted by `(zIndex, original preorder
position)` via `sort.SliceStable`. `zIndex` is parsed by `parseZIndex`,
mirroring flex.go's existing `parseOrder` exactly. Since the input is already
in that same document order, the stable sort's tie-break falls out for free.
Each result is then spliced onto `lines` via the same `spliceColumns`
painter's-algorithm compositing that `relative` and `<select>`'s popup
already use: full overwrite, no partial blending, later z-index wins. The
only bounds-handling left at this point is growing `lines` with blank rows
when Phase A's `canGrow` left Height unclipped. `positions` is updated via
the existing `mergePositions` primitive for the box's own descendants, plus a
direct entry for the element itself. No new position-map machinery is needed.

**Ordering in `Engine.RenderNode`** (engine.go):
`capBlankRuns → forceHeight → applyRelativeOffsets → applyOutOfFlow →
compositeOpenSelects`. `applyRelativeOffsets` runs first so an out-of-flow
element's containing block, if it's a shifted `position: relative`
ancestor, reflects the *shifted* Rect. `applyOutOfFlow` runs before
`compositeOpenSelects` so an out-of-flow `<select>`, which has no Rect
from normal layout having been skipped there, gets one before the popup
pass looks it up.

### Public API

`Renderer.Render(htmlStr string) (string, error)` keeps its exact signature.
Internally: parse → compute the root `box` → run the blank-line-capping
filter → `strings.Join(root.lines, "\n")`. No consumer of the package sees
any difference.

## Migration plan

Ordered so the existing test suite (htmlterm_test.go, table_test.go,
list_test.go, selectors_test.go, layout_test.go, text_test.go,
writer_test.go) can verify *byte-identical* output after each step. If the
new engine's serialized string matches the old engine's for every existing
case, that step preserved behavior.

1. Introduce the `box` type. `box.go`'s primitives don't change (finding #7).
2. Migrate the leaf helpers (`applyLineEdges`, `alignLines`,
   `padLinesToWidth`, `applyBlockBorders`, `drawBlockHBorder`) to operate on
   `[]string`/`box` directly, dropping their internal Split/Join. Low risk,
   mechanical, independently testable. Their string-in/string-out
   observable behavior is unchanged if kept behind a thin compatibility
   shim during this step.
3. Migrate `renderBlockContent` to build and return a `box`, keeping its
   exact current operation order (finding #2). Add a thin wrapper that
   joins `box.lines` back to a string so every existing caller keeps
   compiling and passing unchanged. This lets the rest of the migration
   proceed file-by-file, not as a big-bang cutover.
4. Migrate `renderInlineAcc` to the token-collection design (finding #3/#4
   generalization). This is the highest-risk, most invasive step. Do it
   last among the "core" changes, once 2-3 are solid, and add targeted new
   tests first for: a block-in-inline child mid-paragraph, multiple
   inline-block siblings sharing one output line, and a token wider than
   the available width forcing a hard break.
5. Migrate `table_render.go` and `list.go` to consume and produce
   `box`/`[]wrapToken` instead of ad hoc flatten-then-rewrap (findings #5,
   #6). These should now be adaptations of the primitives from steps 2-4,
   not fresh design.
6. Retire `cappedWriter`'s margin-buffering responsibility; reimplement
   `maxBlankLines` capping as the final-pass filter described above.
7. **Done.** Once fully passing the existing suite unchanged: revisit
   INTERACTIVE.md's Phase 2. Replace the OSC-marker design with
   `Document.Rect(el)` simply reading the position map that composition
   now produces as a natural byproduct. No synthetic markers, no post-hoc
   string scanning. This is substantially simpler than the marker
   design and supersedes it outright rather than merely coexisting with it.

   Implemented via `wrapToken.subPositions` (wraptoken.go) carrying each
   embedded box's own descendant positions, shifted and merged by whichever
   `wordWrapTokens` call ultimately places it (wraptoken.go), with
   `renderBlockContentBox` (block.go) computing its own row/col shift from
   its already-known pt/pl/border-width/ml locals, and `renderTree`
   (htmlterm.go) resolving the final absolute map at the document root.
   That is exactly the "propagated incrementally, one level at a time"
   design this section describes. `Document.Rect` (document.go) refreshes
   the map on every `Render()` call.

   One prerequisite surfaced during implementation that this plan didn't
   anticipate. inline.go's and render.go's "default" dispatch cases for
   plain inline elements such as `<span>` and `<label>` previously flattened
   a child's rendered content to a *string* before splicing it in, which
   silently discarded any box-token identity, and thus position, for a
   trackable descendant nested inside a plain inline wrapper. That is the
   single most common form-control pattern: `<label>Name: <input></label>`.
   Both were changed to splice the child's own `[]wrapToken` directly
   instead of flattening, which is also closer to findings #3/#4's original
   token-splicing intent than the flatten-then-rebox approach it replaced.
   `<a>` and `display:inline-block` children stay string-based, because of
   whole-string OSC8 wrapping and inline-block's own atomic-unit contract.
   That leaves an accepted, documented gap for a further trackable element
   nested inside either of those specifically, since that combination
   essentially never occurs in practice for form controls.

   Also fixed along the way: render.go's root-level `inline-block` case
   only boxed content when it happened to contain a literal `"\n"`,
   unlike inline.go's nested case. That meant a single-line root-level
   inline-block element, such as a bare `<button>` directly under `<body>`,
   silently produced a plain text token with no trackable position at all.
   It is now boxed unconditionally, matching inline.go.

   Margin is **not** inset out of a tracked element's own `Rect`. This is a
   deliberate, documented simplification, not an oversight: horizontal
   margin is baked into a box's lines the same way padding is, and
   unbaking it would require tracking shifts through several more
   per-element transformations, including `text-align:right/center`'s
   variable per-line padding and `text-indent`'s row-0-only shift, for a
   case the primary motivating use doesn't need. That use is hit-testing
   form controls, which essentially never set margin. See wraptoken.go's
   `Rect` doc comment.

## Explicit non-goals / open questions for the implementer

- **Grapheme clusters**: still a live gap, not addressed by this rewrite.
  East-Asian double-width characters *are* handled correctly
  (`runeVisualWidth`/`ansiVisibleLen`, textutil.go, call `go-runewidth` per
  rune), so wide characters alone are done. What's missing is
  multi-codepoint grapheme-cluster segmentation: sequences like emoji ZWJ
  families or combining diacritics are still measured and split rune-by-rune,
  not cluster-by-cluster. Confirmed symptom: `document.go`'s `Backspace`
  handler (`utf8.DecodeLastRuneInString`) deletes one codepoint at a time, so
  backspacing a multi-rune emoji corrupts it instead of removing the whole
  glyph. Fixing this needs a grapheme-segmentation library such as
  `rivo/uniseg` threaded through every width and split call site
  (`ansiVisibleLen`, `splitAtVisualWidthCarry`, `Backspace`, and others). It
  is a scoped feature in its own right, not a byproduct of this rewrite.
- **`maxBlankLines` + pre-formatted region interaction**: flagged above as
  unresolved. It needs a concrete tagging mechanism for the final filter to
  skip `<pre>` ranges, not designed in detail here.
- **Memory**: each `box` holds a `[]string`, roughly comparable to today's
  approach. Not expected to be meaningfully worse, but not benchmarked.
- **Non-uniform per-line width**: `box.width` assumes uniform width across
  lines, matching what today's helpers already enforce via padding in
  practice. Not expected to be a real constraint, but not proven for every
  existing CSS feature combination.
