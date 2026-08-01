# htmlterm CSS Reference

htmlterm renders a restricted subset of HTML+CSS to terminal strings.
This document lists every selector form, HTML element, and CSS property
that is recognized. Anything not listed here is silently ignored.

This is the exhaustive, property-by-property CSS reference. For an
orientation read instead — spanning HTML, CSS, and the DOM/Events API:
what's supported at a glance, what deviates from spec and why, what's a
terminal-native addition with no browser equivalent, and what's simply not
implemented — see **[COMPATIBILITY.md](./COMPATIBILITY.md)**.

---

## Case sensitivity

Everything CSS defines as ASCII case-insensitive is treated that way here, so
`DISPLAY: FLEX` and `display: flex` are the same declaration. That covers
property names, keyword values (and the unit on a length — `10CH` is `10ch`),
element names in selectors, pseudo-classes and pseudo-elements, and attribute
*names* in attribute selectors.

The things CSS defines as case-**sensitive** stay that way:

- **strings**, which are emitted verbatim — `content: "MiXeD"`, `quotes`,
  literal border glyphs (`border-left: "▌"`), custom bullets
  (`list-style-type: "→ "`, `symbols('Ab')`), custom truncation markers
  (`text-overflow: '…'`);
- **custom identifiers** — counter names, so `counter-reset: Xy` and
  `counter(xy)` refer to different counters;
- **custom properties** (`--Foo` and `--foo` are distinct properties, and their
  values are preserved exactly), including values substituted through `var()`;
- **attribute values** in selectors — `[type=TEXT]` does not match
  `type="text"`, matching CSS's default (this engine has no `i` flag);
- **class and ID** names, which HTML itself treats as case-sensitive.

---

## Selectors

| Form | Example |
|------|---------|
| Universal | `* { }`, `*.warn { }` |
| Element | `th { }` |
| Class | `.num { }` |
| Multiple classes | `.warn.big { }` |
| Element + class(es) | `tr.unseen { }`, `p.a.b { }` |
| ID | `#intro { }` |
| Element + ID | `h1#title { }` |
| Attribute presence | `a[href] { }` |
| Attribute value | `td[data-role=header] { }` |
| Attribute word contains | `p[data-tags~=beta] { }` |
| Attribute dash-prefix | `p[lang|=en] { }` |
| Attribute prefix | `a[href^=https://] { }` |
| Attribute suffix | `a[href$=.pdf] { }` |
| Attribute substring | `a[href*=example] { }` |
| Descendant (space) | `tr.unseen td { }` |
| Child (`>`) | `div > p { }` |
| Pseudo-class | `:root { }`, `li:first-child { }`, `tr:nth-child(odd) { }` |
| Pseudo-element | `p::before { content: "→ "; }`, `p::after { content: " ←"; }`, `::scrollbar-thumb { content: "█"; }` |
| Adjacent sibling (`+`) | `h2 + p { }` |
| General sibling (`~`) | `h2 ~ p { }` |
| Comma-separated (any of the above) | `h1, h2, h3 { }` |

**Specificity** follows CSS rules: ID = 100, class / pseudo-class / attribute = 10,
element / pseudo-element = 1, universal selector = 0. Higher specificity wins;
equal specificity last-write wins.

**`!important`** on a declaration (e.g. `color: red !important;`) lifts it into
a separate, higher-priority tier that always wins over any normal declaration,
regardless of specificity. Specificity (and last-write-wins for ties) still
applies *within* the `!important` tier, so an `!important` declaration with
higher specificity beats another `!important` declaration with lower
specificity. `!important` applies per declaration, not per rule, so a single
rule may mix `!important` and normal declarations. See also [Inline `style`
Attribute](#inline-style-attribute) for how `!important` interacts with inline
styles.

**Supported pseudo-classes:** `:root`, `:first-child`, `:last-child`,
`:only-child`, `:first-of-type`, `:last-of-type`, `:only-of-type`, `:empty`,
`:nth-child(<An+B>)`, `:nth-last-child(<An+B>)`, `:nth-of-type(<An+B>)`,
`:nth-last-of-type(<An+B>)`, `:not(<simple-selector>)`,
`:is(<selector-list>)`, `:where(<selector-list>)`, `:has(<relative-selector-list>)`,
`:checked`, `:disabled`, `:required`, `:indeterminate`, `:focus`, `:hover`. `:root` matches the document element
(`html` for parsed HTML documents/fragments). The `:nth-*` family accepts the
full CSS `An+B` micro-syntax (`odd`, `even`, `3`, `2n`, `2n+1`, `-n+3`, etc.),
matched against sibling position (`:nth-child`/`:nth-last-child`) or position
among same-tag siblings (`:nth-of-type`/`:nth-last-of-type`); counting is
1-based, and `:nth-last-*` counts from the last sibling. `:empty` matches an
element with no children other than possibly comments — a text node
(including whitespace-only) or any element child disqualifies it. `:not()`
accepts a single compound selector (element, universal selector, class, id,
attribute, or combinations thereof) as its argument; nested combinators
inside `:not()` are not supported. `:is()` and `:where()` accept a
comma-separated list of compound selectors (same restriction as `:not()` —
no nested combinators) and match if the element matches any selector in the
list, e.g. `:is(header, footer) p` or `p:where(.warn, #important)`. The two
are matching-equivalent; they differ only in specificity: `:is()` takes on
the specificity of its most specific argument (so `:is(#a, .b)` counts as an
ID selector), while `:where()` always contributes zero specificity
regardless of its argument — useful for writing override-friendly base
rules, e.g. in a user stylesheet layered under `Options.Stylesheets`.
`:has()` takes a comma-separated *relative-selector* list and, unlike
`:not()`/`:is()`/`:where()`, does support combinators inside it —
`:has(div > p)` and `:has(a, div > p)` are both valid. An argument item with
no leading combinator is implicitly a descendant relation (`:has(img)`
matches an element with an `img` descendant at any depth); one that starts
with `>`, `+`, or `~` matches against a direct child, the immediately
following sibling, or any later sibling, respectively (`:has(> p)`,
`:has(+ p)`, `:has(~ p)`). Specificity follows the same "most specific
argument wins" rule as `:is()`. `:indeterminate` matches a `<progress>`
element with no `value` attribute (the sole indeterminate trigger per spec —
an unparseable-but-present value is just clamped like any other bad numeric
attribute, not a second indeterminate case). Real spec's `:indeterminate`
also matches `<input type=checkbox>`/`<input type=radio>` in certain
JS-driven states; that slice is not supported here, since this renderer's
checkboxes are attribute-driven only, with no separate `indeterminate` IDL
property to reflect — see `docs/PROGRESS_METER.md`.
`:checked`/`:disabled`/`:required` match
the real HTML `checked`/`disabled`/`required` attributes' presence. There is
no `:read-only`/`:read-write` pair — the `readonly` attribute is honored by
the DOM layer (a readonly text entry takes no typing, paste, or cut; see
`COMPATIBILITY.md`) but can't be selected on. `:focus`
matches whichever element `Element.Focus` (see the package godoc's events
section) most recently marked focused; it has no meaning against
`Renderer.Render`'s one-shot rendering, only against a live `Document`.
`:hover` has no meaning for real pointer movement in a terminal; the only
place it matches anything is `option:hover` inside an open `<select>` popup
— see `docs/SELECT.md`.

**Supported pseudo-elements:** `::before`, `::after`, `::marker`, `::selection`,
`::scrollbar`, `::scrollbar-track`, `::scrollbar-thumb`, `::scrollbar-cap-start`,
`::scrollbar-cap-end`, and their horizontal-scrollbar counterparts
`::scrollbar-x`, `::scrollbar-track-x`, `::scrollbar-thumb-x`,
`::scrollbar-cap-start-x`, `::scrollbar-cap-end-x` (all also accepted with a
single colon), `::progress-bar`, `::progress-value`, `::meter-bar`,
`::meter-optimum-value`, `::meter-suboptimum-value`,
`::meter-even-less-good-value`. `::before`/`::after` inject inline text at the start or end of an
element's content; they require the `content` property. `::marker` styles the list
prefix (bullet or number) of an `<li>` element; supported properties are `color`,
`background-color`, `font-weight`, `font-style`, and `text-decoration`.
`::selection` styles the highlighted range of a focused `<input>`/`<textarea>`
with a non-collapsed caret selection; supported properties are `color`,
`background-color`, `font-weight`, `font-style`, and `text-decoration`
(mirroring `::marker`'s supported set) — with neither `color` nor
`background-color` set, it falls back to reverse video (the UA default
every terminal editor already uses), since there's no terminal-neutral
equivalent of a browser's platform selection color to default to instead.
See `docs/proposals/CARET_SELECTION.md`.
`::scrollbar`/`::scrollbar-track`/`::scrollbar-thumb`/`::scrollbar-cap-start`/
`::scrollbar-cap-end` style the vertical scrollbar gutter drawn by
`overflow-y: scroll`; the `-x`-suffixed names style the horizontal
counterpart drawn by `overflow-x: scroll` — see `docs/SCROLLBARS.md`.
`::progress-bar`/`::progress-value` style `<progress>`'s track/filled
portions; `::meter-bar`/`::meter-optimum-value`/`::meter-suboptimum-value`/
`::meter-even-less-good-value` style `<meter>`'s track/filled portions (the
filled glyph/style chosen per the value's resolved region) — see
`docs/PROGRESS_METER.md`.
All combinator and element-matching forms work: `div p::before`, `.warn::after`,
`li::marker`, `ul.fancy li::marker`, `#pane::scrollbar-thumb`,
`input::selection`, etc. A bare `::scrollbar-thumb { … }` or `::selection { … }`
(no element/class/id prefix) matches every scrollable element, or every text
entry, respectively, the same way a bare `::before` would match every
element.

**Supported attribute operators:** `[attr]` (presence), `[attr=val]` (exact
match), `[attr~=val]` (whitespace-separated word), `[attr|=val]` (exact value
or value followed by `-`), `[attr^=val]` (prefix), `[attr$=val]` (suffix), and
`[attr*=val]` (substring).

**Not supported:** `:active`, and other pseudo-classes beyond those
listed above.

---

## Inheritance

The following properties inherit from parent to child when no direct rule
applies to the child element:

`color` · `font-weight` · `font-style` · `font-variant` · `text-decoration` · `text-align` · `white-space` · `text-transform` · `overflow-wrap` · `word-break` · `text-indent` · `tab-size` · `visibility` · `opacity` · `quotes` · `list-style-position`

Inheritance is resolved by walking up the ancestor chain and taking the value
from the nearest ancestor that sets the property directly. For example,
`tr.unseen { font-weight: bold }` causes all `<td>` children to be bold
without needing `tr.unseen td { font-weight: bold }`.

To explicitly cancel an inherited value, set the property to its `normal` (or
`none`) reset on the child element, or use `unset`/`initial` (below) if the
property has no such reset keyword of its own.

### CSS-wide keywords: `inherit`, `unset`, `initial`

These are recognized on **any** property, not just the inheritable list
above:

- **`inherit`** — always take the value from the parent element's own
  resolved value for that property, regardless of whether the property is
  normally inheritable (e.g. `border-style: inherit` forces inheritance on a
  property that wouldn't otherwise propagate).
- **`unset`** — acts as `inherit` if the property is one of the inheritable
  properties listed above (custom properties — `--foo` — included, since
  they're unconditionally inheritable; see "Custom Properties (Variables)"
  below); otherwise acts as `initial`.
- **`initial`** — reverts the property to its own specified default,
  ignoring any rule that would otherwise apply, at any specificity. This is
  the one that answers "a broader rule set this property; how does a more
  specific selector cancel it without knowing what value to fall back to?"
  — e.g. `td { border-style: solid; } td.plain { border-style: initial; }`
  leaves `.plain` cells with no border at all, rather than needing to know
  and repeat whatever "no border" looks like.

`revert` (CSS's fourth cascade-reset keyword, which reverts to the
user-agent stylesheet specifically) is **not implemented** — it requires
distinguishing UA-stylesheet origin from author-stylesheet origin, which
this cascade doesn't model.

These keywords resolve against real elements (`Cascade.Resolve`); they are
**not** currently supported inside `::before`/`::after`/`::marker`/scrollbar
pseudo-element declarations, which use a separate inheritance mechanism (see
`internal/render/inline.go`'s `mergeInlineStyle`).

---

## Custom Properties (Variables)

`--name: value;` declares a custom property; `var(--name)` and
`var(--name, fallback)` read one back. Design rationale and implementation
notes live in `docs/proposals/VARIABLES.md`.

- **Case-sensitive**, unlike every other property name in this engine —
  `--Foo` and `--foo` are two distinct properties.
- **Inherit unconditionally, by name** — a descendant that never redeclares
  `--brand` at all still sees an ancestor's value, the same as any of the
  fixed inheritable properties above, except there's no fixed list: *every*
  `--*` name inherits. A closer declaration overrides a farther one, same as
  any other cascade.
- **`var(--name, fallback)`** — the fallback is used when `--name` is
  undefined anywhere in the tree (not just on the current element). Only the
  first top-level comma is syntactic; everything after it up to the closing
  `)` is fallback text verbatim, including further commas, nested `var()`
  calls, and quoted strings — e.g. `var(--a, rgb(0, 0, 0))` or
  `var(--a, var(--b, green))` both work as expected.
- **`!important`** on a `--name` declaration participates in the cascade
  exactly like any other property.
- **Cyclic or otherwise unresolvable references resolve to `""`**, not an
  error and not a hang (`--a: var(--b); --b: var(--a);` — both end up
  empty). This approximates real CSS's "guaranteed-invalid value" rather
  than implementing full invalid-at-computed-value-time
  fallback-to-inherited/initial semantics.
- **Works inside `::before`/`::after`/`::marker`/scrollbar pseudo-element
  declarations** — e.g. `::before { content: var(--icon, "> "); }` —
  unlike `inherit`/`unset`/`initial` (above), which pseudo-elements don't
  support at all. var() resolution for pseudo-elements is a plain
  substitution against the real element's already-resolved custom
  properties, not a second inheritance mechanism, which is why it doesn't
  share that limitation.

**Known gaps, both accepted limitations rather than bugs:**

- **Shorthand fan-out.** `expandShorthand` runs once at parse time, before
  any per-node `var()` resolution is possible. `margin: var(--gap) var(--gap)`
  (one `var()` per shorthand slot) works fine, since each token is opaque to
  the whitespace splitter. But `margin: var(--sides)` expecting
  `--sides: 1 2 3 4` to fan out into four independent sides does **not**
  work — the shorthand expander already collapsed to the single-token "all
  sides same" branch before the var had a value, so after substitution all
  four sides get the same literal string `"1 2 3 4"`. Same category of
  limitation as the existing two-token `border: <width> <style>` gap above.
- **`counter-reset`/`counter-increment` only see a `var()` reference to a
  custom property declared on the *same* element**, not one inherited from
  an ancestor — these are non-inherited properties resolved via
  `Cascade.Direct`, which (unlike `Resolve`) never walks ancestors at all,
  for any property. `counter-reset: var(--n)` works when `--n` is set on
  that same element; it silently no-ops if `--n` is only set on an
  ancestor.

---

## HTML Elements

| Element | Notes |
|---------|-------|
| `h1`–`h6` | Rendered as a single styled line |
| `p` | Inline content followed by a newline |
| `blockquote` | Inline content followed by a newline; default `border-left: "│"; border-left-color: #555555; padding-left: 1; padding-right: 2` |
| `a` | Hyperlink; `href` attribute becomes an OSC 8 terminal hyperlink (default: `text-decoration: underline`) |
| `span` | Inline styled text; at block level, followed by a newline |
| `s`, `del` | Inline strikethrough text (default: `text-decoration: line-through`) |
| `u`, `ins` | Inline underlined text (default: `text-decoration: underline`). `ins` represents inserted/added content. |
| `b` | Inline bold text (default: `font-weight: bold`; alias for `<strong>`) |
| `kbd` | Keyboard input (default: `font-weight: bold`) |
| `mark` | Highlighted text (default: `background-color: #cc9900; color: #000000`) |
| `samp` | Sample program output (default: `font-style: italic`) |
| `var` | Variable name (default: `font-style: italic`) |
| `cite` | Citation / title of work (default: `font-style: italic`) |
| `sup` | Superscript text (default: `text-transform: superscript`) |
| `sub` | Subscript text (default: `text-transform: subscript`) |
| `strong` | Inline bold |
| `em`, `i` | Inline italic (default: `font-style: italic`) |
| `dfn` | Definition term; inline italic (default: `font-style: italic`) |
| `abbr` | Abbreviation. The UA stylesheet provides `abbr[title]::after { content: " (" attr(title) ")"; }`, so the expansion is appended inline when `title` is present — e.g. `<abbr title="HyperText Markup Language">HTML</abbr>` renders as `HTML (HyperText Markup Language)`. Override with user CSS to change the format. |
| `small` | Fine print / secondary text (default: `color: #888888`). No font-size reduction is possible in terminals. |
| `q` | Inline quotation; the UA stylesheet injects `open-quote` before and `close-quote` after the content. The characters used depend on the inherited `quotes` property (default `"…"` / `'…'` for nested). |
| `code` | Inline styled span |
| `pre` | Raw text block; `white-space: pre` by default; child elements are styled |
| `br` | Line break (inline or block) |
| `hr` | Horizontal rule. The UA stylesheet provides `hr { display: block; border-top: "─"; }`, drawing a full-width line. Override `border-top` to change the character and `border-top-color` to change the color. |
| `ul` | Unordered list; `• ` prefix by default; see [list CSS section](#css-properties--lists-ul-ol) |
| `ol` | Ordered list; decimal prefix by default; see [list CSS section](#css-properties--lists-ul-ol) |
| `li` | List item; content word-wraps with a hanging indent aligned to the prefix. Style the bullet/number with `li::marker { color: …; font-weight: bold; … }`. |
| `dl` | Definition list block (default: `display: block; margin-bottom: 1`) |
| `dt` | Definition term; rendered as a bold block (default: `display: block; font-weight: bold`) |
| `dd` | Definition description; rendered as an indented block (default: `display: block; padding-left: 4`) |
| `figure` | Block container for self-contained content such as illustrations or code (default: `display: block`). Style with `margin-left`/`margin-right` to indent. |
| `figcaption` | Caption for the nearest `<figure>` ancestor (default: `display: block; font-style: italic`) |
| `img` | Inline image. The UA stylesheet provides `img::before { content: attr(alt); }`, so alt text is shown by default and nothing is emitted when `alt` is absent. Override with user CSS to change the format — for example, to produce Markdown-style output: `img::before { content: "![" attr(alt) "](" attr(src) ")"; }` |
| `address` | Contact/attribution block (default: `display: block; font-style: italic`). |
| `details` | Disclosure container (default: `display: block`). Always rendered fully expanded — no interactivity. Content is preserved and displayed. |
| `summary` | Disclosure summary; the visible heading of a `<details>` block (default: `display: block; font-weight: bold`). |
| `noscript` | Content is always rendered (no JavaScript in terminal). The HTML5 parser may deliver noscript content as raw text; it is re-parsed and rendered as HTML automatically. |
| `template` | Inert template content. The element and all descendants are skipped; styles and counters inside it do not affect the document. |
| `menu` | Semantic list of commands; treated identically to `<ul>` (default `list-style-type: disc`, `padding-left: 4`). |
| `wbr` | Optional line-break hint. Emits nothing (no terminal equivalent). |
| `table` | See table section below. Defaults to `display: table`; set `display: block` with block `tr`/`td` rules to linearize table markup as ordinary document flow. |
| `caption` | Table caption (default: `display: block; text-align: center`). Rendered above the table, centered over the full table width. |
| `colgroup` | Column group; direct child of `<table>`. A `span` attribute (default 1) applies the group's own `style=` across that many columns when no `<col>` children are present. Style via `style=` or CSS selectors. |
| `col` | Column descriptor inside `<colgroup>`. `span` attribute (default 1) repeats the column's declarations across N consecutive columns. Supports `width`, `min-width`, `max-width`, `text-align`, `color`, `background-color`, `font-weight`, `font-style`, `text-decoration` via `style=` or CSS; the legacy `width` HTML attribute is ignored (see [Cell Sizing and Table Borders](#css-properties--cell-sizing-th-td-and-table-borders-table) below, and `docs/TABLES.md`). Cell-level declarations take priority over `<col>` declarations. |
| `thead`, `tbody`, `tfoot` | Transparent wrappers inside `<table>` |
| `tr` | Table row; first `<tr>` containing `<th>` is the header |
| `th`, `td` | Table cells. `colspan`/`rowspan` HTML attributes (not CSS - these predate CSS and have no CSS equivalent) merge a cell across multiple columns/rows: `colspan` widens the cell to the combined width of the columns it spans (reclaiming their interior separator as content space, and growing those columns if the cell's own content needs more room than they'd otherwise have); `rowspan` renders as one genuinely merged box spanning multiple rows - unbroken border, content flowing continuously down through the rows it covers, `vertical-align` centering/bottom-aligning across the whole merged block rather than any single row. `rowspan="0"` spans every remaining row in the table. |
| `div` | Generic block container (default: `display: block`; no other UA styles) |
| `section`, `article`, `aside`, `header`, `footer`, `main`, `nav`, `hgroup`, `search` | HTML5 sectioning elements; all default to `display: block` with no other UA styles. Style freely with CSS. |
| `form` | Generic block container for form controls (default: `display: block`; no other UA styles). |
| `fieldset` | Groups related form controls in a bordered box (default: `display: block; border-style: solid; padding: 1; margin-bottom: 1`). |
| `legend` | Caption for the nearest `<fieldset>` (default: `display: block; font-weight: bold`), rendered as its own line at the top of the fieldset's content — a simplified terminal approximation, not browsers' border-straddling placement. |
| `label` | Inline by default (no UA rule); place its associated control inside it (e.g. `<label>Name: <input type="text"></label>`) to have them flow on one line. |
| `input` | Void element; content is synthesized from attributes, not children (default: `display: inline-block`). `type="checkbox"` → `☐`/`☑` based on the `checked` attribute; `type="radio"` → `○`/`●`; `type="submit"`/`"reset"`/`"button"` → `[ Label ]` using `value` (falling back to "Submit"/"Reset"/"Button"); `type="hidden"` → nothing; every other type (including the default, unset type) → its plain `value` (falling back to `placeholder` when `value` is absent), space-padded (in terminal columns, so double-width CJK/emoji values still fill exactly their resolved width) to its resolved width — no brackets, matching real browsers, which don't bracket text fields either. That width is the `size` attribute (default `20`, matching browsers' own default) unless overridden by an author `width`/`min-width`/`max-width`, the same `resolveWidthConstraints` resolution `<progress>`/`<meter>`'s bar uses; a value longer than the resolved width overflows uncut rather than being truncated/scrolled into view. `Element.Value()`/`SetValue()`/`Checked()`/`SetChecked()` read and write the exact attributes this renders from. A `readonly` (or `disabled`) text entry renders identically but takes no user edits — see `COMPATIBILITY.md`. |
| `button` | Renders its children normally, wrapped in brackets via the UA stylesheet's `button::before { content: "[ "; }` / `button::after { content: " ]"; }` (default: `display: inline-block`). |
| `textarea` | Multi-line bordered box (default: `display: block; border-style: solid; padding-left: 1; padding-right: 1`). Shows the `value` attribute if set (matching `Element.Value()`/`SetValue()`); otherwise falls back to its child text, with one leading newline right after the opening tag ignored, per the HTML spec's default-value rule. Its height comes from that content: the `rows`/`cols` attributes are not implemented, so an empty textarea is one row tall rather than HTML's default two — it is exempt from the [empty-box](#height) zero-height rule precisely so that row survives. |
| `select` | Void of its own text content; content is synthesized from `<option>` children, including those nested one level inside an `<optgroup>` (default: `display: inline-block`). Closed state shows the selected option's label bracketed with a disclosure indicator, e.g. `[ Banana ▾]`, styled like any other inline-block element. On a live `Document`, clicking or pressing Enter/Space while focused opens a dropdown popup listing every `<option>`, independently styleable via CSS; an `<optgroup>`'s `label` renders as its own non-navigable header row, and `<optgroup disabled>` cascades to every option inside it. `Element.Value()`/`SetValue()` read and write the selected option's `value` attribute, mirroring `HTMLSelectElement.value`. **See `docs/SELECT.md`** for the full styling reference (popup border/padding/margin/width, per-option colors, `option:hover`, the `optgroup` label-row selector, fallback behavior) and interactive keybindings. |
| `progress` | Void element; content is synthesized as a fixed-width bar from `value`/`max` (default: `display: inline-block; width: 20; height: 1`). A missing `value` is the sole indeterminate trigger, matched by `:indeterminate`. **See `docs/PROGRESS_METER.md`** for the full styling reference (`::progress-bar`/`::progress-value`, `progress-style` presets, indeterminate rendering). |
| `meter` | Void element; content is synthesized as a fixed-width, region-colored bar from `value`/`min`/`max`/`low`/`high`/`optimum` (default: `display: inline-block; width: 20; height: 1`). **See `docs/PROGRESS_METER.md`** for the full styling reference (`::meter-bar`/`::meter-optimum-value`/`::meter-suboptimum-value`/`::meter-even-less-good-value`, the region algorithm, `meter-style` presets). |

---

## Global Attributes

The UA stylesheet provides `[hidden] { display: none; }` and
`[aria-hidden=true] { display: none; }`, so the boolean HTML `hidden`
attribute and an explicit `aria-hidden="true"` both hide an element and all
its children — same effect as setting `display: none` directly. Either can be
overridden by a more specific rule (e.g. `[hidden] { display: block !important; }`).

---

## CSS Properties — Inline / Block Elements

These apply to any matched element and control text rendering.
`normal` / `none` values explicitly cancel an inherited value.

#### `display`
`block` | `inline` | `inline-block` | `flex` | `inline-flex` | `table` | `contents` | `none`. Controls layout. `block` emits a newline after content and respects `margin-top`/`margin-bottom`. `inline` renders with no newline. `inline-block` is like `inline` but respects `width`. `flex`/`inline-flex` lay out direct element children as a flex container — see [Flexbox](#flexbox) below. `table` uses htmlterm's table renderer when set on a `<table>` element. `contents` makes the element itself generate no box at all — see below. `none` hides the element and all its children. Not inherited. Defaults: `table` defaults to `table`; `p`, `h1`–`h6`, `blockquote`, `pre`, `div`, and common HTML5 sectioning elements (`section`, `article`, `aside`, `header`, `footer`, `main`, `nav`, `hgroup`, `search`) default to `block`; all others default to `inline`.

**`display: contents`** is for semantic wrappers that should be invisible to
layout: the element generates no box of its own — no margin, padding,
border, background, and no forced line break either before or after it —
and its children are spliced directly into the surrounding flow exactly as
if they were direct children of the `contents` element's own parent. A
child that is itself `display: block` (or a table/list) still gets its own
line, since that comes from the child's own display value, not from the
wrapper. Only genuinely inherited properties set on the `contents` element
(see [Inheritance](#inheritance) — `color`, `font-weight`, `font-style`,
`text-decoration`, etc.) still reach its children; non-inherited properties
such as `background-color`, `border`, `margin`, `padding`, and `width` have
no effect, since there is no box left to apply them to.

```css
/* dl-like structure, one shared background: */
.row { display: contents; }
```
```html
<div class="row" style="color: red"><span>label</span><span>value</span></div>
<!-- renders as "labelvalue" in red — the wrapping <div> contributes no box -->
```

To treat table markup as a simple linear document flow, opt out of the table renderer:

```css
table, thead, tbody, tfoot, tr, td, th {
  display: block;
}

td, th {
  width: auto;
  white-space: normal;
}

td + td,
tr + tr {
  margin-top: 1;
}
```

#### `position`
`static` (default) | `relative` | `absolute` | `fixed`. `sticky` is
recognized but not implemented — it parses harmlessly as a no-op (same
`static` behavior). Not inherited.

- **`relative`**: the element stays in normal flow (it still reserves its
  original layout space, and siblings are unaffected), but paints visually
  shifted by `top`/`right`/`bottom`/`left` (see below) — and its recorded
  hit-test `Rect` shifts to match, so `document.Element.Rect()`/
  `DispatchClick` see the shifted position, matching a real
  `getBoundingClientRect()`. A shift that would move content outside the
  document's current row/column bounds clamps to the boundary rather than
  growing the canvas.
- **`absolute`**: removed from normal flow entirely (reserves no space —
  siblings lay out as if it weren't there) and positioned against its
  *containing block*: the nearest ancestor with `position` other than
  `static`, or the whole document if there is none. Per spec, an
  `absolute`/`fixed` element is implicitly treated as `display: block`
  regardless of its own `display` value.
- **`fixed`**: like `absolute`, but its containing block is always the
  whole document, regardless of any positioned ancestor.

For both `absolute` and `fixed`, an element with no explicit `width`
stretches to fill its containing block's width rather than real CSS's
shrink-to-fit-to-content sizing; set an explicit `width` for a
tightly-sized overlay. Spec's "static position"
algorithm (where an unpositioned axis would default to roughly where the
element would have flowed) is not implemented either — an axis with neither
side set defaults to the containing block's own top-left corner instead.
`absolute`/`fixed` on a `<tr>`/`<td>`/`<th>` or `<li>` itself (as opposed to
content nested inside one) is not supported.

When multiple `absolute`/`fixed`/`relative` elements' painted content
overlaps, `z-index` (see below) decides paint order, ties broken by
document order — full painter's-algorithm compositing (each later element
fully overwrites the cells it covers), not per-cell blending.

```css
.overlay { position: absolute; top: 2; left: 4; width: 20; z-index: 1; }
.badge { position: relative; top: -1; left: 3; }
```

#### `top`, `right`, `bottom`, `left`
Integer, `ch`, `%`, or `auto` (the default — no offset on that side). Only
meaningful on a `relative`/`absolute`/`fixed` element. If both `top` and
`bottom` are set (non-`auto`), `top` wins; if both `left` and `right` are
set, `left` wins. For `relative`, percentages resolve against the
document's current width (`left`/`right`) or line count (`top`/`bottom`) —
an approximation of real CSS's containing-block-relative percentages,
consistent with how percentages are approximated elsewhere in this
renderer. For `absolute`/`fixed`, percentages resolve against the real
containing block's own width/height. Not inherited.

#### `z-index`
Integer (default `0`). Only meaningful on a `relative`/`absolute`/`fixed`
element whose painted content overlaps another positioned element's — the
higher value paints on top; equal values (including the default, the common
case of two positioned elements that don't set it) fall back to document
order. Not inherited.

#### `color`
Any CSS color value (see [Color Values](#color-values)). Foreground color. Inherited.

#### `background-color`
Any CSS color value (see [Color Values](#color-values)). Background color. Not inherited.

#### `background`
Shorthand recognized only when it contains a CSS color value. The color is
mapped to `background-color`; image, repeat, attachment, position, size, origin,
and clip components are ignored. For example, `background: url(bg.png) #003366
no-repeat` behaves like `background-color: #003366`. Not inherited.

#### `opacity`
`0.0`–`1.0`. `1` is fully opaque (the default). For `0 < opacity < 1`, scales
the foreground and background color channels toward black (terminals can't
composite against an unknown background, so darkening is the closest
approximation). `0` renders the element's text as blank spaces of the same
width — invisible on any terminal theme, though (like real CSS `opacity:0`)
the element still occupies its layout box. Inherited.

#### `font-weight`
`bold` | `normal`. `normal` cancels inherited bold. Inherited.

#### `font-style`
`italic` | `normal`. `normal` cancels inherited italic. Inherited.

#### `text-decoration`
`underline` | `line-through` | `none` | `normal`. `none`/`normal` cancels both underline and strikethrough. Inherited.

#### `text-align`
`left` | `center` | `right`. Effective on cells when column has a width. Inherited.

#### `margin`
One to four values using CSS shorthand order. Expands to `margin-top`,
`margin-right`, `margin-bottom`, and `margin-left`.

| Values | Expansion |
|--------|-----------|
| `A` | all sides = `A` |
| `A B` | top/bottom = `A`, right/left = `B` |
| `A B C` | top = `A`, right/left = `B`, bottom = `C` |
| `A B C D` | top = `A`, right = `B`, bottom = `C`, left = `D` |

Values use the same formats as the corresponding longhand properties. For
example, `margin: 1 auto` sets top/bottom margins to `1` and left/right margins
to `auto`. Not inherited.

#### `margin-top`
Integer line count (e.g. `1`). Extra blank lines above a block element. Adjacent margins collapse: the larger wins. Not inherited.

#### `margin-bottom`
Integer line count (e.g. `1`). Extra blank lines below a block element. Collapses with the next element's `margin-top`. Not inherited.

#### `margin-left`
Integer (e.g. `4`) or `auto`. Spaces prepended to every line of a block element, outside any `border-left`. Not inherited.

#### `margin-right`
Integer (e.g. `4`) or `auto`. Spaces appended to every line of a block element, outside any `border-right`. Not inherited.

#### `margin-block-start`, `margin-block-end`, `margin-inline-start`, `margin-inline-end`
Logical aliases for the physical margin properties. htmlterm does not model
writing modes or RTL layout, so these always map as follows:

| Property | Alias for |
|----------|-----------|
| `margin-block-start` | `margin-top` |
| `margin-block-end` | `margin-bottom` |
| `margin-inline-start` | `margin-left` |
| `margin-inline-end` | `margin-right` |

Values use the same formats as the corresponding physical longhand properties.
Not inherited.

**`auto` margins:** When an element has an explicit `width` set, `margin-left: auto` and/or `margin-right: auto` distribute the remaining space. Both `auto` centers the element; only `margin-left: auto` right-aligns it; only `margin-right: auto` left-aligns it (fills trailing space). Without an explicit `width` the element already fills the available width and auto margins have no visible effect.

#### `width`
`40` or `50%`. Fixed or percentage width for block and `inline-block` elements. For block elements, `width: 100%` fills the renderer width; margins and border characters are subtracted so the total visual line equals the specified width. Not inherited.

#### `min-width`
`40` or `50%`. Minimum width for block and `inline-block` elements, using
the same box semantics as `width`. On block elements it constrains wrapping,
alignment, borders, and auto-margin placement. Not inherited.

#### `max-width`
`40` or `50%`. Maximum width for block and `inline-block` elements, using
the same box semantics as `width`. On block elements it constrains wrapping,
alignment, borders, and auto-margin placement. Not inherited.

#### `padding`
One to four values using CSS shorthand order. Expands to `padding-top`,
`padding-right`, `padding-bottom`, and `padding-left`.

| Values | Expansion |
|--------|-----------|
| `A` | all sides = `A` |
| `A B` | top/bottom = `A`, right/left = `B` |
| `A B C` | top = `A`, right/left = `B`, bottom = `C` |
| `A B C D` | top = `A`, right = `B`, bottom = `C`, left = `D` |

Values use the same formats as the corresponding longhand properties. Not
inherited.

#### `padding-left`
`4` or `4ch`. Left padding in rune columns; applies to block elements. Not inherited.

#### `padding-right`
`4` or `4ch`. Right padding in rune columns; applies to block elements. Not inherited.

#### `padding-top`
Integer (e.g. `2`). Blank lines inserted above content, inside `border-top`. Each blank row is as wide as the content area so left/right borders and padding align correctly. Not inherited.

#### `padding-bottom`
Integer (e.g. `2`). Blank lines inserted below content, inside `border-bottom`. Same width semantics as `padding-top`. Not inherited.

#### `padding-block-start`, `padding-block-end`, `padding-inline-start`, `padding-inline-end`
Logical aliases for the physical padding properties. htmlterm does not model
writing modes or RTL layout, so these always map as follows:

| Property | Alias for |
|----------|-----------|
| `padding-block-start` | `padding-top` |
| `padding-block-end` | `padding-bottom` |
| `padding-inline-start` | `padding-left` |
| `padding-inline-end` | `padding-right` |

Values use the same formats as the corresponding physical longhand properties.
Not inherited.

#### `height`
Integer line count (e.g. `5`). Content-box height in lines. If the rendered content has fewer lines it is padded with blank lines; if it has more and `overflow: hidden`/`clip` is set it is truncated. Without an overflow setting, extra content is visible. Takes priority over `min-height` and `max-height` when set. Not inherited.

An element with **no content at all** has a content height of zero, so it
reserves no line: an empty bordered box draws its top and bottom rule adjacent
with nothing between them, and an element with only a `border-top` (which is
what `<hr>` is) is a single rule. Its own `padding-top`/`padding-bottom` rows
still apply, as they do around any zero-height content box, and a declared
`height`/`min-height` still reserves exactly what it asks for. Whitespace-only
content counts as no content, matching CSS (a run of whitespace generates no
line box). An empty element with nothing drawn around it — no rules, no vertical
padding — is still one blank line rather than nothing, since that line is what
separates it from its siblings in block flow. A `<textarea>` is never empty in
this sense: its box is a field to type into, so it keeps a row even with no
value.

#### `min-height`
Integer line count (e.g. `3`). Minimum content-box height in lines. The element is always padded to at least this many lines regardless of `overflow`. Has no effect when `height` is also set. Not inherited.

#### `max-height`
Integer line count (e.g. `10`). Maximum content-box height in lines. Content beyond this limit is truncated only when `overflow: hidden` or `overflow: clip` is also set; without overflow the content is still visible. Has no effect when `height` is also set. Not inherited.

#### `white-space`
`normal` | `nowrap` | `pre` | `pre-wrap` | `pre-line`. How text-node whitespace is handled. Inherited. Default `normal` for block/inline elements, including `td` and `th`. Block elements with `normal` word-wrap long lines at the available content width, breaking at word boundaries. `nowrap` disables word wrapping; set it on a cell or ancestor to get single-line truncation (see `text-overflow`) instead of multi-line wrapping. `pre` preserves all whitespace and disables wrapping. `pre-wrap` and `pre-line` preserve newlines but still allow wrapping. Content that is already multi-line (lists, `<br>` tags, nested block elements) is not re-wrapped.

#### `overflow`, `overflow-x`, `overflow-y`
`visible` | `hidden` | `clip` | `scroll` | `auto`. Controls whether content that exceeds an explicit `width`/`height` is clipped. `overflow` is shorthand for the two per-axis longhands: one value sets both `overflow-x`/`overflow-y`; two values set `overflow-x` then `overflow-y` respectively. A longhand set directly overrides just its own axis, per the normal cascade (so `overflow: auto; overflow-y: scroll` leaves `overflow-x` at `auto`). `overflow-x` gates horizontal (width) clipping and scrolling; `overflow-y` gates vertical (height) clipping and scrolling. Default `visible`: content overflows the box. Not inherited.

- **`hidden` / `clip`** — `overflow-x` truncates each line to the content width (**requires an explicit `width`**; without one the element already fills the available width — except on a `display: flex`/`inline-flex` container, where no width is required, since flex items floored at their [automatic minimum size](#automatic-minimum-size-min-widthmin-height-auto) can overflow a container that has none); `overflow-y` truncates excess lines when an explicit `height` — or, failing that, a `max-height` — is also set; this works on a `display: flex`/`inline-flex` container too, truncating it from inside its own border box. `text-overflow` controls the truncation marker.
- **`auto`** — `overflow-y` (with an explicit `height`; `min-height`/`max-height` alone don't count) makes the element a real scrollable viewport: a live per-element scroll offset (`Element.ScrollTop`/`SetScrollTop`) selects which window of lines is visible, adjustable via mouse wheel (`Document.DispatchWheel`), `PageUp`/`PageDown`/`ArrowUp`/`ArrowDown` on a focused descendant (`Document.DispatchKey`), or focus landing on an off-screen descendant (`Element.Focus` auto-scrolls it into view). `overflow-x` (**requires an explicit `width`**) is the same idea transposed: a live horizontal scroll offset (`Element.ScrollLeft`/`SetScrollLeft`) selects which window of columns is visible per line, adjustable via mouse wheel (`Document.DispatchWheel`'s `deltaX`) or `ArrowLeft`/`ArrowRight` on a focused descendant (`Document.DispatchKey`) — there is no horizontal scroll-into-view on focus yet. Either axis draws no visible scrollbar/indicator under `auto`.
- **`scroll`** — same scrolling behavior as `auto` per axis, **plus** an always-reserved gutter — a column (default 1 wide) for `overflow-y`, a row (default 1 tall) for `overflow-x` — with a track and thumb tracking the scroll position, drawn regardless of whether the content actually overflows, matching real CSS's own unconditional-scrollbar semantics for `scroll` vs. only-if-needed for `auto`. Silently omitted (content unaffected) if the box is too narrow/short to spare one, or — for `overflow-x`'s gutter row specifically — if the element also has an explicit `height` of its own (both axes' visible gutters can't coexist yet; the scroll offsets themselves still work in that combination — see `docs/SCROLLING.md`). **See `docs/SCROLLBARS.md`** for the full styling reference (`::scrollbar`/`::scrollbar-track`/`::scrollbar-thumb`/`::scrollbar-cap-*` and their `-x` horizontal counterparts, `scrollbar-style` presets, cap-button click behavior).

See `docs/SCROLLING.md` for the scrolling design itself (including why `auto` never gets an indicator).

#### `text-overflow`
`clip` | `ellipsis` | `"‹str›"`. The truncation marker appended to lines clipped by `overflow: hidden`/`clip`. Only effective when `overflow: hidden` or `overflow: clip` and `white-space: nowrap` and an explicit `width` are all set. Default `clip` (no marker) — including for table cells, where `overflow: hidden` is implicit but `text-overflow` still defaults to `clip`. `ellipsis` appends `…`. A quoted string (e.g. `text-overflow: "+"`) uses that string as the marker. Not inherited.

#### `font-variant`
`small-caps` | `normal`. `small-caps` uppercases all text content (terminal rendering cannot distinguish small-cap glyphs from full capitals). `normal` cancels an inherited value. Inherited. When both `font-variant: small-caps` and `text-transform` are set, `text-transform` wins.

#### `text-transform`
`none` | `uppercase` | `lowercase` | `capitalize` | `superscript` | `subscript`. Case/script transformation applied to text content. Inherited. `capitalize` uppercases the first letter of each whitespace-separated word. `superscript` and `subscript` replace each character with its Unicode superscript or subscript equivalent where one exists; characters with no Unicode equivalent are passed through unchanged.

#### `overflow-wrap`
`normal` | `break-word`. Controls whether long words that overflow the container width may be broken mid-word. `normal` (default): words are never broken — a word longer than the column simply overflows the line. `break-word`: a word that cannot fit on any line is hard-broken at the column boundary. Inherited. See also `word-break`.

#### `word-break`
`normal` | `break-all`. Sets the character-level line-break strategy. `normal` (default): word-boundary breaking only (same as `overflow-wrap: normal`). `break-all`: break at any character boundary, ignoring word boundaries — suitable for CJK text or URLs with no natural break points. When both `overflow-wrap` and `word-break` are set, `overflow-wrap` takes priority. Inherited.

#### `text-indent`
`<integer>` or `<N%>`. Indents the first line of a block element's content by the specified number of columns (or percentage of available width). Only applied when the element's own first content is inline text; when the first child is a block-level element, that child applies its own inherited value. Inherited.

#### `tab-size`
`<integer>`. Tab-stop interval for expanding `\t` characters inside `white-space: pre` or `pre-wrap` content. Tab characters advance to the next multiple of `tab-size` columns. Default: `8`. Has no effect when `white-space` is `normal`, `nowrap`, or `pre-line` (tabs are collapsed to a single space like any other whitespace). Inherited.

#### `visibility`
`visible` | `hidden` | `collapse`. `hidden` hides the element's content while preserving its layout space — blank characters of the same dimensions are emitted instead, measured in terminal columns, so a double-width character (CJK, emoji) is replaced by the two spaces it occupied rather than one. Unlike `display: none`, a hidden element still occupies lines in the output. `hidden` is inherited, so all descendants are also hidden unless they override with `visibility: visible`. For table cells, `visibility: hidden` renders the cell as blank (preserving the column width from other rows). Meaningful distinction from `display: none` in table and fixed-layout contexts.

`collapse` behaves as `hidden` everywhere except **on a flex item**, where it is
CSS's *collapsed flex item* (Flexbox §4.4) and behaves as `display: none`
instead: the item is dropped from layout entirely, and the main-axis space it
would have taken goes to its siblings rather than sitting blank. (A browser
keeps the collapsed item's cross-size strut alive so the flex line doesn't get
shorter; this engine doesn't — see [COMPATIBILITY.md](COMPATIBILITY.md).)
`collapse` on a table row or column is not implemented as a row/column collapse
either; it is just `hidden` there.

#### `content`

Text injected by `::before` or `::after` pseudo-element rules. The value is one or more space-separated **tokens** that are concatenated left-to-right. `none` and `normal` by themselves suppress injection entirely. Not meaningful on regular elements. Not inherited.

| Token | Example | Description |
|-------|---------|-------------|
| Quoted string | `"→ "` | Literal text; CSS escape sequences (`\A` = newline, `\22` = `"`, etc.) are decoded |
| `attr(name)` | `attr(href)` | Value of the named HTML attribute on the element; empty string if absent |
| `counter(name)` | `counter(sec)` | Current value of a CSS counter (see **Counters** below) |
| `counter(name, style)` | `counter(ch, upper-roman)` | Counter value formatted with the given `list-style-type` style |
| `counters(name, sep)` | `counters(item, ".")` | All nested counter values joined by sep (e.g. `1.2.3`) |
| `counters(name, sep, style)` | `counters(item, ".", lower-alpha)` | Nested counter values with a style applied to each |
| `open-quote` | — | Opening quote from the `quotes` property at the current nesting depth; increments depth |
| `close-quote` | — | Closing quote from the `quotes` property; decrements depth |
| `no-open-quote` | — | Increments quote depth without emitting a character |
| `no-close-quote` | — | Decrements quote depth without emitting a character |
| `none` / `normal` | — | Suppress content injection entirely (only valid as the sole token) |

**Concatenation example:**
```css
a::before { content: "["; }
a::after  { content: "](" attr(href) ")"; }
/* renders <a href="/page">link</a> as: [link](/page) */
```

---

### Counters

CSS counters let you auto-number elements. Two companion properties control them:

#### `counter-reset`
`<name> [<integer>] …`. Creates a new counter scope named `<name>`, initialized to `<integer>` (default `0`). Multiple name/value pairs may appear in one declaration. Not inherited.

```css
ol { counter-reset: item; }          /* reset to 0 */
ol { counter-reset: item 9; }        /* reset to 9; first increment → 10 */
```

#### `counter-increment`
`<name> [<integer>] …`. Increments the innermost counter named `<name>` by `<integer>` (default `1`). Multiple name/step pairs may appear. Not inherited.

```css
li { counter-increment: item; }      /* +1 each <li> */
li { counter-increment: item 2; }    /* +2 each <li> */
```

**Complete example — auto-numbered sections:**
```css
body  { counter-reset: section; }
h2    { counter-increment: section; }
h2::before { content: counter(section) ". "; }
```

**Nested numbering with `counters()`:**
```css
ol          { counter-reset: item; list-style-type: none; }
li          { counter-increment: item; }
li::before  { content: counters(item, ".") " "; }
/* produces: 1 · 1.1 · 1.2 · 2 · 2.1 … */
```

Counter styles available in `counter()` / `counters()` match `list-style-type`: `decimal` (default), `lower-alpha`, `upper-alpha`, `lower-roman`, `upper-roman`, `none`.

---

### Quotes

#### `quotes`
`"<open>" "<close>" …`. Pairs of strings used by `open-quote` and `close-quote`. The first pair is used at nesting depth 0, the second at depth 1, and so on; the last pair repeats for any deeper nesting. Inherited. Default: `"“" "”" "‘" "’"` (`"` `"` `'` `'`).

```css
/* English smart quotes (default) */
q { }   /* uses UA-stylesheet q::before/q::after rules */

/* Custom quotes */
blockquote { quotes: "«" "»" "‹" "›"; }
blockquote::before { content: open-quote; }
blockquote::after  { content: close-quote; }
```

The UA stylesheet defines `q::before { content: open-quote; }` and `q::after { content: close-quote; }`, so `<q>` elements are quoted automatically using the inherited `quotes` value.

#### `border`
Shorthand for `border-style` plus `border-color` on the whole box (all four
edges uniformly — there is no per-edge form of this shorthand; use the
individual `border-top`/`border-right`/`border-bottom`/`border-left` and
`border-*-color` longhands for that). Values are matched **positionally**,
not by type: real CSS's `border` shorthand allows `<width>`/`<style>`/
`<color>` in any order, and a real width keyword like `thick` can't be
distinguished from a style keyword by content alone once it's in an
unexpected slot — positional matching sidesteps that, resolving
`border: thick solid red` correctly (slot 0 of a 3-token value is always
the ignored width) regardless of what `thick` means to either vocabulary.

| Values | Meaning |
|--------|---------|
| `<style>` | `border-style: <style>` |
| `<style> <color>` | `border-style: <style>; border-color: <color>` |
| `<width> <style> <color>` | `<width>` ignored; `border-style: <style>; border-color: <color>` |

```css
div { border: solid red; }        /* border-style: solid; border-color: red; */
div { border: 1px solid red; }    /* same; "1px" is ignored */
```

**Not supported: the two-value `<width> <style>` form** (e.g. `border: 2px
solid;`, no color) — with no positional color slot to detect its absence,
this is indistinguishable from the two-value `<style> <color>` form and is
silently dropped like any other unrecognized value. Set `border-style`
directly instead. Not inherited.

#### `border-style`
`solid` | `rounded` | `heavy` | `double` | `markdown` | `hidden` | `none`. Applies a named border preset as a shorthand for all individual border properties. Individual `border-*` properties set on the same element override the preset for that edge (e.g. `border-top: ═` overrides the fill but keeps preset corners). `hidden`/`none` clears all borders. Not inherited.

**Note:** these preset names are htmlterm's own vocabulary, not real CSS's `border-style` keyword set (`solid`/`dashed`/`dotted`/`double`/`groove`/`ridge`/`inset`/`outset`/`none`/`hidden`) — only `solid`/`double`/`none`/`hidden` overlap in name; `rounded`/`heavy`/`markdown` are terminal-specific box-drawing presets with no real-CSS equivalent. `heavy` (drawn with Unicode "Box Drawings Heavy" characters, e.g. `┏━┓`) is not named `thick`, avoiding a collision with real CSS's `border-width: thick` keyword — see [`border`](#border) above for why that distinction matters.

#### `border-width`, `border-top-width`, `border-right-width`, `border-bottom-width`, `border-left-width`
Accepted (parsed without error) and always a no-op. Terminal box-drawing
characters have no notion of a line-thickness distinct from the character
itself — draw a heavier-looking border with `border-style: heavy` or a
custom `border-top`/`border-left`/etc. character instead. These properties
exist purely so real-world CSS (e.g. copy-pasted `border: 1px solid red`,
split into its longhands) doesn't need to be edited before use. Not
inherited.

#### `border-left`, `border-right`, `border-top`, `border-bottom`
Each accepts **two different forms**, dispatched on whether the value is quoted:

| Form | Example | Meaning |
|------|---------|---------|
| Quoted string | `border-left: "▌"` | This engine's literal-glyph form (predates the shorthand below, and remains the primary way to use box-drawing characters that have no CSS style-keyword equivalent). The exact character(s) prepended/appended (for `-left`/`-right`) or repeated as the horizontal-rule fill (for `-top`/`-bottom`). `none` (unquoted) or unset = no border. A glyph is charged its real terminal width, so a double-width one (CJK, emoji) costs the box two columns per side rather than one. |
| Bareword, standard CSS shorthand grammar | `border-left: solid red` | `<style>`, `<style> <color>`, or `<width> <style> <color>` (`<width>` ignored) — the same positional grammar as the [`border`](#border) shorthand, just resolved to *this one edge's* glyph from the named preset (e.g. `top.fill` for `border-top`, `left`/`right` for `border-left`/`border-right`) instead of the whole box. `<style>` is one of the [`border-style`](#border-style) preset names. An explicit `border-top: none` clears just that edge, even when `border-style` is also set on the same element (this used to be silently overridden by the preset — no longer). |

```css
div { border-top: "═"; }              /* literal glyph, unchanged from before */
div { border-top: double; }           /* double preset's top glyph, no color change */
div { border-top: double red; }       /* double preset's top glyph, red */
div { border-top: 1px double red; }   /* same; "1px" is ignored */
div { border-style: solid; border-top: none; }  /* solid box with the top edge removed */
```

As with [`border`](#border), the two-value `<width> <style>` form (no color) has
no positional color slot and is silently dropped. Not inherited.

#### `border-left-color`
Any CSS color value (see [Color Values](#color-values)). ANSI color applied to the left border character. Not inherited.

#### `border-right-color`
Any CSS color value (see [Color Values](#color-values)). ANSI color applied to the right border character. Not inherited.

#### `border-top-color`
Any CSS color value (see [Color Values](#color-values)). ANSI color applied to the top border rule. Not inherited.

#### `border-bottom-color`
Any CSS color value (see [Color Values](#color-values)). ANSI color applied to the bottom border rule. Not inherited.

#### `border-color`
One to four color values using CSS shorthand order (like `margin`/`padding`).
Expands to `border-top-color`, `border-right-color`, `border-bottom-color`,
and `border-left-color`.

| Values | Expansion |
|--------|-----------|
| `A` | all sides = `A` |
| `A B` | top/bottom = `A`, right/left = `B` |
| `A B C` | top = `A`, right/left = `B`, bottom = `C` |
| `A B C D` | top = `A`, right = `B`, bottom = `C`, left = `D` |

The single-value form (`border-color: #555555`) also doubles as the `<table>`
border-color property (see `docs/TABLES.md`), which colors the whole table
frame uniformly rather than per edge. Not inherited.

#### `border-top-left-corner`
`"<string>"` | `'<string>'`. Quoted character placed at the left end of the top border rule. Falls back to the `border-top` fill character when unset. Not inherited.

#### `border-top-right-corner`
`"<string>"` | `'<string>'`. Quoted character placed at the right end of the top border rule. Falls back to the `border-top` fill character when unset. Not inherited.

#### `border-bottom-left-corner`
`"<string>"` | `'<string>'`. Quoted character placed at the left end of the bottom border rule. Falls back to the `border-bottom` fill character when unset. Not inherited.

#### `border-bottom-right-corner`
`"<string>"` | `'<string>'`. Quoted character placed at the right end of the bottom border rule. Falls back to the `border-bottom` fill character when unset. Not inherited.

---

## CSS Properties — Lists (`ul`, `ol`)

These properties apply to `<ul>` and `<ol>` elements and control list rendering.
Item content word-wraps at the available content width.

| Property | Values | Notes |
|----------|--------|-------|
| `list-style` | Any supported `list-style-type` and/or `list-style-position`, in either order | Shorthand for the supported list longhands. `list-style-image` values such as `url(...)` are ignored. |
| `list-style-type` | See table below | Prefix string for each `<li>`. A quoted string literal (e.g. `"→ "`) is used verbatim as the bullet. `symbols("<string>" ...)` cycles a list of custom bullets. Not inherited. Default: `disc` for `<ul>`, `decimal` for `<ol>`. |
| `list-style-position` | `outside` (default), `inside` | `outside`: prefix hangs to the left; continuation lines align under the first text character. `inside`: prefix flows inline with text; continuation lines align with `padding-left`. Inherited. |
| `padding-left` | `4` | Indents the entire list from the left; combined with `margin-left` for total indentation. Default: `4`. |
| `margin-left` | `4` | Left margin for the list block; added to `padding-left`. Default: `0`. |

### `list-style-type` values

| Value | Prefix | For |
|-------|--------|-----|
| `disc` | `• ` | `<ul>` (default) |
| `circle` | `○ ` | `<ul>` |
| `square` | `■ ` | `<ul>` |
| `none` | *(empty)* | `<ul>`, `<ol>` |
| `decimal` | `1.` `2.` … | `<ol>` (default) |
| `lower-alpha` / `lower-latin` | `a.` `b.` … | `<ol>` |
| `upper-alpha` / `upper-latin` | `A.` `B.` … | `<ol>` |
| `lower-roman` | `i.` `ii.` … | `<ol>` |
| `upper-roman` | `I.` `II.` … | `<ol>` |
| `"<string>"` / `'<string>'` | custom | `<ul>`, `<ol>` |
| `symbols("<string>" "<string>" …)` | custom, cycled | `<ul>`, `<ol>` |

A quoted string literal sets a custom bullet used verbatim for every item, e.g. `list-style-type: "→ "`. The string is used as-is with no additional spacing — include a trailing space in the string if desired. Works on both `<ul>` and `<ol>`.

`symbols("<string>" "<string>" …)` takes a whitespace-separated list of two or
more quoted strings and cycles through them one per `<li>`, wrapping back to
the first string after the last (e.g. `symbols("🟥" "🟨" "🟦")` on a four-item
list produces 🟥, 🟨, 🟦, 🟥). Each string is used verbatim, same as the single
quoted-string form — include trailing spacing in the string if desired. Only
the plain string-list form is supported: the optional leading
`<symbols-type>` keyword (`cyclic`/`numeric`/`alphabetic`/`symbolic`/`fixed`)
and `<image>` arguments from the CSS spec are not recognized. Works on both
`<ul>` and `<ol>`.

The `list-style` shorthand accepts the supported type and position values in
either order, e.g. `list-style: square inside`, `list-style: inside upper-roman`,
`list-style: "→ " outside`, or `list-style: symbols("→ " "· ") inside`. Image
values such as `url(bullet.png)` are ignored, so `list-style: url(bullet.png)
square` behaves like `list-style-type: square`.

Numeric prefixes (`decimal`, `*-roman`, `*-alpha`) are right-aligned within a
fixed-width column sized to the widest prefix in the list (e.g. `" 1."` aligns
with `"10."` in a ten-item list).

Nested lists are supported: a `<ul>` or `<ol>` anywhere inside an `<li>` is
rendered recursively, indented within the parent item's content width.

**`<ol start="N">`:** The `start` attribute sets the initial counter value.
For example, `<ol start="5">` numbers items 5, 6, 7 … The prefix column width
is sized to the widest number that will appear (e.g. `start="9"` with two items
produces a two-digit-wide column for items 9 and 10).

**Not supported:** `list-style-image`; the `type` HTML attribute on `<ol>` (use
`list-style-type` CSS instead).

---

## Flexbox

`display: flex` and `display: inline-flex` lay out an element's direct
element children as flex items. A `flex` container is block-level and fills
the width available to it; an `inline-flex` container is **shrink-to-fit**,
sized to its items' own extent unless it declares a `width` of its own (or
their extent exceeds the space available, in which case that space bounds it).
That matters for `justify-content` and `flex-grow`, which have free space to
distribute only when the container's main size is definite — inside a plain
`inline-flex` there is none, so both are inert. Only single-row and single-column layouts
are currently supported, not full CSS Flexbox — see "Not supported" below
for the gaps.

Either kind of container builds the same margin, border, and padding box every
other box in this engine does. An `inline-flex` container's own border is drawn
around its items, its padding reserves columns inside that border, a declared
`width` is its total painted width with the border characters coming out of it,
and `overflow-x`/`overflow-y` clip it from inside its own border box. The one
inline-specific consequence is vertical: a container more than one line tall,
which a border alone is enough to make it, breaks the line of text it sits in,
since this engine places an atomic inline box as a unit rather than on a
baseline. See `COMPATIBILITY.md`.

Text nodes directly inside a flex container are not rendered as flex items
(wrap loose text in a `<span>` to include it); a child with `display: none`
is skipped, same as normal flow. A child with `display: contents` generates
no box of its own, so *its* children become flex items of the container
instead (recursively, for nested `display: contents` wrappers) — they take
part in the container's own `order` sequence, and anything they inherit
through the wrapper still reaches them. Every other direct element child is
blockified into a flex item regardless of its own `display` value (matching
real CSS) — a `<button>` or `<span>` child lays out exactly like a `<div>`
child would.

Blockification changes an element's **outer** display type only, so an item
that renders as something other than a plain block still does: a `<table>` item
is laid out by the table renderer, a `<ul>`/`<ol>`/`<menu>` item keeps its
markers, and `<input>`, `<select>`, `<progress>` and `<meter>` items still
synthesize their content from their attributes rather than from child nodes
(they have none) — see [HTML Elements](#html-elements) for what each renders.
The same is true of a control blockified by author CSS (`display: block` on an
`<input>`). Such a control fills its own content box when it has a resolved
width — flex layout's own resolved main size counts — so a bordered item's
field sits inside its border rather than overflowing it by that border's width.

#### `flex-direction`
`row` (default) | `row-reverse` | `column` | `column-reverse`. `row` places
items left to right (main axis = columns, cross axis = lines); `column`
stacks items top to bottom (main axis = lines, cross axis = columns).
`row-reverse`/`column-reverse` lay out the same axis in the opposite visual
direction — `order` (below) is resolved first, then the reversed direction
flips that sequence, matching real CSS. Under `flex-wrap: wrap` it flips each
line's own placement rather than the sequence lines are broken from; see
[`flex-wrap`](#flex-wrap). A reverse direction also moves
the main-*start* edge to the far end of the axis (the right edge for
`row-reverse`, the bottom for `column-reverse`), so items pack against that
edge under the default `justify-content`, and `justify-content: flex-start`/
`flex-end` swap meaning to match; `center`/`space-between`/`space-around` are
their own mirror images and are unaffected. Physical margins stay physical: an
item's `margin-right` is on its right whichever direction laid it out. Only
those four exact keywords are recognized — a misspelling falls back to `row`
rather than partly applying. Not inherited.

#### `flex-wrap`
`nowrap` (default) | `wrap` | `wrap-reverse`. `row` direction only — `column` direction has no
wrapping support (see "Not supported"). `nowrap` keeps every item on one line
(overflowing the container's width if they don't fit, same as always).
`wrap` buckets items into multiple lines using each item's resolved
`flex-basis`/`width`/natural width (before `flex-grow` is applied): items are
added to the current line until the next one wouldn't fit, then a new line
starts; an item wider than the container still gets its own line rather than
being dropped or split. Lines stack top to bottom with `row-gap` between
them; `flex-grow`/`justify-content` are then resolved independently within
each line, matching real CSS ("the line" is the scope `flex-grow` distributes
within once wrapping is in play, not the whole container). Items are collected
into lines in `order`-modified **document** order whichever way the main axis
points, so `row-reverse` changes where along each line its items land but not
*which* line they land on — again matching real CSS, which breaks lines before
it places anything. `wrap-reverse`
wraps exactly like `wrap`: its one distinguishing feature — stacking the lines
bottom to top, so the first line is the last row — is not implemented, and it
is otherwise treated as `wrap` rather than as `nowrap`, since the alternative
was overflowing the container's width on a single line. Not inherited.

```css
/* items reflow onto additional lines once the row is full */
.row { display: flex; flex-wrap: wrap; width: 100%; gap: 1; }
```

#### `flex-flow`
Shorthand for `flex-direction` and `flex-wrap`. Both components are optional
and may appear in either order (`flex-flow: wrap column` is the same as
`flex-flow: column wrap`); whichever is omitted resets to its own initial
value (`row` / `nowrap`), as a shorthand always resets every longhand it
covers. A value naming the same component twice, or containing a keyword
belonging to neither, is dropped as invalid. Not inherited.

```css
.row { display: flex; flex-flow: row wrap; }
```

#### `justify-content`
`flex-start` (default) | `flex-end` | `center` | `space-between` |
`space-around` | `space-evenly`. `space-between` puts all the free space
between items and none at the edges; `space-around` centers an equal share on
each item, so the edges get half a share; `space-evenly` makes every gap equal,
edges included. The Box Alignment synonyms `start`/`end` are accepted for
`flex-start`/`flex-end`, and `normal` behaves as the unset default.
Distributes leftover main-axis space among items. In `row`
direction this leftover is always available (the row's width is always
definite). In `column` direction it's only available once the flex container
has a [declared main size](#container-height-min-height-and-max-height) —
its `height`, or failing that its `min-height` — taller than the items' own
resolved main-axis (vertical) stack; this engine has no other notion of a
column container's main-axis size (see `flex-grow`/`flex-basis` below). With
neither, there's no leftover to distribute and `justify-content` has no
effect. "Leftover space" is whatever's left after `flex-grow`/`flex-shrink`
have already resolved each item's main-axis size — if any item can grow,
`justify-content` typically has nothing left to distribute. Under
`row-reverse`/`column-reverse`, `flex-start` and `flex-end` swap meaning,
since the main-start edge moves with the direction — see `flex-direction`.

When the items **overflow** the container the free space is negative, and every
value packs against the main-start edge — CSS's own **safe** alignment. Browsers
default to *unsafe* alignment here, so `center` overflows equally off both edges
and `flex-end` off the start edge; a terminal has nowhere to put columns pushed
off the container's start edge (they'd land in its own border/padding, or in a
sibling) and losing them is exactly what safe alignment exists to prevent.
`align-items`/`align-self` do the same for an item taller than its line. Not
inherited.

#### `align-items`
`stretch` (default) | `flex-start` | `center` | `flex-end`. Aligns items on
the cross axis. In `row` direction (cross axis = vertical), `stretch` and
`flex-start` both align content to the top of the row; `stretch` additionally
grows a shorter item's own box to the row's height, so a bordered or filled
item's box really does reach the full height rather than sitting short with
blank rows underneath. The interior of the grown box is blank — this engine
has no way to stretch text content itself. An item that declares its own
`height` is not stretched, matching CSS's "stretch applies only to an auto
cross size", and an item's own `max-height` caps how far it stretches, matching
the spec's "while still respecting the constraints imposed by
`min-height`/`max-height`". That row
height is the tallest item's, unless the container is single-line
(`flex-wrap: nowrap`, the default) and has a
[declared height or min-height](#container-height-min-height-and-max-height) —
then the line takes the container's height instead, so `align-items: center` on
a fixed-height row vertically centers its items, matching real CSS. In `column`
direction (cross axis = horizontal), `stretch` fills the container's full
width (the default for any block-level child) — but only for an item whose
cross size is `auto`, matching the spec: an item's own definite `width` opts it
out of stretching entirely, and its `min-width`/`max-width` still clamp how far
it stretches (a `min-width` larger than the container wins and overflows, as it
does everywhere else here). `flex-start`/`center`/
`flex-end` instead size the item to its own `width` or natural content width
and align it within the container. Either way the size an item resolves to is
the width its `Rect` reports, so click hit-testing agrees with what was
painted. The Box Alignment synonyms
`start`/`self-start` and `end`/`self-end` are accepted for
`flex-start`/`flex-end`, and `normal` behaves as the unset default
(`stretch`). `baseline` is not supported (falls back to `flex-start`). Set on
the container; an individual item can override it with `align-self`. Not
inherited.

#### `align-self`
`auto` (default) | `stretch` | `flex-start` | `center` | `flex-end`. Set on
a flex item to override the container's `align-items` for that one item;
`auto` (or leaving it unset) defers to the container. Same value vocabulary
(including the `start`/`self-start`/`end`/`self-end`/`normal` synonyms) and
per-direction meaning as `align-items` above. Not inherited.

#### `place-content`, `place-items`, `place-self`
The CSS Box Alignment shorthands: `place-content` sets `align-content` and
`justify-content`, `place-items` sets `align-items` and `justify-items`, and
`place-self` sets `align-self` and `justify-self`. One value sets both
components; two set the **align** component first and the **justify** one
second — the opposite order from most two-value shorthands, and worth
double-checking against the axis you meant.

`justify-items` and `justify-self` are grid properties with no effect in a
flex container, and this engine doesn't implement them at all, so
`place-items`/`place-self` are in practice just spellings of
`align-items`/`align-self`. A value carrying a `safe`/`unsafe` overflow-
alignment prefix (e.g. `place-content: safe center`) is dropped as a whole
rather than misparsed — neither keyword does anything here.

```css
/* align-content: flex-end; justify-content: center */
.row { display: flex; flex-wrap: wrap; height: 10; place-content: flex-end center; }
```

#### `align-content`
`flex-start` | `flex-end` | `center` | `space-between` | `space-around` |
`space-evenly` | `stretch` (default). Same value vocabulary and distribution
as `justify-content` above, including the `start`/`end`/`normal` synonyms.
`row` direction only, and only on a **wrapping**
container (`flex-wrap: wrap`) — on a single-line `nowrap` container it has no
effect at all, matching real CSS, and it's the container's `height` feeding
`align-items` that positions content there instead (see above). Distributes
leftover cross-axis (vertical) space across lines the same way
`justify-content` distributes leftover main-axis space across items on one
line — but that leftover space only exists when the flex container has an
**declared [`height` or `min-height`](#container-height-min-height-and-max-height)**
taller than its wrapped lines' natural stack; without one, there's no free
space to distribute and this property has nothing to do. A wrapping container is padded with trailing blank rows to reach exactly
its declared height regardless of which value is set. `stretch` is
approximated as `flex-start` (no distribution): growing each line's own items
taller than their content to fill leftover space isn't implemented. Not
inherited.

```css
/* push wrapped lines to the bottom of a taller-than-content flex container */
.row { display: flex; flex-wrap: wrap; width: 100%; height: 10; align-content: flex-end; }
```

#### Container `height`, `min-height`, and `max-height`

A flex container's own vertical sizing works the same way it does for any other
block box (see the [`height`](#height) entry), with one flexbox-specific
consequence: whatever height the container ends up with is the **main size** its
items distribute among themselves in `column` direction, and the **cross size**
`align-items`/`align-content` align within in `row` direction.

- **`height`** declares that size outright. In `column` direction it's what
  `flex-grow` grows into and `flex-shrink` shrinks against; in `row` direction
  it's the single (`nowrap`) line's cross size, or the space `align-content`
  distributes the wrapped lines within.
- **`min-height`** does the same job whenever it's the larger of the two, and is
  the property to reach for when the container should be *at least* some height
  but still free to grow with its content — the common "this column fills at
  least the screen, its children share the rest" pattern. It is a floor and not
  a target, so unlike `height` it can only ever grow the container: content
  taller than the minimum has already satisfied it and is never handed to
  `flex-shrink` as a deficit. An explicit `height` wins outright when both are
  set.
- **`max-height`** clips the container, and like `max-height` everywhere else in
  this engine it does so only under an explicit `overflow-y: hidden`/`clip` —
  without one, taller content is simply visible past it. `height` takes priority
  when both are set, matching an ordinary block box.

`overflow-y: hidden`/`clip` truncates the container to that height from inside
its own border box, so a bordered container's bottom rule still closes below the
last visible row. `overflow-y: scroll`/`auto` is **not** supported on a flex
container — it needs the live scroll-offset and gutter plumbing ordinary boxes
have (see `docs/SCROLLING.md`) — and neither are percentage heights, matching
this engine's vertical sizing generally. Percentages *inside* the container
(a `flex-basis: 50%` item in a `column` container, a percentage
`min-height`/`max-height` on an item) resolve only against an explicit `height`,
never against a `min-height`: the container's real main size is then
`max(content, min-height)`, which isn't known until the content is laid out, and
real CSS treats a percentage against such an indefinite basis as `auto`.

```css
/* at least the full screen, growing with its content; the body takes the slack */
.page { display: flex; flex-direction: column; min-height: 24; }
.body { flex: 1; }
```

#### `order`
`<integer>` (default `0`). Set on a flex item to change its position in
layout order independent of its position in the document: items are
laid out sorted by ascending `order`, with document order as the tiebreak
among equal values. Applied before `row-reverse`/`column-reverse` flips the
sequence, matching real CSS. Not inherited.

```css
/* visually swap two items without touching the markup */
.first  { order: 2; }
.second { order: 1; }
```

#### `gap`, `row-gap`, `column-gap`
`gap` is shorthand for `row-gap`/`column-gap`: one value sets both, two set
`row-gap` then `column-gap`. `column-gap` inserts blank columns between items
along a row; `row-gap` inserts blank lines between items stacked in a `column`
container, and between the lines of a **wrapping** `row` container
(`flex-wrap: wrap`) — that's the one case where both gap properties are live
at once. `column-gap` has no effect in `column` direction, and `row-gap` none
in a single-line (`nowrap`) row: there's only one row/column of items to space
apart. Absolute rune counts only (e.g. `gap: 2`); percentage gaps are not
supported. Not inherited.

#### `flex-grow`
`<number>` (default `0`). In `row` direction, distributes leftover main-axis
space proportionally by weight — this leftover is always available (the row's
width is always definite). Each item grows from its own **flex base size**, not
from the [automatic minimum size](#automatic-minimum-size-min-widthmin-height-auto) that
minimum may have raised it to; see there for why that distinction is what makes
`flex: 1` split a container equally. In `column` direction, the same distribution applies to leftover
*height*, but only once the container has a
[declared `height` or `min-height`](#container-height-min-height-and-max-height)
taller than the items' own resolved main-axis (vertical) stack — this engine
has no other notion of a column container's main-axis size, so with neither,
`flex-grow` has nothing to distribute into and items simply keep their own
resolved height. "Growing" a column item's height pads it with
blank lines at the bottom (this engine has no way to make text content
itself taller), the same approximation `align-items: stretch` already uses
for row direction's cross axis. Growth never exceeds an item's own
`max-width` (`row` direction) / `max-height` (`column` direction): an item
that hits its cap is frozen there and the space it couldn't use is re-split
across the items that can still grow, repeating until either the leftover is
placed or every item is capped (real CSS resolves flexible lengths with the
same freeze-and-repeat loop). Leftover that no item can absorb flows through
to `justify-content`/the trailing padding instead of being silently dropped.
In `column` direction that cap is an **outer** size, so a bordered or padded
item's own border rules and padding rows are added onto its `max-height` before
it applies: `max-height: 3; border-style: solid` grows to five rows, three of
them content. `max-height` is content-box here while every main size flex
resolves is an outer one, and this is the same conversion `flex-basis` and
`align-items: stretch` already make.

Factors summing to **less than one** distribute only that fraction of the free
space, matching CSS: two items at `flex-grow: 0.25` share half the leftover
between them and leave the other half for `justify-content`, where two items at
`flex-grow: 1` would consume all of it. (The same rule applies to
`flex-shrink` — see there.) The sum is recomputed as items freeze at their
`max-width`/`max-height`, and it bounds each item's share of the *initial* free
space in total, not per round, so a frozen item's unused space stays unplaced
rather than moving to the items that could still grow.

A nested `display: flex`/`inline-flex` item's own height still responds to its
parent's `flex-grow`/`flex-shrink`/`flex-basis`, the same as an ordinary
block child — and the height it's allotted becomes that nested container's own
definite main size, exactly as an explicit `height` on it would: its border
encloses the grown rows, and its own items can distribute them further with
their own `flex-grow`/`justify-content`/`align-items`. Negative values are
treated as `0`, as is any value that isn't a
CSS `<number>` — `NaN`/`Inf` are Go float literals, not CSS numbers, and are
rejected. Not inherited.

```css
/* both rows grow to fill a taller-than-content column, once it has an explicit height */
.col { display: flex; flex-direction: column; height: 10; }
.row { flex-grow: 1; }
```

#### `flex-basis`
`auto` (default) | `content` | `min-content` | `max-content` | `fit-content` |
`<integer>` | `<N%>`. An item's starting main-axis size
before `flex-grow`/`flex-shrink` distribute any leftover space: width in
`row` direction, height in `column` direction. `auto` falls back to the
item's own `width`/`height` if set, else its measured/rendered natural size.
`content` uses that natural size directly, ignoring the item's own
`width`/`height` — which is the only thing distinguishing it from `auto`, so
the two differ exactly when the item declares a main size of its own.

The three **intrinsic sizing keywords** ignore the item's own `width`/`height`
the same way `content` does, and differ only in which measurement of the content
they ask for. In `row` direction:

- **`min-content`** — the item's widest unbreakable run (its longest word, or
  its widest `white-space: nowrap` span) plus its own border/padding. The same
  measurement the [automatic minimum size](#automatic-minimum-size-min-widthmin-height-auto)
  uses.
- **`max-content`** — the width the item would take if never forced to wrap.
  Wider than the container is a normal outcome; the line then overflows or
  `flex-shrink` pulls it back.
- **`fit-content`** — what the content actually comes to within the space
  available, i.e. `min(max-content, max(min-content, available))`. This is the
  same measurement `auto`/`content` already use.

In `column` direction all three behave as `content` (the item's natural
rendered height): there is no vertical min-content/max-content distinction to
draw here, since text can't be made taller or shorter than the lines it
occupies.
That natural size is a genuine **shrink-to-fit** measurement of the item's own
content: an item that paints a rectangle — a border, `text-align`, a closed
box — is measured at the width its content needs, not at the width it would
fill, and the same applies recursively to its descendants, so a bordered child
doesn't inflate its parent's basis. The measurement is taken at the container's
content width, though, so it's `fit-content` rather than real CSS's
`max-content` — see [COMPATIBILITY.md](COMPATIBILITY.md).
Percentages resolve against the container's content width in `row`
direction; in `column` direction they resolve against the container's
explicit `height` if one is set, else (matching real CSS's "percentage basis
against an indefinite container size resolves as auto") fall back to the
item's own natural height, same as `auto`.

A **zero** `flex-basis` is accepted in any unit — `0`, `0%`, `0ch`, `0px`,
`0em`, `0rem` all mean the same definite base size of nothing, so the common
`flex: 1 1 0px` behaves exactly like `flex: 1 1 0`. This is the one place a
unit this engine otherwise ignores is recognized, and it's a narrow exception:
a zero length is dimensionless in CSS (the spec lets the unit be omitted on a
zero `<length>` for that reason), so there's nothing to convert, while the
usual reason for ignoring `px` — no defensible pixel-to-column conversion —
still applies in full to any non-zero value. `flex-basis: 4px` is still
ignored, falling back to `auto`.

A `flex-basis` taller/wider than
an item's own natural content pads it with blank lines/spaces; one shorter
than the natural content doesn't force a fit — the item's content still
renders in full and overflows past the resolved size, the same graceful
degradation `flex-shrink` below relies on. The size is the item's whole
**outer** box either way — a `flex-basis: 4` bordered item in a `column`
container is 4 rows including its two border rules, not 4 rows of content
inside them (see [COMPATIBILITY.md](COMPATIBILITY.md)).

Whatever main size flex layout finally resolves — the basis above, adjusted by
`flex-grow`/`flex-shrink` — is the size the item actually renders at, taking
precedence over the item's own `width` (`row`) / `height` (`column`)
declaration, matching CSS's own "the used main size overrides the main size
property" rule. An item declaring `width: 5` on a line that grows it to `8`
paints 8 columns wide (border included), not 5 columns plus 3 blank ones; one
that shrinks to `5` paints 5, rather than overflowing at its declared width.

However it was arrived at, the basis is then clamped by the item's own minimum
and maximum in the same axis — `min-width`/`max-width` in `row` direction,
`min-height`/`max-height` in `column` — producing the spec's **hypothetical main
size**, which is what `flex-wrap` breaks lines on and what decides whether the
line grows or shrinks. The maximum applies first and the minimum second, so a
minimum larger than the maximum wins, matching CSS. This matters because an
item's own render honors those properties regardless: if flex layout allotted a
column that ignored them, the item would paint at a different width than it was
given and every later item on the line would drift.

The hypothetical is *not* where `flex-grow`/`flex-shrink` start from, though —
they start from the unclamped flex base size and apply the hypothetical as a
floor afterward, freezing any item that violates it and redividing the rest.
See [Automatic minimum size](#automatic-minimum-size-min-widthmin-height-auto).

In `column` direction the clamp is applied to the *declared* base: because flex
sizes are outer and `min-height`/`max-height` are content-box here, each bound
is compared with the item's own border/padding rows added back on, so
`flex-basis: 6; max-height: 1` on a bordered item resolves to three rows (one of
content between two rules), not one and not six. An item sized by its own
content instead is left alone by `max-height`, matching `max-height`'s ordinary
behavior in this engine: it clips only under an explicit `overflow-y:
hidden`/`clip`, and reserving fewer rows for an item than it goes on to paint
would push the rest of the column out of step with it.

The same clamp applies to `column` direction's cross axis, where
`align-items`/`align-self` other than `stretch` size the item to its own
`width` or natural content width: `min-width` there can push an item wider
than the container, matching CSS's "minimum wins" and matching what the
item's own render does to itself anyway. Not inherited.

#### `flex-shrink`
`<number>` (default `1`). When items' resolved `flex-basis` sizes together
overflow the container's available main-axis space — row direction's width
(always definite), or column direction's explicit `height` if one is set —
the overflow is distributed across items proportionally to each item's
`flex-shrink` weight times its own `flex-basis` (the real spec's "scaled
flex shrink factor") and subtracted from that item's main-axis size —
`flex-shrink: 0` opts an item out entirely. An item never shrinks below its
`min-width` (row direction) / `min-height` (column direction), and with neither
declared the floor is CSS's own **automatic minimum size** on that axis — see
below. An item that reaches its
floor is frozen there and the overflow it couldn't absorb is re-split across
the items that can still shrink, the same freeze-and-repeat loop `flex-grow`
uses against `max-width`/`max-height`. `flex-shrink` factors summing to less
than one absorb only that fraction of the overflow, and the rest simply
overflows — the mirror of `flex-grow`'s own sub-one rule (see above).
In `column` direction,
with no explicit container `height`, there's nothing to overflow against, so
`flex-shrink` never applies. Negative values are treated as `1` (the
default), as is any value that isn't a CSS `<number>` — see `flex-grow`. Not
inherited.

#### Automatic minimum size (`min-width`/`min-height`: `auto`)
In `row` direction an item's `min-width` starts at CSS's own initial value of
`auto`, not `0`, which on a flex item's main axis resolves to the
**content-based minimum size** (Flexbox §4.5, defined in CSS Sizing 3 §5.1).
In practice: **an item is never shrunk below the width its content needs**, so
the common case of two long labels in a narrow row leaves them at their natural
widths and lets the *line* overflow, rather than crushing each item until its
text spills through its own border. It applies to the flex base size too, not
just to shrinking — a `flex-basis` smaller than the item's content is raised to
fit before line-breaking sees it, matching the spec's "hypothetical main size"
(§9.2).

That raise is a **clamp, not a head start**. `flex-grow` and `flex-shrink`
distribute from each item's *flex base size* — the size `flex-basis`/`width`
actually asked for — and only then freeze any item whose share landed below its
automatic minimum, redividing what's left among the rest (§9.7 steps 3–4). The
difference is the whole behavior of `flex: 1`, whose basis is `0`: with the
minimum applied as a clamp, two `flex: 1` items split their container equally
whatever their content, and content only tilts the split for an item whose equal
share genuinely doesn't fit its own min-content — which then takes exactly its
minimum and no more. Adding the minimum to the basis instead would hand every
item its min-content width for free and share out only the remainder, so
`flex: 1` would silently size items by their text.

```css
/* both items are exactly half the container, however long their text */
.row { display: flex; width: 100%; }
.row > * { flex: 1; }
```

The content-based minimum is the item's min-content width (its widest
unbreakable run, plus its own border/padding), clamped by a definite `width` and
by `max-width`. So `width: 6` on an item holding a 12-column word still wins,
and the content is what gives way.

There are two opt-outs, both the spec's own — this is why `min-width: 0` and
`overflow: hidden` are each a standard fix for a flex item that refuses to
shrink:

- **an explicit `min-width`** replaces the automatic one outright, including
  `min-width: 0` (which floors at one column here, since no box renders in zero
  columns);
- **a non-`visible` `overflow-x`** (`hidden`, `clip`, `scroll`, `auto`) disables
  it, per §4.5's "on a flex item whose overflow is visible in the main axis".

A definite `width` or `max-width` also clamps the automatic minimum, including a
declared `0`: a zero maximum is a real bound everywhere in flex layout, not an
absent one, so `max-width: 0` genuinely stops an item growing and hands the
space to its siblings. Like `min-width: 0`, it floors at one cell.

```css
/* the standard escape hatch: let items shrink past their content */
.row { display: flex; width: 100%; }
.row > * { min-width: 0; }
```

**In `column` direction** the same rule applies to `min-height`, and it's the
axis where it matters most: nothing here truncates a box vertically without an
explicit `overflow-y`, so an item shrunk below its content would paint its full
height anyway. **An item is never shrunk below its content's own height**; the
deficit goes to the items that can genuinely absorb it, and if none can, the
container overflows rather than its items being "shrunk" on paper only. The
same two opt-outs apply, read on the vertical axis: an explicit `min-height`
replaces the automatic one, and a non-`visible` `overflow-y` disables it — and
since a non-`visible` `overflow-y` is also exactly what makes this engine
truncate a box to its resolved height, that's the one case where shrinking an
item below its content does anything visible at all. A definite `height` or
`max-height` clamps the minimum the same way `width`/`max-width` do in `row`
direction.

The vertical minimum needs no `min-content` measurement the way the horizontal
one does: an item's content height at its resolved cross size *is* its minimum,
since text can't be reflowed into fewer lines.

```css
/* the header keeps its two rows; the body absorbs the whole deficit */
.col { display: flex; flex-direction: column; height: 6; }
.body { flex-basis: 6; }
```

Whatever the automatic minimum can't prevent — a `width` narrower than the
content, a `min-width: 0`, `flex-shrink: 0` against a small declared width — an
item that would paint wider than the main size flex resolved for it is **clipped
to that size**, honoring its `text-overflow` as the marker if one is set. Real
CSS paints such overflow over the item's neighbors instead; a character grid
can't, so see [COMPATIBILITY.md](COMPATIBILITY.md) for why clipping is the
lesser evil. A flex *line* overflowing its container is not clipped — that
behaves like any other overflowing box in this engine.

```css
/* both columns shrink together, proportionally to their basis, once the row overflows */
.row { display: flex; width: 100%; }
.col { flex-basis: 20; }
```

#### `flex`
Shorthand for `flex-grow`, `flex-shrink`, and `flex-basis`:

| Value | Expansion |
|-------|-----------|
| `none` | `flex-grow: 0; flex-shrink: 0; flex-basis: auto` |
| `auto` | `flex-grow: 1; flex-shrink: 1; flex-basis: auto` |
| `initial` | `flex-grow: 0; flex-shrink: 1; flex-basis: auto` |
| `<number>` | `flex-grow: <number>; flex-shrink: 1; flex-basis: 0` (the common `flex: 1` equal-growth pattern) |
| `<basis>` | `flex-grow: 1; flex-shrink: 1; flex-basis: <basis>` — the shorthand's omitted-value defaults are `1`/`1`, not the longhands' own initial `0`/`1`, so `flex: 30%` grows |
| `<number> <number>` | grow, shrink; `flex-basis: 0` |
| `<number> <basis>` | grow, `flex-basis: <basis>`; `flex-shrink: 1` |
| `<basis> <number>` | `flex-basis: <basis>`, grow; `flex-shrink: 1` |
| `<number> <number> <basis>` | grow, shrink, basis |
| `<basis> <number> <number>` | basis, grow, shrink |

`<basis>` above is any `flex-basis` value: `auto`, `content`, one of the
intrinsic sizing keywords, or a length/percentage.

The grammar is `[ <'flex-grow'> <'flex-shrink'>? || <'flex-basis'> ]`, and that
`||` is why the basis may lead the grow/shrink pair as well as trail it —
`flex: 30% 1` and `flex: 1 30%` are the same declaration. A bare number is both
a valid `<number>` and (in this engine) a valid `<basis>`, so the two readings
overlap; the grow-first one wins, matching the grammar's own operand order,
which keeps `flex: 1 2` a grow/shrink pair and `flex: 1 2 3` a
grow/shrink/basis triple.

Every component is validated before any longhand is assigned. A value whose
tokens don't fit the grammar is **dropped whole**, leaving `flex-grow`,
`flex-shrink`, and `flex-basis` at their own initial values (`0`, `1`, `auto`) —
so `flex: 1 junk` and `flex: junk 1` change nothing, rather than granting the
shorthand's `1`/`1` defaults off a component that would then be ignored. The
same applies to the one-token form: `flex: <junk>` can't change layout.

```css
/* three equal-width columns that fill the row */
.row { display: flex; width: 100%; gap: 1; }
.col { flex: 1; }
```

#### `margin-top`, `margin-right`, `margin-bottom`, `margin-left` (on a flex item)
Fixed (integer or percentage) margins on a flex item are respected, same
value vocabulary as elsewhere. Flex items never collapse margins with each
other or the container — unlike ordinary block flow, adjacent flex items'
margins simply **add** together (on top of `gap`), and a first/last item's
margin is never absorbed by the container's own margin. In `row` direction, `margin-left`/
`margin-right` consume main-axis space (subtracted before `flex-grow`
distributes leftover space) and `margin-top`/`margin-bottom` widen the
item's own cross-axis footprint (what `align-items`/`align-self` align
within). In `column` direction, `margin-top`/`margin-bottom` add directly to
the vertical space between items (and before the first / after the last),
and `margin-left`/`margin-right` bound the space `align-items`/`align-self`
positions the item within.

`margin-left`/`margin-right: auto`, in `row` direction only, absorbs
whatever main-axis leftover space remains once `flex-grow`/`flex-shrink`
have already been resolved (an auto side contributes `0` to the item's own
`flex-basis`) — split evenly across every auto margin present anywhere in
the line if more than one exists, an odd remainder favoring the first auto
margin encountered in document order. Present on any item, it **overrides
`justify-content` for the whole line** (matching real CSS: the two
mechanisms both claim the same leftover space, and auto margins take
precedence), though it has nothing left to absorb once `flex-grow` has
already consumed all the leftover space itself. `margin-top`/
`margin-bottom: auto` (row direction's cross axis) and `margin: auto` in
`column` direction are not supported — see below.

```css
/* the classic "push this one item to the far end" pattern */
.row { display: flex; width: 100%; }
.spacer { margin-left: auto; }
```

#### Sizing deviations

An item's `flex-basis`/`width` sizes its whole **outer** box, not its content
box, so its margins come out of that size rather than adding to it —
`flex-basis: 6; margin-right: 4` occupies 6 columns of the line here and 10 in
a browser. That's the same convention `width` has everywhere else in this
engine (see the [`width`](#width) entry: a declared width is the box's total
visual width, margins and border characters included).

A `flex-basis` that *resolves* to zero stays zero as an arithmetic input to
`flex-grow`, so grow ratios are exact — `flex: 1` against `flex: 2` in a
40-column row is 13/27, not a ratio skewed by a cell reserved up front. The
floor of one cell (no box renders in zero columns/lines) is imposed on the
rendered box, not on the base size.

An item whose resolved main size is smaller than its content still paints that
content in full and overflows past the size, rather than being cut to fit —
nothing here clips a box without an explicit `overflow`. The
[automatic minimum size](#automatic-minimum-size-min-widthmin-height-auto) keeps
that from arising through `flex-shrink` on either axis; what's left is an item
that asked to be small outright (a definite `width`/`height` under its content,
`min-width: 0`, `flex-shrink: 0` against a small basis). In `row` direction such
an item is clipped to its allotment, because a line is assembled by
concatenation and an over-wide item would displace its neighbors rather than
paint over them; in `column` direction nothing is displaced by an over-tall
item, so it simply overflows. See [COMPATIBILITY.md](COMPATIBILITY.md).

An `inline-flex` container taller than one line breaks the line of text it
sits in: it's spliced into the inline flow as one indivisible unit, and its
own line breaks go into that flow as-is rather than the box being placed
beside the surrounding text. Any item that wraps is enough to trigger this.
The same is true of a multi-line `inline-block`; see `COMPATIBILITY.md`.

#### Not supported

- **`flex-wrap`/`align-content` in `column` direction** — wrapping into
  multiple columns isn't implemented; `column` direction remains single-line
  regardless of `flex-wrap`. **`flex-wrap: wrap-reverse`** (row direction) —
  only `wrap`'s top-to-bottom line order is supported.
- **`align-content: stretch`** growing each line's items taller than their
  content — approximated as `flex-start`; see above.
- **`baseline`** alignment — falls back to `flex-start`.
- **The physical `left`/`right` alignment keywords** — unlike `start`/`end`,
  they're direction-relative rather than main-start-relative, so they're left
  unrecognized (falling back to `flex-start`) rather than guessed at.
- **Text nodes directly inside a flex container** — real CSS wraps a run of
  loose text in an anonymous flex item; here it isn't rendered at all. Wrap it
  in a `<span>`.
- **`overflow-y: scroll`/`auto` on a flex container** — the clipping values
  (`hidden`/`clip`) are supported, but a scrollable flex container would need
  the live scroll-offset/gutter plumbing ordinary boxes have; see
  [Container `height`, `min-height`, and `max-height`](#container-height-min-height-and-max-height).
- **`margin: auto` beyond `row` direction's main axis** — `margin-top`/
  `margin-bottom: auto` (row direction's cross axis, which would center/align
  a single item vertically within its line) and any `margin: auto` in
  `column` direction are all treated as `0`, not the CSS leftover-space-
  absorbing behavior; see above for the one case (`row` direction's
  `margin-left`/`margin-right`) that is supported.

---

## Size Values

Wherever one of these sizing declarations is accepted (`width`, `min-width`,
`max-width`, and percentage-capable margins), the following forms are recognized:

| Form | Example | Meaning |
|------|---------|---------|
| Bare integer | `14` | Fixed rune count |
| `ch` unit | `14ch` | Fixed rune count (same as bare integer) |
| Percentage | `50%` | Fraction of the available content width |

Pixel (`px`), `em`, `rem`, and other CSS units are ignored. The one exception
is a **zero** [`flex-basis`](#flex-basis), which is accepted in any unit, since
a zero length is dimensionless in CSS and there is nothing to convert — see
that entry.

For block elements, percentages are resolved against the available block width.
For a `width: 100%` table, the available content width is the terminal width
minus the sum of all separator characters.

---

## CSS Properties — Cell Sizing (`th`, `td`) and Table Borders (`table`)

The legacy `width` HTML attribute on `<th>`/`<td>` is ignored — in real-world
markup (especially HTML email) it's almost always a pixel value, and there's
no reliable way to convert pixels to terminal columns. Use CSS `width`
instead (`width: 14` for a fixed character count, or `width: 25%`).

| Property | Applies to | Notes |
|----------|------------|-------|
| `width`/`min-width`/`max-width` | `th`/`td` | Fixed or percentage column width; fixed/percentage columns are immune to the table's expand/shrink pass |
| `white-space` | `th`/`td` | `normal` (default) word-wraps; `nowrap` clips to one line via `text-overflow` instead |
| `text-overflow` | `th`/`td` | Truncation marker when `white-space: nowrap`; default `clip` |
| `vertical-align` | `th`/`td` | `top` (default) \| `middle` \| `bottom`, within the row's own height |
| `border-style` | `table`, `th`/`td` | Whole-box preset (`solid`/`rounded`/`heavy`/`double`/`markdown`/`standard`/`hidden`/`none`), same property used by any block element. Real-CSS default is `none` — a bare `<table>` with no CSS renders completely borderless, matching a real browser |
| `border-top`/`-right`/`-bottom`/`-left`, `border-*-color`, `border-*-corner` | `table`, `th`/`td` | Literal glyph or shorthand grammar, same as block elements |
| `caption-side` | `table` | `top` (default) \| `bottom` |
| `border-collapse: separate` \| `collapse` | `table` | Real CSS semantics — `separate` (the default) gives every `th`/`td` its own independent border box plus `border-spacing`; `collapse` merges adjacent cell/table borders into shared lines via real conflict resolution |
| `border-spacing` | `table` | Gap between cell boxes under `border-collapse: separate` (default `0`) |
| `border-top-junction`/`-right`/`-bottom`/`-left`/`-center` | `table`, `th`/`td` | Literal junction-glyph override for a T-shape/cross under `border-collapse: collapse` — a cell's own version applies at its own edges; no shorthand yet |
| `border-top-left-junction`/`-top-right`/`-bottom-left`/`-bottom-right` | `table` only | Literal junction-glyph override for a corner-shape, wherever it occurs anywhere in the grid, under `border-collapse: collapse` — no cell-level equivalent (redundant with cell-level `border-*-corner`, which already covers "my own corner") |

**See `docs/TABLES.md`** for the full table styling reference — every
property above with real rendered examples of each `border-style` preset,
the multi-line-cell wrapping model, how `margin`/`padding` behave on
`<table>` itself (padding genuinely works under `border-collapse: separate`,
matching real CSS — a fact easy to miss since most real-world tables never
rely on it), and both `border-collapse` modes' real per-cell border support.

---

## `<style>` Tags

A `<style>` element anywhere in the HTML is parsed as a CSS stylesheet and
applied for that `Render()` call only — rules do not persist to subsequent
calls. Rules in `<style>` tags override the base stylesheet at equal
specificity (same cascade position as a later `<link>` in a browser).

```html
<style>
  td.highlight { color: #ff9e64; font-weight: bold; }
</style>
<table>...</table>
```

---

## Inline `style` Attribute

Any element can carry a `style=""` attribute with a declaration list. Inline
styles win over all stylesheet rules (same cascade position as in standard CSS).

```html
<table style="width: 100%; border-style: rounded">
<td style="color: #ff9e64; max-width: 40%">
```

The same property names and value forms apply as in the stylesheet, including
`!important`. A normal (non-`!important`) inline declaration wins over any
normal stylesheet rule but loses to an `!important` stylesheet rule; an
`!important` inline declaration wins over everything, including `!important`
stylesheet rules.

`document.Element.Style()` gives a live `Document` caller a
`CSSStyleDeclaration`-like handle onto this same attribute
(`GetPropertyValue`/`SetProperty`/`RemoveProperty`/`CSSText`/`SetCSSText`)
instead of reading/writing the raw attribute string via `GetAttribute`/
`SetAttribute("style", ...)` directly. Two gaps versus the real DOM API,
both stemming from shorthand properties being expanded into longhands at
parse time with no record that a shorthand was used: `SetProperty("margin",
"1px")` is stored as `margin-top`/`-right`/`-bottom`/`-left`, so
`GetPropertyValue("margin")` afterward returns `""`, not `"1px"`; and
`CSSText`/`SetCSSText` don't preserve declaration order — `CSSText`
serializes properties sorted by name, not in the order they were set or
appeared in the original `style=""` text. See `docs/DOM_API.md` for the
full DOM-vs-htmlterm comparison table.

---

## Table — Width and Full-Width Expansion

Set `width: 100%` on a `<table>` (via a CSS rule or class) to expand it to
the renderer's terminal width. Flexible columns (no `width` or `width: Nch`)
share remaining space evenly, respecting `max-width` caps.

---

## Color Values

Color strings are parsed using CSS Color Level 4 syntax. The following formats are supported:

| Format | Example | Notes |
|--------|---------|-------|
| 6-digit hex | `#ff6600` | Standard `#rrggbb` |
| 3-digit hex | `#f60` | Expands to `#ff6600` |
| 8-digit hex | `#ff660080` | `#rrggbbaa` with alpha channel |
| 4-digit hex | `#f608` | `#rgba` with alpha channel |
| Named color | `red`, `cornflowerblue` | Full W3C named color list |
| `rgb()` | `rgb(255, 102, 0)` | Space or comma separated |
| `rgba()` | `rgba(255, 102, 0, 0.5)` | Fourth value is alpha 0–1 |
| `hsl()` | `hsl(24, 100%, 50%)` | Hue, saturation, lightness |
| `hsla()` | `hsla(24, 100%, 50%, 0.5)` | With alpha |
| `hwb()` | `hwb(24 0% 0%)` | CSS Color Level 4 |
| `transparent` | `transparent` | Fully transparent (renders as black) |

Color values are downsampled to the terminal's color capability at render time:
- **TrueColor terminals** (`COLORTERM=truecolor`): full 24-bit RGB
- **256-color terminals**: quantized to the nearest xterm-256 palette entry
- **16-color terminals**: quantized to the nearest ANSI basic color
- **No color** (`NO_COLOR=1` or non-TTY): color is stripped

Bare ANSI index numbers (e.g. `"214"`) are not supported; use `#rrggbb` or a named color instead.

---

## What Is Not Implemented

- `px`, `em`, `rem`, `vw`, `vh`, and other CSS units (ignored; use bare integers or `ch`)
- CSS math functions: `calc()`, `min()`, `max()`, `clamp()`
- CSS variables (`--my-var`)
- Media queries (`@media`)
- `@font-face`, `@keyframes`, `@import`, `@charset`, `@supports`, `@page`, or any other at-rule — the parser recognizes any `@`-rule and skips it as a unit (its prelude, and its `{ ... }` body if it has one, including any rules nested inside that body), so an at-rule the renderer doesn't understand is simply ignored rather than corrupting whatever rule follows it in the same stylesheet
- Pseudo-classes and pseudo-elements
- The two-value `<width> <style>` form (no color) of `border`/`border-top`/`border-right`/`border-bottom`/`border-left` — see those sections
- `display: grid`, `display: list-item`, or any other display values beyond `block`, `inline`, `inline-block`, `flex`, `inline-flex`, `table`, `contents`, and `none`
- `flex-wrap`, `align-content`, and applied `flex-shrink` — see [Flexbox](#flexbox)'s "Not supported" for the full list and why
- `grid`, and `position: sticky` (`relative`/`absolute`/`fixed` are supported — see [`position`](#position)); shrink-to-fit auto-sizing and the "static position" algorithm for `absolute`/`fixed` (see that section's deviations)
- Multi-line cell content when `white-space: nowrap` is set on a `td`/`th`
- `border-collapse: collapse`'s conflict resolution doesn't consult `tr`/`thead`/`tbody`/`tfoot` `border` (`col`/`colgroup` are consulted, via real conflict resolution against their column's cells) — see `docs/TABLES.md`
- The legacy HTML `border`/`cellpadding`/`cellspacing` presentational attributes on `<table>` (use CSS `border-collapse`/`border-spacing`/cell `padding` instead)
- The 4 corner-shape `border-*-junction` properties on individual `th`/`td` cells (redundant with cell-level `border-*-corner`), and a `border-junctions` shorthand — see `docs/TABLES.md`
