# htmlterm Compatibility Notes

htmlterm reinterprets three separate web-platform surfaces for a fixed-size
character grid instead of a browser: **HTML** (parsing and per-element
rendering), **CSS** (selectors, cascade, properties), and **DOM & Events**
(the mutable `Document`/`Element` API and its native-Go event model). Each
gets the same four-part treatment below:

- **At a Glance** — what's supported, so you can tell quickly whether this
  covers your use case.
- **Deviations from Spec** — real features that exist here but behave
  differently than in a browser, and why (text cells, not pixels; no
  scripting engine; no real pointer/window).
- **Terminal-Native Additions** — things invented for this renderer with no
  browser equivalent at all.
- **Not Supported** — real features that simply aren't implemented.

This is the orientation read. For exact per-property syntax, see
**[CSS.md](./CSS.md)**, the exhaustive reference; for the design rationale
behind the DOM/Events/rendering internals, see `docs/INTERACTIVE.md`,
`docs/RENDERING.md`, `docs/REPAINT.md`, and `docs/SCROLLING.md`.

---

## HTML

### At a Glance

- **Parsing** uses `golang.org/x/net/html`, a real HTML5 tree-construction
  implementation — tag-soup recovery, implied tags, auto-closing, and
  foster parenting all work the way a browser's parser would, not a
  regex/best-effort approximation.
- **Structure:** headings, paragraphs, lists (`ul`/`ol`/`li`/`dl`/`dt`/`dd`,
  including nesting), blockquotes, tables (`thead`/`tbody`/`tfoot`/`tr`/
  `th`/`td`/`colgroup`/`col`/`caption`, `colspan`/`rowspan`), forms
  (`form`/`fieldset`/`legend`/`label`/`input`/`button`/`textarea`/`select`/
  `option`/`optgroup`), HTML5 sectioning (`section`/`article`/`aside`/`header`/
  `footer`/`main`/`nav`/`hgroup`/`search`), inline text-level semantics
  (`a`/`span`/`strong`/`em`/`b`/`i`/`u`/`s`/`del`/`ins`/`mark`/`small`/
  `sub`/`sup`/`code`/`kbd`/`samp`/`var`/`cite`/`dfn`/`abbr`/`q`), disclosure
  (`details`/`summary`), `figure`/`figcaption`, `address`, `hr`/`br`.
- **Global attributes:** `hidden` and `aria-hidden="true"` both hide an
  element and its subtree (via the UA stylesheet's `display: none`).
- Unrecognized elements, attributes, comments, and doctypes are silently
  ignored rather than raising an error — see "Not Supported" below for what
  falls in that bucket.

### Deviations from Spec

- **Form controls are attribute-driven, not stateful.** `<input>`'s
  `value`/`checked`/`type` attributes (and `<select>`'s selected `<option>`)
  are the only source of truth — `Element.Value()`/`SetValue()`/
  `Checked()`/`SetChecked()` read and write those same attributes directly.
  There's no separate "live DOM property vs. reflected HTML attribute"
  distinction the way real browsers maintain for form controls.
- **Typing has no cursor.** `DispatchKey` on a focused text `<input>`/
  `<textarea>` always appends to, or trims from, the *end* of `value` —
  there's no caret position, no text selection, and no Home/End/ArrowLeft/
  ArrowRight-driven insertion point within the field itself (those arrow
  keys are reserved for select-popup/scroll navigation instead — see
  `docs/SELECT.md`).
- **`<noscript>` content always renders** — there's no scripting engine to
  disable it for, so (unlike a browser, which only shows `noscript` content
  when JavaScript is off) it's unconditionally treated as regular markup.
- **`<details>`/`<summary>` always render fully expanded** — no collapse/
  expand interactivity exists; a real browser's native disclosure widget
  has no terminal equivalent here.

### Terminal-Native Additions

- **`img::before { content: attr(alt) }`** (UA stylesheet default) — since
  no image can ever actually render, alt text is shown inline directly,
  rather than in a broken-image icon's box the way a browser shows it.
- **`abbr[title]::after`** appends the title attribute's expansion inline —
  there's no hover-tooltip concept in a terminal to show it in instead.
- **`<hr>`** renders as a text rule line via `border-top`, not a pixel-drawn
  line.

### Not Supported

- **`<script>`, `<canvas>`, `<video>`, `<audio>`, `<iframe>`, `<embed>`,
  `<object>`, `<svg>`** — no scripting, media playback, embedding, or
  vector-graphics rendering exists. `<script>`/`<meta>`/`<link>` content is
  explicitly skipped during rendering; other unhandled elements aren't
  specifically stripped, they just fall back to generic inline treatment of
  whatever text content they happen to contain (usually none).
- **The legacy `width` HTML attribute** on table cells/columns — ignored in
  favor of CSS `width`; in real-world markup (especially HTML email) it's
  almost always a pixel value with no reliable pixel-to-column conversion.

---

## CSS

### At a Glance

- **Selectors:** universal, element, class (including multiple classes),
  ID, all attribute operators (`[attr]`, `=`, `~=`, `|=`, `^=`, `$=`, `*=`),
  descendant/child/adjacent-sibling/general-sibling combinators, comma
  groups, the full `:nth-*` family (`:nth-child`, `:nth-last-child`,
  `:nth-of-type`, `:nth-last-of-type`, full `An+B` syntax),
  `:first/last/only-child`, `:first/last/only-of-type`, `:empty`, `:not()`,
  `:is()`, `:where()`, `:checked`, `:disabled`, `:required`, `:focus`.
  Specificity and `!important` both work per spec.
- **Cascade keywords:** `inherit`, `unset`, and `initial` are recognized on
  any property, not just the usual inheritable set — see CSS.md's
  Inheritance section.
- **Custom properties:** `--name: value;` declarations and `var(--name)`/
  `var(--name, fallback)` usage, including unconditional by-name
  inheritance and pseudo-element (`::before`/`::after`/etc.) `content`
  values — see CSS.md's "Custom Properties (Variables)" section and
  `docs/proposals/VARIABLES.md` for the design. `calc()` and other CSS math
  functions are not implemented (see "Not Supported" below).
- **Colors:** every CSS Color Level 4 format — hex (3/4/6/8-digit), named
  colors (full W3C list), `rgb()`/`rgba()`, `hsl()`/`hsla()`, `hwb()` —
  downsampled automatically to the terminal's actual color capability
  (TrueColor/256/16/none).
- **Box model:** `margin`/`padding`/`border` (shorthand and per-side
  longhands, including logical `*-block-*`/`*-inline-*` aliases),
  `width`/`min-width`/`max-width`, `height`/`min-height`/`max-height`,
  percentage and `ch` sizing, auto margins.
- **Text:** `color`, `background-color`, `font-weight`, `font-style`,
  `text-decoration`, `text-align`, `text-indent`, `text-transform` (including
  Unicode super/subscript), `font-variant: small-caps`, `white-space` (all
  five values), `overflow-wrap`/`word-break`, `tab-size`, `visibility`,
  `opacity`.
- **Wide characters:** column widths (wrapping, alignment, padding, table
  sizing) account for double-width CJK and emoji glyphs via
  `go-runewidth`, not a naive one-rune-one-column assumption.
- **Generated content:** `::before`/`::after` with `content` (quoted
  strings, `attr()`, `counter()`/`counters()`, open/close-quote), CSS
  counters (`counter-reset`/`counter-increment`), `quotes`.
- **Lists:** `list-style-type` (all standard numbering systems plus custom
  string/`symbols()` bullets), `list-style-position`, nested lists, `<ol
  start>`.
- **Tables:** column sizing (fixed/percentage/min/max), multi-line cell
  wrapping, `vertical-align`, 6 named border-style presets, per-edge border
  and corner-glyph overrides, `colspan`/`rowspan`, `<colgroup>`/`<col>`,
  `caption-side`, `padding`/`margin` on `<table>` itself (surprisingly,
  given real browsers rarely use it). A bare `<table>` with no CSS renders
  completely borderless, matching a real browser. Both real
  `border-collapse` modes are supported: `separate` (the default) gives
  every `th`/`td` its own independent border box plus `border-spacing`;
  `collapse` merges adjacent cell/table borders into shared lines via real
  conflict resolution and junction-glyph synthesis — see `docs/TABLES.md`.
- **Flexbox:** currently limited to a single row/column of items —
  `flex-direction`, `justify-content`, `align-items`/`align-self`, `order`,
  `gap`, `flex-grow`, `flex-shrink`, `flex-basis` (all four working in
  `column` direction too, once the container has an explicit `height`), the
  `flex` shorthand, and (row direction only) `flex-wrap`/`align-content`/
  `margin: auto` (main-axis only). See CSS.md's Flexbox section for the
  (sizeable) list of real-Flexbox features not yet implemented here.
- **Forms and interactivity:** `<input>`/`<button>`/`<textarea>`/`<select>`
  (see `docs/SELECT.md`), scrolling (`overflow: auto|scroll`, see
  `docs/SCROLLING.md`/`docs/SCROLLBARS.md`).
- **`position: relative/absolute/fixed`** with `top`/`right`/`bottom`/`left`
  offsets and `z-index`. `relative` keeps its normal-flow layout space but
  paints visually shifted; `absolute`/`fixed` are removed from flow
  entirely and positioned against a containing block (the nearest
  positioned ancestor, or the whole document for `fixed`/an
  ancestor-less `absolute`). All three update their hit-test `Rect` to
  match what's painted. `sticky` is not implemented — see "Not Supported"
  below.

### Deviations from Spec

- **All sizing is in character cells/lines, not pixels.** `width`/`height`/
  `margin`/`padding`/border thickness are all integer counts (or `ch`/`%`).
  `px`, `em`, `rem`, `vw`, `vh` are parsed as unsupported and ignored
  entirely, not converted.
- **`opacity`** can't alpha-composite against an unknown terminal
  background, so fractional values darken the foreground/background color
  channels toward black instead; `opacity: 0` blanks content to spaces
  (still occupying its layout box, matching spec) rather than true
  transparency.
- **`border-width`/`border-*-width`** parse without error but are always a
  no-op — a box-drawing character has no notion of line thickness separate
  from the glyph itself. Use `border-style: heavy` or a custom glyph
  instead.
- **`border-style` values don't match real CSS's keyword set.** Real CSS
  has `solid`/`dashed`/`dotted`/`double`/`groove`/`ridge`/`inset`/`outset`/
  `none`/`hidden`; htmlterm's are named ASCII-art presets instead —
  `solid`/`rounded`/`heavy`/`double`/`markdown`/`hidden`/`none` (only
  `solid`/`double`/`none`/`hidden` overlap in name with real CSS, and even
  those pick a specific box-drawing character set rather than a line style).
  `heavy` is not named `thick`, avoiding a collision with real CSS's
  `border-width: thick` keyword.
- **The `border`/`border-<edge>` shorthand is matched positionally, not by
  CSS value type** — real CSS's border shorthand allows `<width>`/
  `<style>`/`<color>` in any order, and a real width keyword like `thick`
  can't be distinguished from a style keyword by content alone once it's in
  an unexpected slot. A consequence: the real-CSS two-value `<width>
  <style>` form (no color) is indistinguishable from this engine's
  `<style> <color>` form and is silently dropped — use the three-value form
  or set `border-style` directly.
- **`border-left: none`/`border-right: none` keep that side's corners** even
  though the vertical rule itself is gone (`┌────┐` with no `│` down the
  left side, not `────┐`) — the same convention any block element's border
  already uses; `<table>`'s own border unifies with that shared machinery
  (see `docs/TABLES.md`).
- **`:hover` has no real pointer-hover meaning** — the only place it
  matches anything is `option:hover` inside an open `<select>` popup,
  repurposed to mean "the arrow-key-highlighted option" (see
  `docs/SELECT.md`). **`:focus`** likewise matches a synthetic marker
  `Element.Focus` sets, not real window/pointer focus, and only means
  anything against a live `Document`, not one-shot `Renderer.Render`.
- **`text-transform: superscript`/`subscript`** substitutes each character
  for its Unicode superscript/subscript code point where one exists
  (there's no real script/font rendering) — characters with no Unicode
  equivalent pass through unchanged. **`font-variant: small-caps`**
  similarly just uppercases everything; terminals can't render true small
  caps.
- **`background` shorthand** only extracts the color component — `image`,
  `repeat`, `attachment`, `position`, `size`, `origin`, and `clip` tokens
  are recognized as present (so they don't break parsing) but otherwise
  ignored.
- **Bare ANSI color index numbers** (e.g. a raw `"214"`) are not accepted
  as a color value, even though they'd be meaningful to this renderer
  specifically — use `#rrggbb` or a named color and let automatic
  downsampling handle the terminal's actual palette.
- **`list-style-type`/`symbols()` implement only a small slice of the real
  spec's predefined counter styles** — `disc`/`circle`/`square`/`decimal`/
  `lower-alpha`/`upper-alpha`/`lower-roman`/`upper-roman`/a quoted
  string/`symbols()`, versus the full
  [CSS Counter Styles](https://developer.mozilla.org/en-US/docs/Web/CSS/Reference/Properties/list-style-type)
  list (`armenian`, `georgian`, the CJK/Japanese/Korean variants, `hebrew`,
  `devanagari`, `disclosure-open`/`disclosure-closed`, etc. — see "Not
  Supported" below). `symbols()` itself is a real CSS Counter Styles
  function, just without its `<symbols-type>` keyword or image arguments.

### Terminal-Native Additions

- **Literal-glyph border values** — `border-left: "▌"`, `border-top: "═"` —
  a quoted string sets that exact character directly, this engine's
  original (and still primary) way to use arbitrary box-drawing characters
  that have no named-preset equivalent.
- **`border-*-corner`** — literal corner-glyph overrides for any bordered
  box; real CSS has no per-corner styling concept at all. **`border-style`**
  naming a *complete glyph-set preset* (`solid`/`rounded`/`heavy`/`double`/
  `markdown`/`standard`/`hidden`/`none`) rather than a real-CSS line-style
  keyword is the same kind of addition — see `docs/TABLES.md` for examples
  of every preset, and its "`border-collapse: collapse`" section for how
  htmlterm ranks these presets against each other when adjacent
  cells/tables disagree (a conflict-resolution model real CSS also has, but
  keyed to its own line-style vocabulary, not this one).
- **`border-*-junction`** (`border-top-junction`/`-right`/`-bottom`/
  `-left`/`-center`, plus the 4 corner-shape variants) — a literal-glyph
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
  `::scrollbar-cap-start`/`::scrollbar-cap-end`** — real CSS has no
  standardized scrollbar-styling API (only nonstandard, prefixed
  `::-webkit-scrollbar-*` in Chromium); this is htmlterm's own equivalent,
  including clickable cap buttons with no real precedent at all. See
  `docs/SCROLLBARS.md`.
- **OSC 8 terminal hyperlinks** for `<a href>` — an actual terminal escape
  sequence, emitted automatically, not a CSS feature at all.
- **A plain `optgroup` selector** styles an open `<select>` popup's
  `<optgroup label>` header row (`background-color`/`color`) — real CSS has
  essentially no `<optgroup>` styling hook at all (browsers just bold+indent
  the label via their own UA stylesheet, not something an author can
  restyle); this is the same kind of repurposing `option:hover` already is.
  See `docs/SELECT.md`.

### Not Supported

- **CSS units:** `px`, `em`, `rem`, `vw`, `vh`, and friends (ignored; use
  bare integers, `ch`, or `%`).
- **CSS math:** `calc()`, `min()`, `max()`, `clamp()`. (Custom properties —
  `--foo`/`var()` — *are* supported; see the CSS "At a Glance" entry below
  and CSS.md's "Custom Properties (Variables)" section.)
- **At-rules:** `@media`, `@font-face`, `@keyframes`, `@import`,
  `@charset`, `@supports`, `@page`, `@counter-style` (no custom counter
  styles), etc. — the parser recognizes any `@`-rule and skips it as a unit
  rather than erroring or corrupting the rest of the stylesheet, but none
  of them do anything.
- **Pseudo-classes/elements beyond the supported list** — notably
  `:active` and any real mouse-hover semantics.
- **`revert`** (the fourth CSS-wide cascade keyword, alongside `inherit`/
  `unset`/`initial`, which are supported) — reverting to the user-agent
  stylesheet's value requires distinguishing UA-stylesheet origin from
  author-stylesheet origin, which this cascade doesn't model.
  `inherit`/`unset`/`initial` are also not supported inside
  `::before`/`::after`/`::marker`/scrollbar pseudo-element declarations,
  which resolve inheritance through a separate mechanism.
- **Layout models:** `display: grid`, `display: list-item`, any other
  `display` value beyond `block`/`inline`/`inline-block`/`flex`/
  `inline-flex`/`table`/`contents`/`none`; `float`/`clear`.
- **Positioned layout:** `position: sticky` (`relative`/`absolute`/`fixed`
  and `z-index` are all supported — see "At a Glance" above). For
  `absolute`/`fixed`: real CSS's shrink-to-fit auto-sizing (an
  unconstrained element here stretches to its containing block's width
  instead) and the "static position" algorithm (an axis with neither side
  set defaults to the containing block's top-left corner, not roughly
  where the element would have flowed) are not implemented; neither is
  `absolute`/`fixed` set directly on a `<tr>`/`<td>`/`<th>`/`<li>` itself
  (as opposed to content nested inside one). `<select>`'s dropdown remains
  the only overlay driven by dedicated Go code (`select_popup.go`) rather
  than routed through the general `position` mechanism — the underlying
  compositing primitives (`spliceColumns`, shifting a sub-rendered box's
  positions by an offset) are shared by both, but `<select>`'s popup
  predates and isn't (yet) reimplemented on top of `position`. See
  `docs/RENDERING.md`.
- **Visual effects:** `box-shadow`, gradients, `background-image`,
  `transform`, `transition`/`animation`, `filter`.
- **Flexbox gaps:** `flex-wrap`/`align-content` in `column` direction,
  `flex-wrap: wrap-reverse`, `align-content: stretch` (approximated as
  `flex-start`), `baseline` alignment, `margin: auto` beyond row direction's
  main axis (row direction's `margin-left`/`margin-right: auto` is
  supported). `flex-grow`/`flex-shrink`/`justify-content` now work in
  `column` direction too, but only once the container has an explicit CSS
  `height` — this engine has no other notion of a column flex container's
  main-axis size (row direction's width is always definite, so it never
  needed this condition) — see CSS.md's Flexbox section for the full
  reasoning per gap.
- **Table gaps:** `border-collapse: collapse`'s conflict resolution doesn't
  consult `tr`/`thead`/`tbody`/`tfoot` `border` (`col`/`colgroup` are
  consulted, via real conflict resolution against their column's cells). The
  legacy HTML `border`/`cellpadding`/`cellspacing` presentational attributes
  on
  `<table>` aren't read either (use the CSS equivalents). Multi-line cell
  content combined with `white-space: nowrap` remains unsupported under
  either border model. See `docs/TABLES.md`.
- **List gaps:** `list-style-image`; most of the real spec's predefined
  `list-style-type` counter styles — `armenian`/`lower-armenian`/
  `upper-armenian`, `georgian`, the CJK/Japanese/Korean variants
  (`cjk-decimal`, `cjk-ideographic`, `japanese-formal`/`informal`,
  `korean-hangul-formal`, etc.), `hebrew`, `devanagari` and the other
  script-specific systems, `disclosure-open`/`disclosure-closed`,
  `decimal-leading-zero`, `lower-greek` — only the small Western subset
  listed above under "At a Glance" is implemented.
- **`<select>` gaps:** per-`<option>` (and per-group-label-row) border/
  padding/width, `<optgroup>` nested inside another `<optgroup>` — see
  `docs/SELECT.md`.
- **`font-size`** — there is no concept of font size at all; terminal
  glyphs are a fixed cell size.

---

## DOM & Events

### At a Glance

- **`Document`/`Element`:** `ParseDocument`, `GetElementByID`,
  `QuerySelector`/`QuerySelectorAll`, attribute get/set/remove, `ClassList`,
  `Style` (inline `style=""` get/set/remove — see "Deviations from Spec"
  below), `Value`/`SetValue`, `Checked`/`SetChecked` — parse once, mutate
  and re-render repeatedly, instead of `Renderer.Render`'s
  parse-once-discard model.
- **Events:** `AddEventListener`/`RemoveEventListener` with capture/target/
  bubble dispatch order, `StopPropagation`/`StopImmediatePropagation`/
  `PreventDefault`/`DefaultPrevented` — `"click"`, `"keydown"`, `"input"`,
  `"focus"`, `"blur"`, `"submit"`, `"change"`, `"resize"`, `"paste"`, `"cut"`
  event types, each with real default actions (checkbox/radio toggle, focus
  traversal, text entry, implicit form submit on Enter, clipboard-text
  insertion/removal). A focused text `<input>`/`<textarea>` fires `"input"`
  on every keystroke that mutates its value and `"change"` once on commit
  (Enter, or losing focus) if the value actually changed — matching real
  DOM's `input`-vs-`change` distinction.
- **Clipboard:** `Document.DispatchPaste(text)`/`DispatchCut()` dispatch
  `"paste"`/`"cut"` with `Event.ClipboardData` carrying the plain-text
  payload (readable/rewritable by a listener, mirroring
  `clipboardData.getData`/`setData`), and — unless prevented — insert into or
  clear a focused text-like `<input>`/`<textarea>`'s value. `Loop` wires
  these to a real terminal's bracketed paste and Ctrl-X, handing `DispatchCut`'s
  returned text to the system clipboard via `tcell.Screen.SetClipboard`
  (OSC 52) when the terminal advertises clipboard support.
- **Focus:** `Element.Focus`/`Blur`, `Document.FocusNext`/`FocusPrev`/
  `FocusedElement`, matching `:focus` in CSS. **`tabindex`** is read on any
  element (not just native form controls) via the plain HTML attribute —
  `tabindex="0"` makes an otherwise non-focusable element (e.g. `<div>`, or
  `<a href>`, which is never a tab stop without it — see "Deviations from
  Spec" below) a real tab stop; a positive value front-loads it into Tab
  order ahead of every `0`/default element, ascending by value; `tabindex="-1"`
  is reachable via `Focus()`/click but skipped by `FocusNext`/`FocusPrev`.
  No typed `Element.TabIndex()` accessor exists — use
  `GetAttribute`/`SetAttribute("tabindex", ...)` directly (see
  `docs/DOM_API.md`).
- **Hit-testing:** `Element.Rect()` returns the on-screen box (row/column/
  width/height in terminal cells) as a byproduct of rendering, for
  translating real input coordinates into `DispatchClick` calls.
- **Scrolling:** `Element.ScrollTop`/`SetScrollTop` (and their horizontal
  counterparts `ScrollLeft`/`SetScrollLeft`), `Document.DispatchWheel`,
  `PageUp`/`PageDown`/arrow-key scrolling on a focused descendant.
  `Element.Focus` auto-scrolls a focused descendant into view on both axes
  if it falls outside a scrollable ancestor's visible row or column range.
- **`Loop`:** drives a `Document` against a real terminal — raw mode, SGR
  mouse decoding, `SetInterval`/`SetTimeout` timers, repaint on every
  event/timer/resize.

### Deviations from Spec

- **Coordinates are terminal cells, not pixels.** `DispatchClick(row, col)`
  hit-tests against `Element.Rect()`'s row/column grid, not a
  `MouseEvent`'s `clientX`/`clientY`.
- **`Event` carries less than a real DOM event, but does carry modifier
  keys.** `ShiftKey`/`CtrlKey`/`AltKey`/`MetaKey` are set from the
  `Modifiers` passed to `DispatchClick`/`DispatchKey` (zero/false for event
  types with no real modifier-key concept — `"resize"`, `"submit"`,
  `"focus"`, `"blur"`, `"change"` — matching real DOM's UIEvent-only
  modifier fields). Still missing: `relatedTarget`, `button`, `detail`
  (click count). `Event.Key` is a single UTF-8 rune or one name from a fixed
  vocabulary (`"Enter"`, `"Backspace"`, `"Tab"`, `"Escape"`, arrow keys) —
  the host translates raw terminal input into these strings (and raw
  terminal modifier bits into `document.Modifiers`) itself; htmlterm never
  reads a terminal directly outside of `Loop`.
- **Only one click "kind" exists** — there's no `mousedown`/`mouseup`/
  `dblclick`/`contextmenu`/drag events, just a single synthesized
  `"click"` that hit-tests and runs default actions atomically.
- **`"cut"`/`"paste"` always act on a whole field, never a selection** —
  there's no text-selection/caret-range concept narrower than "the focused
  text entry's entire value" (see "Not Supported" below), so `DispatchCut`
  always clears the whole value rather than removing a selected range, and
  `DispatchPaste` always appends at the end rather than inserting at a caret
  position. `Event.ClipboardData` is a plain string, not a real
  `DataTransfer`/`clipboardData` object with MIME types.
- **`DispatchWheel` mutates scroll position directly** and returns whether
  anything scrolled — unlike every other `Dispatch*` method, it does not
  dispatch a `"wheel"` `Event` a listener could observe or prevent. It
  takes both a `deltaX` and `deltaY` (mirroring a real `WheelEvent`),
  scrolling each axis's own nearest scrollable ancestor independently (they
  can be two different elements for a nested pane). `Loop` wires tcell's
  `WheelLeft`/`WheelRight` mouse buttons to `deltaX` directly, and remaps a
  Shift-held vertical wheel notch to `deltaX` too (the common browser/
  terminal fallback convention for input hardware that never reports a
  horizontal wheel bit at all).
- **Tab order defaults to a document-order walk** over form controls
  (`input`/`button`/`textarea`/`select`, skipping `disabled`) plus
  focusable scroll containers, reorderable via `tabindex` (see "At a
  Glance" above) — but **plain `<a>` links are never tab stops** unless
  given explicit `tabindex` (real browsers make links focusable by default
  with no attribute needed; htmlterm requires opting in, since link-dense
  rendered content — documentation pages, HTML email — would otherwise
  flood Tab order with every inline link ahead of a page's actual
  controls).
- **`Element.Style()` doesn't round-trip shorthand properties or preserve
  declaration order.** It mirrors `CSSStyleDeclaration` for the inline
  `style=""` attribute only (there's no `getComputedStyle` — see
  `docs/DOM_API.md`), but `SetProperty("margin", "1px")` is stored as
  `margin-top`/`-right`/`-bottom`/`-left` (so `GetPropertyValue("margin")`
  afterward returns `""`, not `"1px"`), and `CSSText`/`SetCSSText` serialize
  properties sorted by name rather than in original/set order — both
  because shorthand expansion happens at parse time with no record that a
  shorthand was used, the same limitation the cascade itself has.

### Terminal-Native Additions

- **Scroll containers as tab stops:** an `overflow-x`/`overflow-y`
  `auto|scroll` element with an explicit width/resolved height respectively,
  and no other focusable descendant, automatically becomes a keyboard tab
  stop, purely so `PageUp`/`PageDown`/arrow keys have something to scroll
  once focused.
- **Scrollbar cap buttons** (`::scrollbar-cap-start`/`-end`) are clickable
  via `DispatchClick`, but do *not* dispatch a `"click"` `Event` on the
  scrollable element — they're rendering chrome, not real element content
  (see `docs/SCROLLBARS.md`).
- **`<select>`'s dropdown popup** is a synthesized text overlay with its
  own synthetic `Rect` per `<option>`, composited on top of already-
  rendered output — real DOM has no equivalent to this text-based popup
  compositing step (see `docs/RENDERING.md`).

### Not Supported

- **`mousemove`, `mouseover`/`mouseout`, `dblclick`, `contextmenu`,
  drag-and-drop events** — no continuous hover tracking exists in a
  terminal, and none of these are wired up.
- **Text selection and `"select"`/`"copy"` events.** There's no
  click-drag/keyboard text-selection model at all (a real terminal's own
  mouse-drag selection is unavailable once `Loop.Run` enables mouse
  reporting — held-modifier drag, e.g. Option on macOS Terminal.app/iTerm2
  or Shift on many Linux terminals/Windows Terminal, bypasses the app and
  falls back to native OS selection/copy). `"cut"`/`"paste"` are supported
  (see "Clipboard" above) but always act on a whole field, not a selected
  range, since there's no selection to scope them to.
- **Custom events / arbitrary `dispatchEvent`** — only the fixed built-in
  event names above are ever dispatched.
- **Shadow DOM, custom elements, `MutationObserver`** — there is no tree-
  change observation API; a host must re-render after mutating.
- **Windows.** `Loop`'s automatic resize tracking requires `syscall.SIGWINCH`,
  which doesn't exist on Windows at all — a compile-time constraint there,
  not just an unverified one. The rest of the interactive layer (raw
  terminal mode via `golang.org/x/term`) is POSIX-oriented and hasn't been
  verified on Windows either.
- **Concurrent access.** A `Document` has no internal locking — `Loop.Run`'s
  own goroutine is the only place that may ever mutate a `Document` or
  trigger a repaint; timer callbacks and dispatched event handlers all run
  there too. There's no DOM-level concurrency model to compare this against
  (browsers are single-threaded per document too), but it's worth stating
  plainly for Go callers: calling `Document` methods from multiple
  goroutines is not safe.

---

## See Also

- **CSS.md** — the exhaustive property-by-property reference.
- **docs/DOM_API.md** — a field-by-field table comparing the real DOM's
  `Document`/`Element`/`Node` interfaces against `document.Document`/
  `document.Element`, method by method.
- **docs/SELECT.md** — `<select>` popup styling in full.
- **docs/SCROLLBARS.md** — scrollbar gutter styling in full.
- **docs/TABLES.md** — table border/margin/padding styling in full,
  including rendered examples of every `border-style` preset.
- **docs/SCROLLING.md** — the scrolling/viewport design.
- **docs/INTERACTIVE.md** — the `Document`/`Element`/events design history.
- **SECURITY.md** — a different axis from this doc (safety against
  untrusted HTML/CSS, not fidelity to spec): terminal escape-sequence
  stripping, and `StripHiddenInline`/`IgnoreDocumentCSS` for content a
  browser would render invisibly but htmlterm can remove structurally.
- **docs/RENDERING.md** — the rendering engine's own internals, including
  why popups (`<select>`'s dropdown) are the one exception to "no
  positioned layout."
