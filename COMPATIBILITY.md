# htmlterm Compatibility Notes

htmlterm reinterprets three separate web-platform surfaces for a fixed-size
character grid instead of a browser: **HTML** (parsing and per-element
rendering), **CSS** (selectors, cascade, properties), and **DOM & Events**
(the mutable `Document`/`Element` API and its native-Go event model).

This document is about the *gap* between htmlterm and the real spec, not a
feature list — for what's actually supported, see **[CSS.md](./CSS.md)**
(the exhaustive CSS/selector/element reference) and
**[docs/DOM_API.md](./docs/DOM_API.md)** (a method-by-method table against
the real `Document`/`Element`/`Node` interfaces). Duplicating "what's
supported" here as prose would just drift out of sync with those as features
land, so each surface below gets three sections instead:

- **Deviations from Spec** — real features that exist here but behave
  differently than in a browser, and why (text cells, not pixels; no
  scripting engine; no real pointer/window).
- **Terminal-Native Additions** — things invented for this renderer with no
  browser equivalent at all.
- **Not Supported** — real features that simply aren't implemented.

For the design rationale behind the DOM/Events/rendering internals, see
`docs/INTERACTIVE.md`, `docs/RENDERING.md`, `docs/REPAINT.md`, and
`docs/SCROLLING.md`.

---

## HTML

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
- **An indeterminate `<progress>` (no `value` attribute) renders statically,
  not animated.** Real browsers show an animated barber-pole; this renderer
  has no `animation` support at all, so `:indeterminate` instead shows
  `::progress-bar`'s track glyph repeated across the full width, with no
  fill — see `docs/PROGRESS_METER.md`.

### Terminal-Native Additions

- **`img::before { content: attr(alt) }`** (UA stylesheet default) — since
  no image can ever actually render, alt text is shown inline directly,
  rather than in a broken-image icon's box the way a browser shows it.
- **`abbr[title]::after`** appends the title attribute's expansion inline —
  there's no hover-tooltip concept in a terminal to show it in instead.
- **`<hr>`** renders as a text rule line via `border-top`, not a pixel-drawn
  line.
- **`progress-style`/`meter-style` and `::progress-bar`/`::progress-value`/
  `::meter-bar`/`::meter-optimum-value`/`::meter-suboptimum-value`/
  `::meter-even-less-good-value`** — real CSS has no author-facing hook to
  restyle `<progress>`/`<meter>`'s bar glyphs at all (WebKit's own
  `-webkit-`-prefixed pseudo-elements this borrows names from are
  non-standard); `progress-style`/`meter-style` reuse `scrollbar-style`'s
  exact five preset names and merge mechanism — see `docs/PROGRESS_METER.md`.

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
- **`<dialog>`/`<datalist>`** — no native-modal or autocomplete-popup
  equivalent exists; unhandled the same way any other unrecognized element
  is (generic inline fallback, usually rendering no visible content).
- **`contenteditable`** — the attribute isn't read anywhere; there's no
  editable-content model outside a real `<input>`/`<textarea>` (see "Form
  controls are attribute-driven" above).

---

## CSS

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
  `--foo`/`var()` — *are* supported; see CSS.md's "Custom Properties
  (Variables)" section.)
- **At-rules:** `@media`, `@font-face`, `@keyframes`, `@import`,
  `@charset`, `@supports`, `@page`, `@counter-style` (no custom counter
  styles), etc. — the parser recognizes any `@`-rule and skips it as a unit
  rather than erroring or corrupting the rest of the stylesheet, but none
  of them do anything. `@media`/`@container`-style responsive breakpoints
  (width, `prefers-color-scheme`) are a deliberate scope decision, not an
  oversight — `internal/cssengine.ParseStylesheet` is a flat, non-nesting
  lexer that can't represent a braced at-rule body at all, and the intended
  answer is for a host (typically `tui.Loop`) to pick which pre-authored
  stylesheet string to hand `Options.Stylesheets` in response to a resize
  or theme-change event, not for `cssengine` to evaluate conditions itself.
  See `docs/proposals/RESPONSIVE.md`.
- **Pseudo-classes/elements beyond the supported list** — notably
  `:active`, and any real mouse-hover semantics.
- **`revert`** (the fourth CSS-wide cascade keyword, alongside `inherit`/
  `unset`/`initial`, which are supported) — reverting to the user-agent
  stylesheet's value requires distinguishing UA-stylesheet origin from
  author-stylesheet origin, which this cascade doesn't model.
  `inherit`/`unset`/`initial` are also not supported inside
  `::before`/`::after`/`::marker`/scrollbar pseudo-element declarations,
  which resolve inheritance through a separate mechanism.
- **Layout models:** `display: grid`, `display: list-item`, any other
  `display` value beyond `block`/`inline`/`inline-block`/`flex`/
  `inline-flex`/`table`/`contents`/`none` — including the standalone
  CSS-table display values (`table-row`/`table-cell`/`table-row-group`/
  etc.) that let a non-`<table>` element opt into table layout; only a
  literal `<table>`/`<tr>`/`<td>`/`<th>` element tree gets table treatment
  here; `float`/`clear`.
- **`box-sizing`** — parses without error but is always a no-op; there's no
  `content-box`/`border-box` distinction because htmlterm's box model
  always behaves like `border-box` (padding/border are subtracted from a
  declared `width`/`height`, not added on top of it).
- **`line-height`, `letter-spacing`, `word-spacing`** — no per-line vertical
  spacing or extra inter-character/inter-word spacing concept exists; a
  terminal cell grid has no sub-line leading to adjust and no fractional
  character-width budget to insert gaps into.
- **`outline`/`outline-offset`** — no separate box-decoration layer outside
  the border exists to draw one on; use `border`/`border-style` directly
  (including on `:focus`) for a terminal-native focus ring instead.
- **`direction`/`unicode-bidi`, and the `dir` HTML attribute** — no
  bidirectional text algorithm is implemented; RTL scripts render in
  logical (source) order, not visually reordered, and `dir="rtl"`/`dir="auto"`
  have no effect on layout.
- **Logical inset properties** (`inset-inline-start`/`-end`,
  `inset-block-start`/`-end`) for `position` offsets — only the physical
  `top`/`right`/`bottom`/`left` longhands are read (unlike margin/padding/
  border, which do support the logical `*-block-*`/`*-inline-*` aliases).
- **Positioned layout:** `position: sticky` (`relative`/`absolute`/`fixed`
  and `z-index` are all supported — see CSS.md). For
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
  `decimal-leading-zero`, `lower-greek` — only the small Western subset in
  CSS.md's `list-style-type` entry is implemented.
- **`<select>` gaps:** per-`<option>` (and per-group-label-row) border/
  padding/width, `<optgroup>` nested inside another `<optgroup>` — see
  `docs/SELECT.md`.
- **`font-size`** — there is no concept of font size at all; terminal
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
  focusable scroll containers, reorderable via `tabindex` (see
  `docs/DOM_API.md`) — but **plain `<a>` links are never tab stops** unless
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
- **`"focus"`/`"blur"` bubble here, unlike spec.** Every dispatched event
  runs the same capture/target/bubble chain (`runDispatch` in `event.go`
  has no per-type bubbling suppression) — real DOM's `focus`/`blur` never
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
  not in the shared dispatch engine `DispatchEvent` also uses — a real,
  named gap versus spec, not spec ambiguity.

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
- **`Event` is one flat struct, not a class hierarchy.** `NewCustomEvent`/
  `Element.DispatchEvent` support custom events with an untyped
  `Detail any` payload, `Bubbles`, and `Cancelable` — real DOM instead has
  `Event`/`CustomEvent`/`UIEvent`/`MouseEvent`/`KeyboardEvent` as separate
  constructors/types in a class hierarchy. htmlterm reuses the same `Event`
  type (with `ClipboardData`/`Key`/modifier fields already sitting unused
  alongside `Detail` for a custom event) for every event, built-in or
  custom, matching this package's existing "one struct, not subclasses"
  style.

### Not Supported

- **`mousemove`, `mouseover`/`mouseout`, `dblclick`, `contextmenu`,
  drag-and-drop events** — no continuous hover tracking exists in a
  terminal, and none of these are wired up.
- **`keyup`/`keypress`** — only `"keydown"` is dispatched; there's no
  key-release event, so a host can't detect a key being held or released
  (only that a key was pressed once).
- **`focusin`/`focusout`** — there's no separately-named bubbling pair;
  see "`\"focus\"`/`\"blur\"` bubble here, unlike spec" under "Deviations
  from Spec" above for how to observe focus entering/leaving a descendant
  instead.
- **Text selection and `"select"`/`"copy"` events.** There's no
  click-drag/keyboard text-selection model at all (a real terminal's own
  mouse-drag selection is unavailable once `Loop.Run` enables mouse
  reporting — held-modifier drag, e.g. Option on macOS Terminal.app/iTerm2
  or Shift on many Linux terminals/Windows Terminal, bypasses the app and
  falls back to native OS selection/copy). `"cut"`/`"paste"` are supported
  (see "Clipboard" above) but always act on a whole field, not a selected
  range, since there's no selection to scope them to.
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
