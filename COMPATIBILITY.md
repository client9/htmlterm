# htmlterm Compatibility Notes

htmlterm reinterprets three separate web-platform surfaces for a fixed-size
character grid instead of a browser: **HTML** (parsing and per-element
rendering), **CSS** (selectors, cascade, properties), and **DOM & Events**
(the mutable `Document`/`Element` API and its native-Go event model).

This document is about the *gap* between htmlterm and the real spec, not a
feature list. For what's supported, see **[CSS.md](./CSS.md)**
(the exhaustive CSS/selector/element reference) and
**[docs/DOM_API.md](./docs/DOM_API.md)** (a method-by-method table against
the real `Document`/`Element`/`Node` interfaces). Duplicating "what's
supported" here as prose would just drift out of sync with those as features
land, so each surface below gets three sections instead:

- **Deviations from Spec.** Real features that exist here but behave
  differently than in a browser, and why (text cells, not pixels; no
  scripting engine; no real pointer/window).
- **Terminal-Native Additions.** Things invented for this renderer with no
  browser equivalent at all.
- **Not Supported.** Real features that simply aren't implemented.

For the design rationale behind the DOM/Events/rendering internals, see
`docs/INTERACTIVE.md`, `docs/RENDERING.md`, `docs/REPAINT.md`, and
`docs/SCROLLING.md`.

---

## HTML

### Deviations from Spec

- **Form controls are attribute-driven, not stateful.** `<input>`'s
  `value`/`checked`/`type` attributes (and `<select>`'s selected `<option>`)
  are the only source of truth. `Element.Value()`, `SetValue()`,
  `Checked()`, and `SetChecked()` read and write those same attributes
  directly. There's no separate "live DOM property vs. reflected HTML
  attribute"
  distinction the way real browsers maintain for form controls. The one
  place spec's own setter/attribute split does survive is newline
  sanitization: `SetValue()` and `DispatchPaste`'s default action strip
  CR/LF from a single-line control's value (every text entry but
  `<textarea>`), exactly like a real value setter, while
  `SetAttribute("value", ...)` writes whatever it's given. A literal
  `"\n"` in an inline `<input>`'s value *does* render as a real line break
  here, unlike a browser, which ignores it.
- **Text-field selection is a single flat range, not real DOM's
  `Selection`/`Range` model.** `Element.SelectionStart()`/`SelectionEnd()`/
  `SelectionDirection()`/`SetSelectionRange()` mirror
  `HTMLInputElement`/`HTMLTextAreaElement`'s own selection API (rune offsets,
  not UTF-16 code units), and `DispatchKey`'s
  ArrowLeft/Right/Home/End/Backspace/Delete/Ctrl+A default actions, plus
  `DispatchClick`'s click-to-position-caret default action (Shift extends),
  all operate on it. But there's no `window.getSelection()` or `Range`
  object, no cross-element selection, and no `contenteditable` selection at
  all (see "`contenteditable`" below). See `docs/proposals/CARET_SELECTION.md`.

  Those offsets are **runes, not UTF-16 code units and not terminal
  columns**. The first half is the deviation from spec (real DOM counts
  UTF-16 code units, so an emoji costs 2 there and 1 here). The second half
  is not a deviation but a distinction worth stating, because this renderer
  makes it tempting to conflate them: a caret at offset 3 in `"日本語ab"`
  sits at screen column 6, and only the cursor-placement and click
  hit-testing paths ever convert between the two
  (`internal/textcell`'s `ColumnForRuneIndex`/`RuneIndexForColumn`).
  `SelectionStart`/`SelectionEnd` always answer in runes, whether or not the
  document has ever been rendered.
- **A `<textarea>`'s caret/selection placement doesn't account for
  word-wrapping.** Both the terminal cursor (`focusCursorPos`) and the
  `::selection` highlight locate a rune offset by its `"\n"`-delimited
  *logical* line, not by which *wrapped* row the renderer actually placed
  it on. A caret inside a long line that word-wraps onto a second screen
  row will render one logical line "early". Single-line `<input>` and short
  or un-wrapped `<textarea>` content are unaffected.
- **`<noscript>` content always renders.** There's no scripting engine to
  disable it for, so (unlike a browser, which only shows `noscript` content
  when JavaScript is off) it's unconditionally treated as regular markup.
- **`<details>`/`<summary>` always render fully expanded.** No collapse/
  expand interactivity exists; a real browser's native disclosure widget
  has no terminal equivalent here.
- **An indeterminate `<progress>` (no `value` attribute) renders statically,
  not animated.** Real browsers show an animated barber-pole; this renderer
  has no `animation` support at all, so `:indeterminate` instead shows
  `::progress-bar`'s track glyph repeated across the full width, with no
  fill. See `docs/PROGRESS_METER.md`.

### Terminal-Native Additions

- **`img::before { content: attr(alt) }`**, a UA stylesheet default. Since no
  image can ever render, alt text is shown inline directly, rather than in a
  broken-image icon's box the way a browser shows it.
- **`abbr[title]::after`** appends the title attribute's expansion inline,
  there being no hover-tooltip concept in a terminal to show it in instead.
- **`<hr>`** renders as a text rule line via `border-top`, not a pixel-drawn
  line.
- **`progress-style`/`meter-style` and `::progress-bar`/`::progress-value`/
  `::meter-bar`/`::meter-optimum-value`/`::meter-suboptimum-value`/
  `::meter-even-less-good-value`.** Real CSS has no author-facing hook to
  restyle `<progress>`/`<meter>`'s bar glyphs at all (WebKit's own
  `-webkit-`-prefixed pseudo-elements this borrows names from are
  non-standard); `progress-style`/`meter-style` reuse `scrollbar-style`'s
  exact five preset names and merge mechanism. See `docs/PROGRESS_METER.md`.

### Not Supported

- **`<script>`, `<canvas>`, `<video>`, `<audio>`, `<iframe>`, `<embed>`,
  `<object>`, `<svg>`.** No scripting, media playback, embedding, or
  vector-graphics rendering exists. `<script>`/`<meta>`/`<link>` content is
  explicitly skipped during rendering; other unhandled elements aren't
  specifically stripped, they just fall back to generic inline treatment of
  whatever text content they happen to contain (usually none).
- **The legacy `width` HTML attribute** on table cells and columns. It is
  ignored in favor of CSS `width`: in real-world markup, especially HTML
  email, it's almost always a pixel value with no reliable pixel-to-column
  conversion.
- **`<dialog>`/`<datalist>`.** No native-modal or autocomplete-popup
  equivalent exists; unhandled the same way any other unrecognized element
  is (generic inline fallback, usually rendering no visible content).
- **`contenteditable`.** The attribute isn't read anywhere; there's no
  editable-content model outside a real `<input>`/`<textarea>` (see "Form
  controls are attribute-driven" above).

---

## CSS

### Deviations from Spec

- **All sizing is in character cells/lines, not pixels.** `width`/`height`/
  `margin`/`padding`/border thickness are all integer counts (or `ch`/`%`).
  `px`, `em`, `rem`, `vw`, `vh` are parsed as unsupported and ignored
  entirely, not converted.
- **An empty box keeps one line when nothing is drawn around it.** A box with
  no content has zero content height, and that is honored where it shows: an
  empty bordered box's two rules meet, `<hr>` is a single rule, and vertical
  padding rows are all that's left of a padded empty box, all matching CSS.
  But a box with no rules and no vertical padding still renders one blank line
  rather than collapsing to nothing, because that line is how this renderer
  separates one block from the next: a zero-line box would run its siblings
  together. `<textarea>` is exempt for a different reason: it's a field to
  type into, sized in HTML by `rows` (unimplemented here), so an empty one
  keeps a row to put the caret in.
- **`opacity`** can't alpha-composite against an unknown terminal
  background, so fractional values darken the foreground/background color
  channels toward black instead; `opacity: 0` blanks content to spaces
  (still occupying its layout box, matching spec) rather than true
  transparency.
- **`border-width`/`border-*-width`** parse without error but are always a
  no-op, since a box-drawing character has no notion of line thickness separate
  from the glyph itself. Use `border-style: heavy` or a custom glyph
  instead.
- **`border-style` values don't match real CSS's keyword set.** Real CSS
  has `solid`/`dashed`/`dotted`/`double`/`groove`/`ridge`/`inset`/`outset`/
  `none`/`hidden`. htmlterm's are named ASCII-art presets instead:
  `solid`/`rounded`/`heavy`/`double`/`markdown`/`hidden`/`none` (only
  `solid`/`double`/`none`/`hidden` overlap in name with real CSS, and even
  those pick a specific box-drawing character set rather than a line style).
  `heavy` is not named `thick`, avoiding a collision with real CSS's
  `border-width: thick` keyword.
- **The `border`/`border-<edge>` shorthand is matched positionally, not by
  CSS value type.** Real CSS's border shorthand allows `<width>`/
  `<style>`/`<color>` in any order, and a real width keyword like `thick`
  can't be distinguished from a style keyword by content alone once it's in
  an unexpected slot. A consequence: the real-CSS two-value `<width>
  <style>` form (no color) is indistinguishable from this engine's
  `<style> <color>` form and is silently dropped. Use the three-value form
  or set `border-style` directly.
- **`border-left: none`/`border-right: none` keep that side's corners** even
  though the vertical rule itself is gone (`┌────┐` with no `│` down the
  left side, not `────┐`). That is the same convention any block element's
  border already uses, and `<table>`'s own border unifies with that machinery
  (see `docs/TABLES.md`).
- **`:hover` has no real pointer-hover meaning.** The only place it
  matches anything is `option:hover` inside an open `<select>` popup,
  repurposed to mean "the arrow-key-highlighted option" (see
  `docs/SELECT.md`). **`:focus`** likewise matches a synthetic marker
  `Element.Focus` sets, not real window/pointer focus, and only means
  anything against a live `Document`, not one-shot `Renderer.Render`.
- **`::selection` supports a narrower property set than real CSS:**
  `color`, `background-color`, `font-weight`, `font-style`, and
  `text-decoration` (mirroring `::marker`'s own supported set here), versus
  spec's `color`/`background-color`/`text-decoration`/`text-shadow`/
  `-webkit-text-stroke-color`/`caret-color`. With neither `color` nor
  `background-color` set, it falls back to reverse video rather than a
  browser's platform-native selection color, which has no terminal-neutral
  equivalent. See `docs/proposals/CARET_SELECTION.md`.
- **`text-transform: superscript`/`subscript`** substitutes each character
  for its Unicode superscript/subscript code point where one exists
  (there's no real script or font rendering). Characters with no Unicode
  equivalent pass through unchanged. **`font-variant: small-caps`**
  similarly uppercases everything, since terminals can't render true small
  caps.
- **`background` shorthand** only extracts the color component. `image`,
  `repeat`, `attachment`, `position`, `size`, `origin`, and `clip` tokens
  are recognized as present (so they don't break parsing) but otherwise
  ignored.
- **Bare ANSI color index numbers** (e.g. a raw `"214"`) are not accepted
  as a color value, even though they'd be meaningful to this renderer
  specifically. Use `#rrggbb` or a named color and let automatic
  downsampling handle the terminal's actual palette.
- **`list-style-type`/`symbols()` implement only a small slice of the real
  spec's predefined counter styles.** `disc`/`circle`/`square`/`decimal`/
  `lower-alpha`/`upper-alpha`/`lower-roman`/`upper-roman`/a quoted
  string/`symbols()`, versus the full
  [CSS Counter Styles](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/list-style-type)
  list (`armenian`, `georgian`, the CJK/Japanese/Korean variants, `hebrew`,
  `devanagari`, `disclosure-open`/`disclosure-closed`, and so on; see "Not
  Supported" below). `symbols()` itself is a real CSS Counter Styles
  function, just without its `<symbols-type>` keyword or image arguments.
- **`width` is a border-box size but `height` is a content-box one.** A
  declared `width` is the box's total visual width, with margins, border
  characters, and padding all coming out of it, while a declared `height` is
  the number of *content* lines, with the box's own top/bottom border rules
  and vertical padding added on top. `width: 10; height: 3; padding: 1;
  border-style: solid` is 10 columns wide and 7 rows tall. The asymmetry is
  deliberate (a width that didn't include its border characters wouldn't let
  `width: 100%` line up with the terminal's own width), but it does mean the
  two properties can't be reasoned about interchangeably. Flex layout resolves
  main-axis sizes as outer sizes on *both* axes and converts at the boundary,
  so a flex item is exempt; see the next entry.
- **`flex-basis`/`width` on a flex item size the item's whole outer box**, not
  its content box, so an item's margins come out of that size rather than
  adding to it: `flex-basis: 6; margin-right: 4` occupies 6 columns of the
  line here and 10 in a browser. This follows the same convention `width`
  already has everywhere else in this engine (a declared width is the total
  visual width of the box, margins and border characters included), which is
  what makes `width: 100%` line up exactly with the renderer's width. Every
  main-axis size flex layout resolves is an outer size in the same sense,
  including `column` direction's heights: a `flex-basis: 4` bordered column
  item is 4 rows tall in total, two of which are its own border rules.
- **An atomic inline box taller than one line breaks the line it sits in.**
  `inline-block` and `inline-flex` elements are rendered as one indivisible
  unit and spliced into the surrounding inline flow; when that unit is more
  than one line tall, its line breaks go into the flow as-is instead of the
  box being placed beside the text on the same baseline, so the text before
  and after it ends up on separate rows. Keep such boxes to a single line if
  they share a line with text. Easiest to hit via `inline-flex`, where any
  item that wraps makes the container two lines tall.
- **Flex alignment is `safe`, not `unsafe`, when content overflows.** With
  negative free space, `justify-content`/`align-content` and
  `align-items`/`align-self` all pack against the start edge, as if the value
  carried CSS Box Alignment's `safe` keyword. Browsers default to `unsafe`
  there: `center` overflows equally off both edges, `flex-end` off the start
  edge. Aligning that way means placing content at a negative offset from the
  container's own content origin, meaning columns to the left of it or rows
  above it,
  and on a character grid those cells belong to the container's border/padding
  or to a sibling, so the content wouldn't overflow, it would be lost. Safe
  alignment is what CSS itself defines for exactly that case. (This also
  matches, incidentally, the spec's own negative-free-space fallbacks for the
  `space-*` values: §8.2 makes `space-between` behave as `flex-start`, and
  `space-around`/`space-evenly` as `center`, which needs the same
  unrepresentable negative shift.)
- **A flex item is never narrower or shorter than one cell.** No box can render
  in zero columns or lines, so every resolved main size floors at one, which is
  visible where CSS would produce a genuinely empty box: `max-width: 0` caps an
  item at one column rather than none, and an item whose whole allotment is
  border characters paints them anyway. The floor is applied to *resolved* sizes
  only, not to the arithmetic: a `flex-basis` of `0` (what the common `flex: 1`
  expands to) really is zero when `flex-grow` divides the container up, on both
  axes, so equal factors give exactly equal shares and unequal ones split in
  exactly their ratio.
- **An over-shrunk flex item is clipped, where CSS would let it overflow.**
  This one is unusual, so it's worth stating why. CSS keeps it from arising in
  the first place: `min-width`'s initial value on a flex item's main axis is
  `auto`, the *automatic minimum size*, which stops `flex-shrink` at the item's
  min-content width, and that is implemented here (see CSS.md's Flexbox
  section, including both of the spec's opt-outs). But the automatic minimum is
  clamped by a definite `width`, and either opt-out (`min-width: 0`, a
  non-`visible` `overflow-x`) removes it, so an item can still end up narrower
  than its content. A browser then paints the overflow *over* its neighbors,
  leaving their positions untouched. This engine assembles a flex line by
  concatenating each item's rendered strings, so an item painting wider than
  its allotment doesn't overlay its siblings. It **displaces** them into
  columns that belong to somebody else, and only on the rows that actually
  overflow, so the item's own border no longer lines up with its own content
  and the container's border lands mid-text. That's not visible overflow, it's
  a desynchronized frame, and a character grid has no way to express the
  former. The item is therefore clipped to the main size flex resolved for it,
  honoring `text-overflow` as the marker if one is set (CSS paints no marker
  for `overflow: visible`, so that part is a pure opt-in).

  The clip is deliberately narrow, and it is *not* a general "flex clips" rule:
  a flex **line** that overflows its container is left alone, exactly like
  every other overflowing box in this engine: a plain bordered block whose
  unbreakable content exceeds its `width` also paints past its own border. Only
  the item-vs-allotment case is clipped, because only that case corrupts the
  positions of *other* elements.

  Nor does it have a `column`-direction counterpart. An item that ends up
  shorter than its content overflows *downward*, and a taller row stack pushes
  nothing sideways, so there's nothing to desynchronize and the item simply
  paints in full past its resolved height, as it would in a browser. The
  vertical automatic minimum keeps `flex-shrink` from producing that situation
  in the first place, same as the horizontal one.
- **`flex-basis: auto` measures `fit-content`, not `max-content`.** An item
  with no `flex-basis`/`width` gets a shrink-to-fit measurement of its own
  content, but the measurement is taken at the container's content width, so
  an item whose text is longer than that reports the container width (its text
  already wrapped) rather than the single-line length a browser would use.
  Items narrower than the container measure exactly as they would in a
  browser; the difference only shows up in how `flex-shrink` splits a deficit
  among items that are individually wider than their container. `flex-basis:
  max-content` asks for the real thing and gets it: the measurement there is
  taken at an effectively unbounded budget.

### Terminal-Native Additions

- **Literal-glyph border values.** In `border-left: "▌"` or
  `border-top: "═"`, a quoted string sets that exact character directly. This
  is the engine's original, and still primary, way to use arbitrary
  box-drawing characters that have no named-preset equivalent.
- **`border-*-corner`** supplies literal corner-glyph overrides for any bordered
  box; real CSS has no per-corner styling concept at all. **`border-style`**
  naming a *complete glyph-set preset* (`solid`/`rounded`/`heavy`/`double`/
  `markdown`/`standard`/`hidden`/`none`) rather than a real-CSS line-style
  keyword is the same kind of addition. See `docs/TABLES.md` for examples
  of every preset, and its "`border-collapse: collapse`" section for how
  htmlterm ranks these presets against each other when adjacent
  cells/tables disagree (a conflict-resolution model real CSS also has, but
  keyed to its own line-style vocabulary, not this one).
- **`border-*-junction`** (`border-top-junction`/`-right`/`-bottom`/
  `-left`/`-center`, plus the 4 corner-shape variants). A literal-glyph
  override for a T-junction/cross/corner-shape wherever that exact arm
  combination occurs in a `border-collapse: collapse` grid; real CSS has
  no equivalent concept at all (no notion of styling "where a divider
  joins a border" independent of the border lines themselves). The 5
  T-shape/cross properties work on both `<table>` and `<th>`/`<td>`, same
  as every other border property; the 4 corner-shape variants are
  table-only (a cell-level version would be redundant with cell-level
  `border-*-corner`). See `docs/TABLES.md`'s "Junction and corner
  overrides".
- **`scrollbar-style: block|shaded|classic|ascii|line`** and
  **`::scrollbar`/`::scrollbar-track`/`::scrollbar-thumb`/
  `::scrollbar-cap-start`/`::scrollbar-cap-end`.** Real CSS has no
  standardized scrollbar-styling API (only nonstandard, prefixed
  `::-webkit-scrollbar-*` in Chromium); this is htmlterm's own equivalent,
  including clickable cap buttons with no real precedent at all. See
  `docs/SCROLLBARS.md`.
- **OSC 8 terminal hyperlinks** for `<a href>`, an actual terminal escape
  sequence, emitted automatically, not a CSS feature at all.
- **A plain `optgroup` selector** styles an open `<select>` popup's
  `<optgroup label>` header row (`background-color`, `color`). Real CSS has
  essentially no `<optgroup>` styling hook at all (browsers just bold+indent
  the label via their own UA stylesheet, not something an author can
  restyle); this is the same kind of repurposing `option:hover` already is.
  See `docs/SELECT.md`.

### Not Supported

- **CSS units:** `px`, `em`, `rem`, `vw`, `vh`, and friends (ignored; use
  bare integers, `ch`, or `%`).
- **Percentage `height`/`min-height`/`max-height`** (`%` on the *vertical*
  axis). Widths resolve percentages fine; heights don't, on any box, since there is
  no established containing-block height to resolve them against, since a
  terminal's own height is a viewport this renderer writes past rather than a
  bound it lays out within. `height: 50%` is simply ignored, leaving the box at
  its content height. One visible consequence in flex layout: a percentage
  `height` doesn't count as a definite cross size, so it does *not* opt an item
  out of `align-items: stretch` the way an absolute one does: the item is
  stretched to the line instead, which is the better of the two available
  answers given the percentage can't be resolved either way.
- **CSS math:** `calc()`, `min()`, `max()`, `clamp()`. (Custom properties,
  `--foo` and `var()`, *are* supported; see CSS.md's "Custom Properties
  (Variables)" section.)
- **At-rules:** `@media`, `@font-face`, `@keyframes`, `@import`,
  `@charset`, `@supports`, `@page`, `@counter-style` (no custom counter
  styles), and the rest. The parser recognizes any `@`-rule and skips it as a unit
  rather than erroring or corrupting the rest of the stylesheet, but none
  of them do anything. `@media`/`@container`-style responsive breakpoints
  (width, `prefers-color-scheme`) are a deliberate scope decision, not an
  oversight: `internal/cssengine.ParseStylesheet` is a flat, non-nesting
  lexer that can't represent a braced at-rule body at all, and the intended
  answer is for a host (typically `tui.Loop`) to pick which pre-authored
  stylesheet string to hand `Options.Stylesheets` in response to a resize
  or theme-change event, not for `cssengine` to evaluate conditions itself.
  See `docs/proposals/RESPONSIVE.md`.
- **Pseudo-classes/elements beyond the supported list.** Notably
  `:active`, and any real mouse-hover semantics.
- **`revert`** (the fourth CSS-wide cascade keyword, alongside `inherit`/
  `unset`/`initial`, which are supported). Reverting to the user-agent
  stylesheet's value requires distinguishing UA-stylesheet origin from
  author-stylesheet origin, which this cascade doesn't model.
  `inherit`/`unset`/`initial` are also not supported inside
  `::before`/`::after`/`::marker`/scrollbar pseudo-element declarations,
  which resolve inheritance through a separate mechanism.
- **Layout models:** `display: grid`, `display: list-item`, any other
  `display` value beyond `block`/`inline`/`inline-block`/`flex`/
  `inline-flex`/`table`/`contents`/`none`, including the standalone
  CSS-table display values (`table-row`/`table-cell`/`table-row-group`/
  etc.) that let a non-`<table>` element opt into table layout; only a
  literal `<table>`/`<tr>`/`<td>`/`<th>` element tree gets table treatment
  here; `float`/`clear`.
- **`box-sizing`.** Parses without error but is always a no-op; there's no
  `content-box`/`border-box` distinction because htmlterm's box model
  always behaves like `border-box` (padding/border are subtracted from a
  declared `width`/`height`, not added on top of it).
- **`line-height`, `letter-spacing`, `word-spacing`.** No per-line vertical
  spacing or extra inter-character/inter-word spacing concept exists; a
  terminal cell grid has no sub-line leading to adjust and no fractional
  character-width budget to insert gaps into.
- **`outline`/`outline-offset`.** No separate box-decoration layer outside
  the border exists to draw one on; use `border`/`border-style` directly
  (including on `:focus`) for a terminal-native focus ring instead.
- **`direction`/`unicode-bidi`, and the `dir` HTML attribute.** No
  bidirectional text algorithm is implemented; RTL scripts render in
  logical (source) order, not visually reordered, and `dir="rtl"`/`dir="auto"`
  have no effect on layout.
- **Logical inset properties** (`inset-inline-start`/`-end`,
  `inset-block-start`/`-end`) for `position` offsets. Only the physical
  `top`/`right`/`bottom`/`left` longhands are read (unlike margin/padding/
  border, which do support the logical `*-block-*`/`*-inline-*` aliases).
- **Positioned layout:** `position: sticky` (`relative`/`absolute`/`fixed`
  and `z-index` are all supported; see CSS.md). For
  `absolute`/`fixed`: real CSS's shrink-to-fit auto-sizing (an
  unconstrained element here stretches to its containing block's width
  instead) and the "static position" algorithm (an axis with neither side
  set defaults to the containing block's top-left corner, not roughly
  where the element would have flowed) are not implemented; neither is
  `absolute`/`fixed` set directly on a `<tr>`/`<td>`/`<th>`/`<li>` itself
  (as opposed to content nested inside one). `<select>`'s dropdown remains
  the only overlay driven by dedicated Go code (`select_popup.go`) rather
  than routed through the general `position` mechanism. The underlying
  compositing primitives (`spliceColumns`, shifting a sub-rendered box's
  positions by an offset) are shared by both, but `<select>`'s popup
  predates and isn't (yet) reimplemented on top of `position`. See
  `docs/RENDERING.md`.
- **Visual effects:** `box-shadow`, gradients, `background-image`,
  `transform`, `transition`/`animation`, `filter`.
- **Flexbox gaps:** `flex-wrap`/`align-content` in `column` direction,
  `flex-wrap: wrap-reverse`'s reversed line order (it wraps, but the lines
  stack top to bottom like plain `wrap`), `align-content: stretch` (approximated as
  `flex-start`), `justify-items`/`justify-self` (grid properties, inert in a
  flex container anyway, so the `place-items`/`place-self` shorthands that
  set them are effectively just `align-items`/`align-self`),
  `baseline` alignment, the physical `left`/`right` alignment
  keywords, `margin: auto` beyond row direction's main axis (row direction's
  `margin-left`/`margin-right: auto` is supported),
  `flex-basis`'s intrinsic sizing keywords in `column` direction
  (`min-content`/`max-content`/`fit-content` all behave as `content` there;
  they are fully distinct in `row` direction),
  a collapsed flex item's cross-size **strut** (`visibility: collapse` does drop
  the item from layout as `display: none`, per §4.4, but a browser keeps the
  collapsed item's cross size contributing to its line's height and this engine
  sizes the line from its remaining items instead, since reserving a blank band for
  an item nobody can see reads as a rendering bug on a character grid),
  and text nodes directly
  inside a flex container, which real CSS wraps in anonymous flex items but
  this engine drops (wrap loose text in a `<span>`).
  Also `overflow-y: scroll`/`auto` on a flex container, which would need the
  live scroll-offset and gutter plumbing ordinary boxes have (`overflow-y:
  hidden`/`clip` does clip a flex container to its `height`/`max-height`, and
  `overflow-x` is likewise supported; see `docs/SCROLLING.md` for what the
  scrolling variants entail).
  `flex-grow`/`flex-shrink`/`justify-content` do work in `column` direction,
  but only once the container has a declared `height` or `min-height`. This
  engine has no other notion of a column flex container's main-axis size (row
  direction's width is always definite, so it never needed this condition).
  A `min-height` serves only as a floor there: it can grow the container for
  `flex-grow` to fill, but content taller than it overflows rather than being
  handed to `flex-shrink`, and percentages inside the container still need an
  explicit `height` to resolve against (the real main size is
  `max(content, min-height)`, which isn't definite until the content is laid
  out; real CSS calls a percentage against such a basis `auto` too).
  See CSS.md's Flexbox section for the full reasoning per gap.
- **Table gaps:** `border-collapse: collapse`'s conflict resolution doesn't
  consult `tr`/`thead`/`tbody`/`tfoot` `border` (`col`/`colgroup` are
  consulted, via real conflict resolution against their column's cells). The
  legacy HTML `border`/`cellpadding`/`cellspacing` presentational attributes
  on
  `<table>` aren't read either (use the CSS equivalents). Multi-line cell
  content combined with `white-space: nowrap` remains unsupported under
  either border model. See `docs/TABLES.md`.
- **List gaps:** `list-style-image`; most of the real spec's predefined
  `list-style-type` counter styles: `armenian`/`lower-armenian`/
  `upper-armenian`, `georgian`, the CJK/Japanese/Korean variants
  (`cjk-decimal`, `cjk-ideographic`, `japanese-formal`/`informal`,
  `korean-hangul-formal`, etc.), `hebrew`, `devanagari` and the other
  script-specific systems, `disclosure-open`/`disclosure-closed`,
  `decimal-leading-zero`, and `lower-greek`. Only the small Western subset in
  CSS.md's `list-style-type` entry is implemented.
- **`<select>` gaps:** per-`<option>` (and per-group-label-row) border/
  padding, and width, and `<optgroup>` nested inside another `<optgroup>`; see
  `docs/SELECT.md`.
- **`font-size`.** There is no concept of font size at all; terminal
  glyphs are a fixed cell size.

---

## DOM & Events

### Deviations from Spec

- **Coordinates are terminal cells, not pixels.** `DispatchClick(row, col)`
  hit-tests against `Element.Rect()`'s row/column grid, not a
  `MouseEvent`'s `clientX`/`clientY`.
- **`Event` carries less than a real DOM event, but does carry modifier
  keys.** `ShiftKey`/`CtrlKey`/`AltKey`/`MetaKey` are set from the
  `Modifiers` passed to `DispatchClick`/`DispatchKey` (zero/false for event
  types with no real modifier-key concept, namely `"resize"`, `"submit"`,
  `"focus"`, `"blur"`, and `"change"`, matching real DOM's UIEvent-only
  modifier fields). Still missing: `relatedTarget`, `button`, `detail`
  (click count). `Event.Key` is a single UTF-8 rune or one name from a fixed
  vocabulary (`"Enter"`, `"Backspace"`, `"Tab"`, `"Escape"`, arrow keys). The
  host translates raw terminal input into these strings, and raw terminal
  modifier bits into `document.Modifiers`, itself; htmlterm never reads a
  terminal directly outside of `Loop`.
- **Only one click "kind" exists.** There's no `mousedown`/`mouseup`/
  `dblclick`/`contextmenu`/drag events, just a single synthesized
  `"click"` that hit-tests and runs default actions atomically, including
  the focus-plus-caret-placement default action a real `mousedown` alone
  would run (see `docs/proposals/CARET_SELECTION.md`). There's no way to
  observe focus having moved before "click" fires, the way a listener on a
  real page could.
- **`"cut"`/`"paste"` are selection-scoped, with one narrower-than-spec
  fallback.** `DispatchCut`/`DispatchPaste` act on the focused text entry's
  current `[SelectionStart, SelectionEnd)` range when it's non-collapsed
  (removing it, or replacing it with the pasted text), but a *collapsed*
  cut (no active selection) falls back to acting on the field's whole value,
  rather than real spec's "nothing to cut" no-op, preserving this package's
  original append/whole-field behavior for any caller that never touches
  `SetSelectionRange`. `Event.ClipboardData` is a plain string, not a real
  `DataTransfer`/`clipboardData` object with MIME types.
- **`DispatchWheel` mutates scroll position directly** and returns whether
  anything scrolled. Unlike every other `Dispatch*` method, it does not
  dispatch a `"wheel"` `Event` a listener could observe or prevent. It
  takes both a `deltaX` and `deltaY` (mirroring a real `WheelEvent`),
  scrolling each axis's own nearest scrollable ancestor independently (they
  can be two different elements for a nested pane). `Loop` wires tcell's
  `WheelLeft`/`WheelRight` mouse buttons to `deltaX` directly, and remaps a
  Shift-held vertical wheel notch to `deltaX` too (the common browser/
  terminal fallback convention for input hardware that never reports a
  horizontal wheel bit at all).
- **`readonly` blocks edits but has no matching pseudo-class.** A text
  entry carrying `readonly` (or `disabled`) still focuses, moves its caret,
  selects, and submits with its form, but `DispatchKey`'s typing/Backspace/
  Delete/Enter-newline default actions and `DispatchPaste`'s insertion all
  skip it, and `DispatchCut` degrades to a copy (`ClipboardData` is still
  populated; the value is left alone), matching real HTML. What's missing
  is the CSS half: there's no `:read-only`/`:read-write` pseudo-class to
  style the state with (see CSS.md's supported list), and no
  `Element.SetValue` restriction, since spec's own `readonly` only ever
  constrained *user* edits.
- **Tab order defaults to a document-order walk** over form controls
  (`input`/`button`/`textarea`/`select`, skipping `disabled`) plus
  focusable scroll containers, reorderable via `tabindex` (see
  `docs/DOM_API.md`), but **plain `<a>` links are never tab stops** unless
  given explicit `tabindex` (real browsers make links focusable by default
  with no attribute needed; htmlterm requires opting in, since link-dense
  rendered content, such as documentation pages and HTML email, would otherwise
  flood Tab order with every inline link ahead of a page's actual
  controls).
- **`Element.Style()` doesn't round-trip shorthand properties or preserve
  declaration order.** It mirrors `CSSStyleDeclaration` for the inline
  `style=""` attribute only (there's no `getComputedStyle`; see
  `docs/DOM_API.md`), but `SetProperty("margin", "1px")` is stored as
  `margin-top`/`-right`/`-bottom`/`-left` (so `GetPropertyValue("margin")`
  afterward returns `""`, not `"1px"`), and `CSSText`/`SetCSSText` serialize
  properties sorted by name rather than in original or set order. Both
  because shorthand expansion happens at parse time with no record that a
  shorthand was used, the same limitation the cascade itself has.
- **`"focus"`/`"blur"` bubble here, unlike spec.** Every dispatched event
  runs the same capture/target/bubble chain (`runDispatch` in `event.go`
  has no per-type bubbling suppression). Real DOM's `focus` and `blur` never
  bubble (only their `focusin`/`focusout` counterparts do, which htmlterm
  doesn't have separately-named events for). An ancestor listener can
  observe a descendant's `"focus"`/`"blur"` directly here; no
  `focusin`/`focusout` equivalent is needed or provided.
- **`Element.DispatchEvent` runs no default action for a built-in-named
  event type.** Dispatching e.g. `NewCustomEvent("click", ...)` through
  `DispatchEvent` runs any registered listeners normally, but skips
  checkbox/radio toggle, submit forwarding, focus traversal, and every other
  default action a real browser *would* still run for a synthetically
  dispatched built-in event (`checkbox.dispatchEvent(new MouseEvent("click"))`
  genuinely toggles it in a real browser). Here, that behavior lives inside
  each specific `Dispatch*` method (`DispatchClick`, `DispatchKey`, etc.),
  not in the shared dispatch engine `DispatchEvent` also uses. A real,
  named gap versus spec, not spec ambiguity.

### Terminal-Native Additions

- **Scroll containers as tab stops:** an `overflow-x`/`overflow-y`
  `auto|scroll` element with an explicit width/resolved height respectively,
  and no other focusable descendant, automatically becomes a keyboard tab
  stop, purely so `PageUp`/`PageDown`/arrow keys have something to scroll
  once focused.
- **Scrollbar cap buttons** (`::scrollbar-cap-start`/`-end`) are clickable
  via `DispatchClick`, but do *not* dispatch a `"click"` `Event` on the
  scrollable element, since they're rendering chrome, not real element content
  (see `docs/SCROLLBARS.md`).
- **`<select>`'s dropdown popup** is a synthesized text overlay with its
  own synthetic `Rect` per `<option>`, composited on top of already-
  rendered output. Real DOM has no equivalent to this text-based popup
  compositing step (see `docs/RENDERING.md`).
- **`Event` is one flat struct, not a class hierarchy.** `NewCustomEvent`/
  `Element.DispatchEvent` support custom events with an untyped
  `Detail any` payload, `Bubbles`, and `Cancelable`. Real DOM instead has
  `Event`/`CustomEvent`/`UIEvent`/`MouseEvent`/`KeyboardEvent` as separate
  constructors/types in a class hierarchy. htmlterm reuses the same `Event`
  type (with `ClipboardData`/`Key`/modifier fields already sitting unused
  alongside `Detail` for a custom event) for every event, built-in or
  custom, matching this package's existing "one struct, not subclasses"
  style.

### Not Supported

- **`mousemove`, `mouseover`/`mouseout`, `dblclick`, `contextmenu`,
  drag-and-drop events.** No continuous hover tracking exists in a
  terminal, and none of these are wired up.
- **`keyup`/`keypress`.** Only `"keydown"` is dispatched; there's no
  key-release event, so a host can't detect a key being held or released
  (only that a key was pressed once).
- **`focusin`/`focusout`.** There's no separately-named bubbling pair;
  see "`\"focus\"`/`\"blur\"` bubble here, unlike spec" under "Deviations
  from Spec" above for how to observe focus entering/leaving a descendant
  instead.
- **Text selection and `"select"`/`"copy"` events.** There's no
  click-drag/keyboard text-selection model at all (a real terminal's own
  mouse-drag selection is unavailable once `Loop.Run` enables mouse
  reporting: held-modifier drag, such as Option on macOS Terminal.app or iTerm2
  or Shift on many Linux terminals/Windows Terminal, bypasses the app and
  falls back to native OS selection/copy). `"cut"`/`"paste"` are supported
  (see "Clipboard" above) but always act on a whole field, not a selected
  range, since there's no selection to scope them to.
- **Shadow DOM, custom elements, `MutationObserver`.** There is no tree-
  change observation API; a host must re-render after mutating.
- **Windows.** `Loop`'s automatic resize tracking requires `syscall.SIGWINCH`,
  which doesn't exist on Windows at all, a compile-time constraint there,
  not just an unverified one. The rest of the interactive layer (raw
  terminal mode via `golang.org/x/term`) is POSIX-oriented and hasn't been
  verified on Windows either.
- **Concurrent access.** A `Document` has no internal locking. `Loop.Run`'s
  own goroutine is the only place that may ever mutate a `Document` or
  trigger a repaint; timer callbacks and dispatched event handlers all run
  there too. There's no DOM-level concurrency model to compare this against
  (browsers are single-threaded per document too), but it's worth stating
  plainly for Go callers: calling `Document` methods from multiple
  goroutines is not safe.

---

## See Also

- **CSS.md.** The exhaustive property-by-property reference.
- **docs/DOM_API.md.** A field-by-field table comparing the real DOM's
  `Document`/`Element`/`Node` interfaces against `document.Document`/
  `document.Element`, method by method.
- **docs/SELECT.md.** `<select>` popup styling in full.
- **docs/SCROLLBARS.md.** Scrollbar gutter styling in full.
- **docs/TABLES.md.** Table border/margin/padding styling in full,
  including rendered examples of every `border-style` preset.
- **docs/SCROLLING.md.** The scrolling/viewport design.
- **docs/INTERACTIVE.md.** The `Document`/`Element`/events design history.
- **SECURITY.md.** A different axis from this doc (safety against
  untrusted HTML/CSS, not fidelity to spec): terminal escape-sequence
  stripping, and `StripHiddenInline`/`IgnoreDocumentCSS` for content a
  browser would render invisibly but htmlterm can remove structurally.
- **docs/RENDERING.md.** The rendering engine's own internals, including
  why popups (`<select>`'s dropdown) are the one exception to "no
  positioned layout."
