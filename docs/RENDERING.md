# Rendering Architecture V2: Positioned Boxes

## Status

**Implemented in full** — see the migration plan below, all 7 steps done, and
docs/INTERACTIVE.md's own "Status" line ("RENDERING.md landed (all 9 steps)").
`box` (box.go), `wrapToken`/`wordWrapTokens` (wraptoken.go), and the
`select_popup.go` popup-compositing mechanism described below all exist in
the code as designed; `cappedWriter` is gone, referenced only in comments
explaining what it was replaced by. docs/INTERACTIVE.md's Phase 2
("hit-test map via zero-width markers") was superseded by this document as
planned — see "Migration plan" step 7 below for what actually shipped instead
(`Document.Rect(el)` reading the position map composition produces directly).
The rest of this document is kept as design history/rationale, not a live
plan — read it for the "why", not as a to-do list.

## Motivation

Three separate mechanisms exist today purely because position/size aren't
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
collapse into one clean mechanism instead of three different ones. It also
makes something the current model cannot express *at all*, regardless of
cleverness: overlapping content (a `<select>` dropdown, a tooltip) — a linear
string has no way to represent two things occupying the same screen area.

## Current architecture — precise findings

These are the concrete seams the rewrite exploits, confirmed by reading the
code (not paraphrased from memory):

1. **The "lines as a slice" representation already exists, it's just
   ephemeral.** `applyLineEdges`, `alignLines`, `padLinesToWidth`, and
   `applyBlockBorders` (all block.go) each independently do
   `strings.Split(content, "\n")` → operate per-line → `strings.Join(...)`.
   `renderBlockContent` itself does the same ad hoc for height-padding and
   text-indent. Every one of these reconstructs and re-flattens a `[]string`
   on every call. The rewrite's core mechanical move is: stop
   reconstructing it repeatedly, thread one shared type through instead.

2. **`renderBlockContent` (block.go:218-453) already distinguishes
   content-box from border-box, just not by type.** Order of operations,
   confirmed from the full body: border-chars/margin/padding parsed →
   `clampCellPadding` (box.go) resolves final `pl`, `pr`, `innerW` →
   `renderInlineAcc` produces raw content → word-wrap
   (`wordWrapANSI(rawContent, innerW, breakMode)`) → overflow/text-overflow →
   text-align → **height/min-height/max-height padding applied to `content`
   — this stabilizes line count** → text-indent → padding baked in via
   `applyLineEdges` → left/right borders baked in via `applyBlockBorders` →
   **top/bottom border-rule lines prepended/appended after that** → margins
   baked in via `applyLineEdges` again. Border-box height (rule lines) is
   deliberately added *after* the content-box height is fixed — real CSS box
   model semantics, already implicit in the code's ordering.

3. **Every current word-wrap call site operates on already-fully-flattened
   text**, not on any kind of structured token stream:
   `render.go:29` (root-level transparent content), `list.go:136` and
   `list.go:149` (an `<li>`'s first/subsequent lines, wrapped *twice* — once
   implicitly via whatever produced the flat string, once explicitly here),
   `table_render.go:523` (one cell's flat text, forced `break-word`),
   `block.go:318` (a block's inline content). All five call sites receive a
   plain `string` from `renderInline`/`renderInlineAcc`/`plainInlineText`
   *before* wrapping ever runs. **This means introducing an atomic
   "pre-rendered sub-box, don't split this" token requires restructuring the
   flattening step at all five call sites — it is not a `wordWrapANSI`
   signature change alone.**

4. **`wordWrapANSI` (textutil.go:344-419) is a greedy word-fill over
   `splitANSITokens`'s whitespace-delimited tokens**, each measured once via
   `ansiVisibleLen` and treated as an opaque atomic unit (a token is either
   placed whole on the current line, forces a line flush if it doesn't fit,
   or — under `break-word` — gets hard-split via `splitAtVisualWidthCarry` if
   it's wider than the whole line). The fill/break decision loop only ever
   consults each token's *width*, never its content structure — good news:
   generalizing the token type (adding an "this is a pre-rendered sub-box"
   case alongside plain text) mostly widens the token type, the fill loop
   itself barely changes.

5. **`table_render.go` flattens cell content early, wraps late, and
   interleaves borders with content in the same pass.** `renderTable`
   collects each cell's inline content into flat `cellText string` (before
   column widths are even known) → `buildTableColumns`/`sizeColumns`
   determine `widths []int` → only then does `fillTableCellLines` word-wrap
   each cell into `tableCell.lines []string` (already a slice) → but
   `renderTableRow` composes columns and draws border glyphs in the *same*
   string-building loop, immediately serialized — there is no intermediate
   "positioned grid" stage even though `lines []string` almost gets there.

6. **`list.go` renders each `<li>` via one `renderInline` call (fully
   flattened), then re-wraps the resulting string a second time** (at a
   narrower first-line width to account for the bullet/number prefix), and
   combines prefix + wrapped body via plain string concatenation — never as
   structured `(prefix, body)` data.

7. **`box.go`'s primitives (`parsePaddingLen`, `resolveMarginSide`,
   `splitAutoMargins`, `clampCellPadding`) are pure `int`-in/`int`-out** with
   no string/line awareness at all — fully reusable as-is, no changes needed
   anywhere in this rewrite.

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
keeps its *exact current sequence of operations* (see finding #2 above) —
only the type threaded through changes, from `content string` to a `box`.

### Position tracking

A side map, built as boxes are composed: when a parent embeds a child's box
into its own `lines`, it knows the child's local offset (however many lines
are already in the parent's own accumulator, and whatever column offset its
own margin/border/padding already established — the same `ml`/`bl.char`
width/`pl` values `renderBlockContent` already computes, per finding #2,
just no longer only used to bake spaces into a string). Record
`layout[child.node] = Rect{row: parentLineCountSoFar, col: knownColOffset,
width: child.width, height: len(child.lines)}`, relative to the parent's own
origin. As that parent's own box gets embedded one level up, shift all its
recorded entries by *its* offset in the grandparent, and merge. This
resolves to absolute coordinates once the walk reaches the document root —
propagated incrementally, one level at a time, not deferred to some later
pass.

Margin collapse between siblings becomes `max(a.marginBottom, b.marginTop)`
— arithmetic performed once when the parent composes its children — not a
runtime buffering trick.

**Rect semantics — border-box, not margin-box.** `Document.Rect(el)` (and the
`layout`/position map generally) must report the CSS *border box* (content +
padding + border), excluding margin — matching `getBoundingClientRect()`, and
needed for accurate hit-map testing. This matters because the two margin axes
are asymmetric today: vertical margin (`margin-top`/`margin-bottom`) is
already injected externally by the parent as blank lines around the child's
own returned content, never baked into the child's own string — already
border-box-clean. Horizontal margin (`margin-left`/`margin-right`) is baked
directly into every line of the child's own returned box via the same
`applyLineEdges`-style mechanism used for padding, just outside the border
char — once baked in, those space characters are indistinguishable from
content unless something remembers how wide they were.

No new type is needed to fix this: `renderBlockContent` already computes
`ml`/`mr`/`marginTop`/`marginBottom` as local variables at exactly the point
it finalizes the box (finding #2's confirmed operation order). Whichever step
builds the position map (the token-based `wordWrapTokens`'s `Rect` output,
and `Document.Rect` itself) must inset by those already-known values:
`Rect{row: origin.row + marginTop, col: origin.col + ml, width: box.width -
ml - mr, height: len(box.lines) - marginTop - marginBottom}` — not the box's
full bounds. `renderBlockContent` keeps returning the margin-inclusive box
exactly as it does today (preserving the string-shim's byte-identical
behavior); only the position-map builder needs to know the inset.

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
changes — it already only consults a token's width. What changes is the
**tokenization step upstream**, at all five call sites from finding #3:
`renderInlineAcc` must *collect* `[]wrapToken` as it walks children (text
runs become text tokens; block-in-inline and inline-block children become
box tokens via their own recursive box-producing render call) instead of
writing into a `cappedWriter` as it goes, then call `wordWrapTokens` once at
the end. `list.go` and the table-cell extraction path need the equivalent
change. Simplification for a box token that's *multiple lines tall*: force
a line break before and after it — no "flowing text around a tall embedded
object," matching the earlier design discussion's stated scope.

This also resolves the historical worry about block-in-inline elements not
having a knowable position until their surrounding text's wrap is decided:
in the new model, wrapping and position assignment happen in the *same*
step, in the same function call — `wordWrapTokens` decides line breaks and
hands back positions for every token (including box tokens) as one
integrated result, not something reconstructed afterward.

### Fate of `cappedWriter`

- Margin-collapse buffering (`WriteAtLeastNewlines`) is replaced entirely by
  the integer arithmetic described above — no writer needed for this.
- `maxBlankLines` capping (the messy/untrusted-HTML feature, unrelated to
  margin collapse — see INTERACTIVE.md's output-model section) becomes a
  single filter pass over the *final* flattened `[]string` of the whole
  document's lines, collapsing runs of blank entries down to `maxBlanks` —
  a plain slice filter, not a stateful runtime machine.
- `EnterPre`/`ExitPre`'s job (exempt pre-formatted regions from that
  capping) needs *some* residual tracking — e.g. tag a box's lines as
  "pre" so the final filter skips over that range. **Open question, not
  resolved here** — flagged below.

### Popups / z-order

Once `Rect`s are real (not reconstructed), compositing a popup is: render
its box separately, then at final serialization splice its `lines` over the
base box's lines at the known row/col range. Sufficient for opaque
rectangular overlays like a `<select>` dropdown — no per-cell blending
needed, matching the granularity decision from the prior design discussion
(line-level compositing, not a full per-cell grid).

**Done** — the splicing primitive is `spliceColumns(line string, col, width
int, replacement string) string` (textutil.go, alongside
`splitAtVisualWidthCarry`), not `splitAtVisualWidth`: that function only
chops a string into sequential fixed-width chunks starting from column 0
(this doc's own migration plan corrected that claim before implementation —
see "Corrections to RENDERING.md" in the plan this repo used). Overwriting
an *interior* column range needed a real primitive: it walks the line once,
tracking `ansiCarry` state, to (1) copy the untouched prefix, closing
whatever span is open right before the splice so it doesn't leak into
`replacement`; (2) skip the overwritten `[col, col+width)` region without
emitting it, still tracking carry state through it; (3) reopen whatever
span was active at that point (if any) before the resuming suffix, so a
style span that happened to cover the entire cut region doesn't leave the
suffix orphaned (its own eventual reset, past the cut, still closes it
correctly). `replacement` is inserted verbatim — the caller is responsible
for it already being exactly `width` visible columns wide.

**Wired up** (post-migration): `select_popup.go`'s `compositeOpenSelects`/
`compositeSelectPopup` are the composition-level caller — invoked from
`Engine.RenderNode` after `capBlankRuns`/`forceHeight`, so they operate on
the exact final `lines`/`positions` about to be emitted. For every
`<select>` currently carrying the reserved `selectOpenAttr` marker (see
`document`'s `toggleSelectOpen`; threaded through the same way `focusAttr`
is — `Options.SelectOpenAttr`/`Engine.selectOpenAttr`), the popup's own
lines (one per `<option>`, reverse-video wrapped) are spliced beneath the
select's own `Rect` via `spliceColumns`, and a synthetic `Rect` is recorded
for each `<option>` at its spliced row/col — which is what lets
`elementAt`/`DispatchClick` hit-test individual options with no dedicated
hit-testing code of their own. If there isn't already enough room below the
select for every option, the popup either grows the document with extra
blank rows (natural/automatic height) or clips to whatever room remains
(an `Options.Height`-fixed viewport, so as not to exceed the caller's
requested size) — see `compositeOpenSelects`'s `canGrow` parameter.

### Public API

`Renderer.Render(htmlStr string) (string, error)` keeps its exact signature.
Internally: parse → compute the root `box` → run the blank-line-capping
filter → `strings.Join(root.lines, "\n")`. No consumer of the package sees
any difference.

## Migration plan

Ordered so the existing test suite (htmlterm_test.go, table_test.go,
list_test.go, selectors_test.go, layout_test.go, text_test.go,
writer_test.go) can verify *byte-identical* output after each step — if the
new engine's serialized string matches the old engine's for every existing
case, that step preserved behavior.

1. Introduce the `box` type. `box.go`'s primitives don't change (finding #7).
2. Migrate the leaf helpers (`applyLineEdges`, `alignLines`,
   `padLinesToWidth`, `applyBlockBorders`, `drawBlockHBorder`) to operate on
   `[]string`/`box` directly, dropping their internal Split/Join. Low risk,
   mechanical, independently testable — their string-in/string-out
   observable behavior is unchanged if kept behind a thin compatibility
   shim during this step.
3. Migrate `renderBlockContent` to build and return a `box`, keeping its
   exact current operation order (finding #2). Add a thin wrapper that
   joins `box.lines` back to a string so every existing caller keeps
   compiling and passing unchanged — this lets the rest of the migration
   proceed file-by-file, not as a big-bang cutover.
4. Migrate `renderInlineAcc` to the token-collection design (finding #3/#4
   generalization). This is the highest-risk, most invasive step — do it
   last among the "core" changes, once 2-3 are solid, and add targeted new
   tests first for: a block-in-inline child mid-paragraph, multiple
   inline-block siblings sharing one output line, and a token wider than
   the available width forcing a hard break.
5. Migrate `table_render.go` and `list.go` to consume/produce
   `box`/`[]wrapToken` instead of ad hoc flatten-then-rewrap (findings #5,
   #6) — should now be adaptations of the primitives from steps 2-4, not
   fresh design.
6. Retire `cappedWriter`'s margin-buffering responsibility; reimplement
   `maxBlankLines` capping as the final-pass filter described above.
7. **Done.** Once fully passing the existing suite unchanged: revisit
   INTERACTIVE.md's Phase 2. Replace the OSC-marker design with
   `Document.Rect(el)` simply reading the position map that composition
   now produces as a natural byproduct — no synthetic markers, no post-hoc
   string scanning. This should be substantially simpler than the marker
   design and supersedes it outright, not merely coexists with it.

   Implemented via `wrapToken.subPositions` (wraptoken.go) carrying each
   embedded box's own descendant positions, shifted and merged by whichever
   `wordWrapTokens` call ultimately places it (wraptoken.go), with
   `renderBlockContentBox` (block.go) computing its own row/col shift from
   its already-known pt/pl/border-width/ml locals, and `renderTree`
   (htmlterm.go) resolving the final absolute map at the document root —
   exactly the "propagated incrementally, one level at a time" design
   this section describes. `Document.Rect` (document.go) refreshes the map
   on every `Render()` call.

   One prerequisite surfaced during implementation that this plan didn't
   anticipate: inline.go's and render.go's "default" (plain inline, e.g.
   `<span>`/`<label>`) dispatch cases previously flattened a child's
   rendered content to a *string* before splicing it in — which silently
   discarded any box-token identity (and thus position) for a trackable
   descendant nested inside a plain inline wrapper, the single most common
   form-control pattern (`<label>Name: <input></label>`). Both were changed
   to splice the child's own `[]wrapToken` directly instead of flattening,
   which is also closer to findings #3/#4's original token-splicing intent
   than the flatten-then-rebox approach it replaced. `<a>` and
   `display:inline-block` children stay string-based (whole-string OSC8
   wrapping, and inline-block's own atomic-unit contract) — an accepted,
   documented gap for a further trackable element nested inside either of
   those specifically, since that combination essentially never occurs in
   practice for form controls.

   Also fixed along the way: render.go's root-level `inline-block` case
   only boxed content when it happened to contain a literal `"\n"`,
   unlike inline.go's nested case — meaning a single-line root-level
   inline-block element (e.g. a bare `<button>` directly under `<body>`)
   silently produced a plain text token with no trackable position at all.
   Now boxed unconditionally, matching inline.go.

   Margin is **not** inset out of a tracked element's own `Rect` (a
   deliberate, documented simplification, not an oversight): horizontal
   margin is baked into a box's lines the same way padding is, and
   unbaking it would require tracking shifts through several more
   per-element transformations (`text-align:right/center`'s variable
   per-line padding, `text-indent`'s row-0-only shift) for a case the
   primary motivating use — hit-testing form controls, which essentially
   never set margin — doesn't need. See wraptoken.go's `Rect` doc comment.

## Explicit non-goals / open questions for the implementer

- **Grapheme clusters**: still not addressed, and this is a real, live gap
  — not addressed by this rewrite. East-Asian double-width characters
  *are* handled correctly (`runeVisualWidth`/`ansiVisibleLen`, textutil.go,
  call `go-runewidth` per rune), so "wide characters" alone is done. What's
  missing is multi-codepoint grapheme-cluster segmentation: sequences like
  emoji ZWJ families or combining diacritics are still measured and split
  rune-by-rune, not cluster-by-cluster. Confirmed symptom: `document.go`'s
  `Backspace` handler (`utf8.DecodeLastRuneInString`) deletes one codepoint
  at a time, so backspacing a multi-rune emoji corrupts it instead of
  removing the whole glyph. Fixing this needs a grapheme-segmentation
  library (e.g. `rivo/uniseg`) threaded through every width/split call site
  (`ansiVisibleLen`, `splitAtVisualWidthCarry`, `Backspace`, etc.) — a
  scoped feature in its own right, not a byproduct of this rewrite.
- **`maxBlankLines` + pre-formatted region interaction**: flagged above as
  unresolved — needs a concrete tagging mechanism for the final filter to
  skip `<pre>` ranges, not designed in detail here.
- **Memory**: each `box` holds a `[]string`, roughly comparable to today's
  approach; not expected to be meaningfully worse, but not benchmarked.
- **Non-uniform per-line width**: `box.width` assumes uniform width across
  lines, matching what today's helpers already enforce via padding in
  practice. Not expected to be a real constraint, but not proven for every
  existing CSS feature combination.
