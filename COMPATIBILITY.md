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
- **Not Implemented.** Features that are not currently implemented.
- **Not Supported.** Features that can't work in terminal rendering, 
  or are unlikely to be implemented.

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

  Form reset (`<input type="reset">`, `<button type="reset">`, Enter on a
  focused reset control) works despite that, but through a narrower
  mechanism than a real dirty-value flag: `ParseDocument` walks the tree once
  and snapshots every control's parse-time value, checkedness, or selected
  option, the same one-time approximation `autofocus` already uses elsewhere
  in this list. Activating a reset control fires `"reset"` on the nearest
  `<form>` and, unless a listener calls `PreventDefault`, restores every
  descendant control to its snapshot, mirroring HTML's own event-then-reset
  order. A control added to the tree after `ParseDocument` (`AppendChild`,
  `SetInnerHTML`) has no snapshot and is left untouched by reset, the same
  gap `autofocus` has for a control inserted after parse.
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
- **A `<label>` click redirects to its named control as one event, not two.**
  Both association forms work: an explicit `for` pointing at an id anywhere
  in the document, and, lacking that, the first labelable descendant nested
  inside the label (an `<input>` other than `type="hidden"`, a `<button>`,
  a `<textarea>`, or a `<select>`). A real browser fires `"click"` on the
  label itself first, then, as the label's own default action, synthesizes
  a *second*, separate `"click"` at the named control. `DispatchClick`
  instead resolves the control before dispatching anything and fires one
  `"click"` targeted at the control directly; a listener on the label never
  sees the event at all. This is the same "one click kind, atomic" model
  `DispatchClick`'s own doc comment already uses for
  mousedown-plus-click-plus-default-action, applied to label forwarding too,
  and it means a label with no resolvable control (a stale `for`, or no
  labelable descendant) keeps its own `"click"` dispatch rather than being
  left with nothing to fire at all. A click that lands directly on a nested
  control's own glyph, rather than the label's surrounding text, was
  already going straight to that control before this existed and is
  unaffected. See `docs/DOM_API.md`'s `dispatchEvent` row for the general
  shape of what a `Dispatch*` default action does and doesn't replicate from
  spec.

  Getting a `<label>` a click target of its own at all is also a rendering
  deviation, not just an event-dispatch one: `<label>` is `display: inline`
  by default, and a plain inline element normally has no `Rect` of its own
  here (see the "trackable element" invariant in `docs/ARCHITECTURE.md`) so
  there'd be nothing for `DispatchClick` to hit-test a click on the label's
  surrounding text against. Giving it one means rendering a label's content
  as one atomic unit, the same way `display: inline-block` already is,
  rather than letting it reflow across its own boundary with text before or
  after it on the same line. In the overwhelmingly common case, a label on
  its own line in a form, that's not a visible difference; it only matters
  for a `<label>` deliberately mixed into running prose. See CSS.md's
  `label` entry.
- **`autofocus` applies once, at `ParseDocument`, not through HTML's live
  algorithm.** Real HTML tracks a per-document "autofocus candidates" list
  and can focus a later-inserted autofocus element asynchronously, after
  the element with the attribute is inserted, provided nothing else already
  claimed focus by then. `ParseDocument` instead walks the freshly parsed
  tree once, synchronously, and focuses the first element, in document
  order, that carries `autofocus` and is currently focusable
  (`Document.isFocusable`, the same predicate `Element.Focus()` and Tab
  order use); an `autofocus` candidate that isn't focusable, disabled, say,
  is skipped in favor of the next one, rather than leaving nothing
  focused. Since there's no later "element inserted" moment this renderer
  reacts to, `autofocus` set through `SetAttribute` or present on an
  element added after `ParseDocument` (`AppendChild`, `SetInnerHTML`, and
  the like) has no effect; call `Element.Focus()` directly for that.

  The `"focus"` event this fires is unobservable, for a related reason: it
  dispatches inside `ParseDocument`, before that call has returned the
  `*Document` a caller needs in order to register a listener for it at
  all. `FocusedElement()` still correctly reports the autofocused element
  afterward; only the event itself is lost, not the resulting state. A
  real browser's own script can already be listening by the time its
  autofocus processing runs; this package's synchronous, code-only
  construction model has no equivalent moment.

### Terminal-Native Additions

- **`img::before { content: attr(alt) }`**, a UA stylesheet default. Since no
  image can ever render, alt text is shown inline directly, rather than in a
  broken-image icon's box the way a browser shows it.
- **`abbr[title]::after`** appends the title attribute's expansion inline,
  there being no hover-tooltip concept in a terminal to show it in instead.
- **`progress-style`/`meter-style` and `::progress-bar`/`::progress-value`/
  `::meter-bar`/`::meter-optimum-value`/`::meter-suboptimum-value`/
  `::meter-even-less-good-value`.** Real CSS has no author-facing hook to
  restyle `<progress>`/`<meter>`'s bar glyphs at all (WebKit's own
  `-webkit-`-prefixed pseudo-elements this borrows names from are
  non-standard); `progress-style`/`meter-style` reuse `scrollbar-style`'s
  exact five preset names and merge mechanism. See `docs/PROGRESS_METER.md`.

### Not Implemented

- **`<details>`/`<summary>` always render fully expanded.** No collapse/
  expand interactivity exists; a real browser's native disclosure widget
  has no terminal equivalent here.
- **An indeterminate `<progress>` (no `value` attribute) renders statically,
  not animated.** Real browsers show an animated barber-pole; this renderer
  has no `animation` support at all, so `:indeterminate` instead shows
  `::progress-bar`'s track glyph repeated across the full width, with no
  fill. See `docs/PROGRESS_METER.md`.
- **`<dialog>`/`<datalist>`.** No native-modal or autocomplete-popup
  equivalent exists; unhandled the same way any other unrecognized element
  is (generic inline fallback, usually rendering no visible content).
- **Type-specific `<input>` UI beyond `checkbox`/`radio`/`submit`/
  `reset`/`button`/`hidden`.** Every other `type` value, including
  `range`, `number`, `date`, `time`, `month`, `week`, `color`, `file`,
  `email`, `url`, `tel`, and `search`, renders the same generic way as
  the default, unset type: the plain `value` (or `placeholder` when
  empty), space-padded to the resolved width, no brackets (CSS.md's
  `input` entry covers that rendering). The type-specific interaction is
  missing along with it. A `range` input has no slider track or thumb. A
  `number` input has no spinner buttons and no arrow-key increment or
  decrement default action. A `date`/`time`/`month`/`week` input has no
  picker and no field-by-field editing. A `color` input has no swatch or
  picker. A `file` input has no file-open dialog and no `FileList`; its
  `value` is whatever string an author or `SetValue` put there. Typing
  still works, since none of these types opt out of the generic
  single-line-text default actions `DispatchKey` already runs for any
  text entry.

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
- **`contenteditable`.** The attribute isn't read anywhere; there's no
  editable-content model outside a real `<input>`/`<textarea>` (see "Form
  controls are attribute-driven" above).
- **Accessibility attributes: ARIA beyond `aria-hidden`, and
  `accesskey`.** The UA stylesheet's `[aria-hidden=true] { display: none
  }` rule (see CSS.md) is the only place any `aria-*` attribute has
  meaning. `role`, `aria-label`, `aria-live`, `aria-expanded`,
  `aria-checked`, and the rest of the ARIA vocabulary are read as plain
  attributes and given no semantics. `accesskey` isn't read at all;
  there's no keyboard-shortcut dispatch tied to it. Neither gap has a
  terminal-native workaround the way `:hover` or `::selection` do,
  because there's no accessibility tree here to attach one to: htmlterm
  produces styled text, not a tree a screen reader could query
  independently of that text.

---

## CSS

### Deviations from Spec

- **All sizing is in character cells/lines, not pixels.** `width`/`height`/
  `margin`/`padding`/border thickness are all integer counts, or `ch`, `%`,
  `vw`, or `vh`. `px`, `em`, `rem`, and the other absolute units are parsed as
  unsupported and ignored entirely, not converted: a cell is not a pixel and
  there is no font size for an `em` to be relative to. `vw`/`vh` *are*
  supported, because a terminal has an unambiguous viewport and a fraction of
  it is an exact number of cells. They quantize coarsely at terminal sizes
  (every value from `1vh` to `4vh` is one row on a 24-row screen), and `vh`
  needs `Options.Height` set, being ignored without one.
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
  `solid`/`rounded`/`heavy`/`double`/`markdown`/`standard`/`hidden`/`none`
  (only `solid`/`double`/`none`/`hidden` overlap in name with real CSS, and
  even those pick a specific box-drawing character set rather than a line
  style).
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
- **`box-sizing` defaults to `border-box` everywhere, via the UA
  stylesheet's own `*, ::before, ::after { box-sizing: border-box; }` rule**
  (`internal/render/defaultstylesheet.go`), the same reset Bootstrap's
  Reboot and `sindresorhus/modern-normalize` both ship, moved down one
  cascade layer. Real CSS's own initial value is `content-box`; that value
  is fully supported too, reachable by overriding the UA rule on any
  selector, the same way it's reachable in a browser with no reset loaded.
  Both `width` and `height` read it, and read it the same way, unlike
  before this existed (see the next paragraph for what changed). See
  `docs/proposals/BOX_SIZING.md` for the design history, including why the
  default landed on `border-box` rather than spec's own `content-box`.

  Under the default `border-box`, both `width` and `height` are a box's
  whole outer size: border characters and padding come out of a declared
  value rather than adding to it. `width: 10; height: 6; border-style:
  solid` is 10 columns wide and 6 rows tall in total, 2 of which (top and
  bottom) are its own border rules. Under an explicit `box-sizing:
  content-box`, both axes instead read as a pure content size, with border
  and padding adding on top: the same box is 10 + 2 = 12 columns wide and
  6 + 2 = 8 rows tall. A height too small to fit its own border and padding
  floors at 1 content line rather than disappearing, the same floor a
  too-small `width` already has (CSS never lets border and padding shrink
  content below 0): `height: 1; border-style: solid` is 3 rows tall, not 2,
  its one content line intact. `width: 100%` under `content-box` combined
  with padding or a border can produce a box wider than its container, the
  classic browser box-model gotcha the `border-box` default exists to
  avoid; an ordinary block in that state paints past its own right edge,
  the same as any other box whose unbreakable content exceeds its `width`
  (see that entry below).

  Before `box-sizing` existed, `width` unconditionally behaved like
  `border-box` and `height` unconditionally behaved like `content-box`, an
  asymmetric mix neither real value produces on its own. Implementing
  `box-sizing` for real, defaulting to `border-box`, reproduces the old
  `width` behavior exactly (it already was border-box-shaped) but changes
  the old `height` behavior for anyone combining an explicit `height` with
  padding or a border (it was always content-box-shaped, and now needs an
  explicit `box-sizing: content-box` to get that back). This is a real,
  visible output change on upgrade for that combination, matching what
  adopting a Bootstrap-style reset changes on a real page too, not an
  oversight.

  `width`'s own margin-subtraction behavior predates `box-sizing` and is
  independent of it: a declared `width`, under either value, still has
  horizontal margin subtracted out of it before border and padding are
  considered, so that `width: 100%` lines up exactly with the terminal's
  width. That is not what real CSS's own `border-box`/`content-box`
  distinction means (both real modes exclude margin from the box entirely);
  it is layered on top of it, the same way it always has been. `height` was
  never adjusted for margin this way and still isn't.

  **Table cells (`<th>`/`<td>`) are a compatibility gap here, not a spec
  exception.** Real CSS applies `box-sizing` to table cells exactly like any
  other box (CSS Box Sizing Module Level 3 states it applies to "all
  elements that accept width or height," internal table elements included,
  and every mainstream browser honors `box-sizing: border-box` on a `<td>`
  the same way it does on a `<div>`); nothing in spec singles tables out.
  htmlterm's cells simply don't read the property yet. Their column-width
  algorithm resolves a declared `width` through a different, older
  mechanism than `renderBlockContentBox`'s, one that predates this section
  and already behaves like neither real value on its own: it subtracts a
  cell's padding from its declared width, `border-box`-shaped, but adds
  border characters on top as separate overhead reserved outside that width
  entirely, `content-box`-shaped. Implementing real `box-sizing` there needs
  that existing mixed model replaced with an actual `content-box`/
  `border-box` pair first, which is a rewrite of `sizeColumns`'s fixed-column
  branch and the border-overhead accounting around it, not a small
  extension of the general-box work above. See `docs/TABLES.md`.
- **A percentage height needs `Options.Height` to resolve at the root.** CSS
  resolves a percentage `height`/`min-height`/`max-height` against the
  containing block's height only when that height is definite, and to `auto`
  otherwise (CSS 2.1 §10.5). That rule is implemented, and the chain is the
  real one: an explicit `height` on an ancestor is a definite basis, an
  auto height or a bare `min-height` is not, and the initial containing block
  is the viewport. The viewport here is `Options.Height`. Set it, and
  `height: 100%` on a root-level box resolves to the terminal's height, which
  is what makes the "column flex container fills the screen, one child takes
  the slack" pattern work. Leave it at `SizeAutomatic`/`SizeNatural` and there
  is no viewport, so a root percentage height resolves to `auto` and is inert.
  A browser always has a viewport and so never shows that second behavior.
  `height: 100vh` is the same thing without the chain of definite ancestors,
  and is under the same `Options.Height` condition.
- **`flex-basis`/`width` on a flex item size the item's whole outer box**, not
  its content box, so an item's margins come out of that size rather than
  adding to it: `flex-basis: 6; margin-right: 4` occupies 6 columns of the
  line here and 10 in a browser. This follows the same convention `width`
  already has everywhere else in this engine (a declared width is the total
  visual width of the box, margins and border characters included), which is
  what makes `width: 100%` line up exactly with the renderer's width. Every
  main-axis size flex layout resolves is an outer size in the same sense,
  including `column` direction's heights: a `flex-basis: 4` bordered column
  item is 4 rows tall in total, two of which are its own border rules. This
  makes a flex item's main-axis size exempt from `box-sizing`: it always
  behaves like `border-box` there, regardless of what the item's own
  `box-sizing` resolves to for anything else about it (a real declared
  height on a `row`-direction item, say, still honors its own `box-sizing`
  normally, only the synthesized main-axis size is exempt). Distribution,
  wrapping, and the anti-corruption clip below all depend on the main-axis
  size meaning the item's whole outer box; a flex container's own outer box
  is not exempt and follows `box-sizing` the same as any other block box.
- **An atomic inline box taller than one line breaks the line it sits in.**
  `inline-block` and `inline-flex` elements are rendered as one indivisible
  unit and spliced into the surrounding inline flow; when that unit is more
  than one line tall, its line breaks go into the flow as-is instead of the
  box being placed beside the text on the same baseline, so the text before
  and after it ends up on separate rows. Keep such boxes to a single line if
  they share a line with text. Easiest to hit via `inline-flex`, where a
  border, or any item that wraps, already makes the container two lines tall.
- **An `inline-block` element's own border and padding are not drawn.** The
  inline flow renders such an element by laying out its content and applying
  its `width`, which is the property CSS.md's `display` entry names as what
  `inline-block` adds to `inline`. The rest of the box model is skipped, so
  `display: inline-block; border-style: solid` paints no border and reserves no
  columns for one. `inline-flex` is the exception rather than the rule here: it
  is rendered through the same box model a block-level `display: flex`
  container uses, so its border, padding, margin, and overflow clipping all
  apply.
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
- **A `flex: 1` container measures narrower than a browser's when something
  asks how wide it is.** Sizing a flex container from its own content, rather
  than laying it out at a width it was handed, happens when it is a flex item
  of another container, an `inline-flex` box, or the content of an auto-width
  table cell. This engine answers with the sum of its items' hypothetical main
  sizes. That is exact for items whose flex base size comes from their content,
  meaning `flex-basis: auto` and width-sized items. It is too narrow for
  `flex-basis: 0` items, the expansion of the common `flex: 1`, whose
  hypothetical main size is the automatic minimum size, meaning min-content.
  Flexbox §9.9 instead runs each item's max-content contribution through a
  *chosen flex fraction*, applying the largest item's fraction to every item,
  which makes such a container **wider** than the same items carry with no flex
  factors at all. Nothing here implements that step, so `flex: 1` children of
  an intrinsically sized container wrap text a browser would keep on one line.
  This and the `flex-basis: auto` entry above are the same missing machinery:
  Flexbox §9.9's chosen-flex-fraction step, which nothing in this engine
  implements yet.

### Terminal-Native Additions

- **Literal-glyph border values.** In `border-left: "▌"` or
  `border-top: "═"`, a quoted string sets that exact character directly. This
  is the engine's original, and still primary, way to use arbitrary
  box-drawing characters that have no named-preset equivalent. More than one
  character (`border-top: "=-"`) tiles as a repeating pattern across the
  box's exact width rather than repeating by character count, real CSS having
  no equivalent to a literal repeating border glyph at all. `<hr>` is the
  most direct use of this: its UA default is `border-top: "─"`, so this is
  also the property that changes the rule's own character (see CSS.md's `hr`
  entry).
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
- **`text-transform: superscript`/`subscript`** substitutes each character
  for its Unicode superscript/subscript code point where one exists
  (there's no real script or font rendering). Characters with no Unicode
  equivalent pass through unchanged.
- **`font-variant: small-caps`**
  uppercases everything, since terminals can't render true small
  caps.

### Not Implemented

- `display: grid` is not implemented.
- **CSS math:** `calc()`, `min()`, `max()`, `clamp()`. (Custom properties,
  `--foo` and `var()`, *are* supported; see CSS.md's "Custom Properties
  (Variables)" section.)
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

### Not Supported

- **CSS units:** `px`, `em`, `rem`, `pt`, `cm`, `vmin`, `vmax`, and friends
  (ignored; use bare integers, `ch`, `%`, `vw`, or `vh`). A **zero** length is
  accepted in any unit, since a zero length is dimensionless in CSS and there
  is nothing to convert, so `flex: 1 1 0px` works.
- **Visual effects:** `box-shadow`, gradients, `background-image`,
  `transform`, `transition`/`animation`, `filter`.
- **`font-size`.** There is no concept of font size at all; terminal
  glyphs are a fixed cell size.
- **`line-height`, `letter-spacing`, `word-spacing`.** No per-line vertical
  spacing or extra inter-character/inter-word spacing concept exists; a
  terminal cell grid has no sub-line leading to adjust and no fractional
  character-width budget to insert gaps into.
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
- **Layout models:** `display: list-item`, any other
  `display` value beyond `block`/`inline`/`inline-block`/`flex`/
  `inline-flex`/`table`/`contents`/`none`, including the standalone
  CSS-table display values (`table-row`/`table-cell`/`table-row-group`/
  etc.) that let a non-`<table>` element opt into table layout; only a
  literal `<table>`/`<tr>`/`<td>`/`<th>` element tree gets table treatment
  here; `float`/`clear`.
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
- **Flexbox gaps:** `flex-wrap`/`align-content` in `column` direction
  (`wrap-reverse` included; in `row` direction it reverses both the line order
  and the cross-axis alignment keywords, as it should),
  `justify-items`/`justify-self` (grid properties, inert in a
  flex container anyway, so the `place-items`/`place-self` shorthands that
  set them are effectively just `align-items`/`align-self`),
  `baseline` alignment, the physical `left`/`right` alignment
  keywords,
  `flex-basis`'s intrinsic sizing keywords in `column` direction
  (`min-content`/`max-content`/`fit-content` all behave as `content` there;
  they are fully distinct in `row` direction),
  a collapsed flex item's cross-size **strut** (`visibility: collapse` does drop
  the item from layout as `display: none`, per §4.4, but a browser keeps the
  collapsed item's cross size contributing to its line's height and this engine
  sizes the line from its remaining items instead, since reserving a blank band for
  an item nobody can see reads as a rendering bug on a character grid).
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
- **`<select>` gaps:** per-`<option>` (and per-group-label-row) border/
  padding, and width, and `<optgroup>` nested inside another `<optgroup>`; see
  `docs/SELECT.md`.

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
- **A radio-button group is one Tab stop, and its own members ignore an
  explicit `tabindex`.** Every `<input type="radio">` sharing a `name`
  (scoped to the nearest `<form>`, or the whole document without one, the
  same scope `clearRadioSiblings` already used for click-driven
  exclusivity) is one stop in sequential Tab navigation: the checked
  member if one exists, otherwise the first in document order. The rest of
  the group drops out of Tab order the same way a negative `tabindex`
  already does (`inTabOrder`), reachable by `Element.Focus()` or a click
  regardless, same as any other `tabindex="-1"` element. Once a group
  member is focused, ArrowUp/Down/Left/Right move to the next or previous
  member, wrapping at either end and skipping a disabled one, and check
  the newly focused member (unlike Tab, which never changes the checked
  state by itself), matching real HTML's radio-group keyboard model. Real
  HTML lets an individual group member's own `tabindex` override its
  group-derived position in this computation; that refinement isn't
  implemented here, so an explicit `tabindex` on one radio in a group is
  silently ignored for Tab-order purposes, though it still takes effect
  for every other element type.
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
  `auto|scroll` element that actually became a scroll container (an explicit
  width/resolved height respectively for an ordinary block box; a
  `display: flex`/`inline-flex` container's `overflow-x` doesn't need one,
  per its own carve-out — see `CSS.md`'s overflow entry), and has no other
  focusable descendant, automatically becomes a keyboard tab stop, purely so
  `PageUp`/`PageDown`/arrow keys have something to scroll once focused.
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

### Not Implemented

- **`mousemove`, `mouseover`/`mouseout`, `dblclick`, `contextmenu`,
  drag-and-drop events.** No continuous hover tracking exists in a
  terminal, and none of these are wired up.
- **`keyup`/`keypress`.** Only `"keydown"` is dispatched; there's no
  key-release event, so a host can't detect a key being held or released
  (only that a key was pressed once).
- **Constraint validation.** `required`, `pattern`, `min`, `max`, `step`,
  `minlength`, and `maxlength` sit on the element as ordinary attributes
  and are never given effect beyond that. `required`'s bare presence is
  the one exception: it feeds the `:required` selector (see CSS.md) and
  nothing else. There is no `ValidityState`, no `Element.CheckValidity()`/
  `ReportValidity()`/`SetCustomValidity()`, and no
  `:valid`/`:invalid`/`:in-range`/`:out-of-range` CSS to style a field
  against. A submit control's default action fires a form's `"submit"`
  event unconditionally, regardless of whether a required field is empty
  or a value fails its own `pattern`/`min`/`max`/`step`. A real browser
  runs the constraint validation algorithm first, blocking submission and
  focusing the first invalid control when a field fails.

### Not Supported

- **`compositionstart`/`compositionupdate`/`compositionend`, and
  `beforeinput`.** No IME composition model exists. A terminal's own
  input path delivers text after composition has already happened,
  whether that's an OS-level input method composing CJK text or a
  terminal emulator resolving a dead-key accent, so there's no
  in-progress composition state that could ever reach htmlterm to
  expose. Composed input arrives the same way any other keystroke does,
  through `DispatchKey`'s single-rune default typing action.
- **Undo and redo for text entry.** There's no edit history for
  `<input>`/`<textarea>` values, and no `Event.Key` name reserved for an
  undo or redo chord (see the `Event.Key` deviation above). Every
  `DispatchKey` typing, Backspace, Delete, cut, or paste default action
  mutates a field's `value` directly and irreversibly. A host that wants
  undo has to build it itself, typically by snapshotting `Value()` on
  each `"input"` event and replaying a snapshot through `SetValue()`.
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
