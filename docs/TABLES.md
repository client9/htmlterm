# Styling Tables

`<table>` is one of the more complicated corners of htmlterm — real HTML
table CSS is already complicated, and rendering it as character-grid ASCII
art adds its own layer on top (there's no pixel-perfect equivalent of a table
border, only a chosen set of glyphs). This page pulls together everything
about styling a table's frame and cells in one place; CSS.md's own table
entries are one-line pointers here.

## Quick reference

| Property | Applies to | Notes |
|---|---|---|
| `border-style` | `<table>` | Whole-frame preset — see below. Not a real-CSS property name reused; htmlterm's own vocabulary. |
| `border-top`/`-right`/`-bottom`/`-left` | `<table>` | Literal glyph or shorthand grammar, same as block elements |
| `border-top-mid`/`-bottom-mid`/`-left-mid`/`-right-mid`/`border-center` | `<table>` | Junction glyph overrides — no real-CSS equivalent |
| `border-top-left-corner` etc. | `<table>` | Corner glyph overrides |
| `border-header`/`border-columns`/`border-rows` | `<table>` | On/off edge toggles — no real-CSS equivalent |
| `border-color`, `border-*-color` | `<table>` | Whole-frame / per-edge color |
| `margin`, `padding` | `<table>` | Work like any block element — see "Margin and padding" below |
| `width`/`min-width`/`max-width` | `<th>`/`<td>` | Column sizing |
| `white-space`, `text-overflow` | `<th>`/`<td>` | Wrapping vs. truncation |
| `vertical-align` | `<th>`/`<td>` | Content placement within row height |
| `caption-side` | `<table>` | Caption above/below the frame |
| `border-collapse: separate` | `<table>` | Opt-in: real per-cell `border`/`border-style`/etc. on `<th>`/`<td>`, real CSS semantics — see "`border-collapse: separate`" below |
| `border-spacing` | `<table>` | Gap between cell boxes, only meaningful under `border-collapse: separate` |

## Border-style presets

**Not a real CSS property value set.** Real CSS's `border-style` is a
per-edge line-style keyword (`solid`/`dashed`/`dotted`/`groove`/`ridge`/
`inset`/`outset`/`double`/`none`/`hidden`) — htmlterm's `border-style` on
`<table>` instead names a *complete whole-table-frame preset*
(`solid`/`rounded`/`heavy`/`double`/`markdown`/`standard`/`hidden`/`none`).
Only the *concept* of "solid"/"double"/"hidden"/"none" carries the name
over from real CSS; `rounded`/`heavy`/`markdown`/`standard` are
htmlterm-specific, and even the overlapping names pick a specific
box-drawing character set rather than a line style. This is a deliberate
design choice, not an accident: writing out a dozen individual glyph
properties for every table would be unusable, and the realistic set of
table "looks" is small enough that a handful of named presets covers it.
Individual `border-*` properties on the same element still override the
preset for that one edge.

```css
table { border-style: solid; }   /* default */
```
```
┌──────┬───┐
│Name  │Qty│
├──────┼───┤
│Apple │3  │
│Banana│5  │
└──────┴───┘
```

```css
table { border-style: rounded; }
```
```
╭──────┬───╮
│Name  │Qty│
├──────┼───┤
│Apple │3  │
│Banana│5  │
╰──────┴───╯
```

```css
table { border-style: heavy; }
```
```
┏━━━━━━┳━━━┓
┃Name  ┃Qty┃
┣━━━━━━╋━━━┫
┃Apple ┃3  ┃
┃Banana┃5  ┃
┗━━━━━━┻━━━┛
```

```css
table { border-style: double; }
```
```
╔══════╦═══╗
║Name  ║Qty║
╠══════╬═══╣
║Apple ║3  ║
║Banana║5  ║
╚══════╩═══╝
```

```css
table { border-style: markdown; }
```
```
|Name  |Qty|
|------|---|
|Apple |3  |
|Banana|5  |
```

```css
table { border-style: standard; }  /* no outer frame, no column rules */
```
```
Name   Qty
────── ───
Apple  3  
Banana 5  
```

```css
table { border-style: hidden; }  /* (or "none") no borders at all */
```
```
Name   Qty
Apple  3  
Banana 5  
```

## Edge toggles

**Not real CSS properties.** `border-header`, `border-columns`, and
`border-rows` have no equivalent at all in the spec — real CSS has no
on/off toggle for a table's internal rule lines. They exist here because
htmlterm's table border model has no per-cell `border` at all: `<th>`/`<td>`
never read a `border*` declaration in table-layout mode (only `<table>`
itself does — see "Border-style presets" above), so there's no independent
cell edge to set to `none` the way real CSS would turn off one row's or
column's divider. In real CSS you'd get "no header divider" by setting
`border-bottom: none` on the header cells, or "no column dividers" via
`border-collapse` and per-cell `border: none` — options that don't exist
here since cells don't carry their own border in the first place. These
three properties are the table-level escape hatch that compensates for
that: on/off switches for the internal rule lines a whole-frame preset
draws by default, since there's no cell-level lever to pull instead.

```css
table { border-rows: solid; }   /* off by default */
```
```
┌──────┬───┐
│Name  │Qty│
├──────┼───┤
│Apple │3  │
├──────┼───┤
│Banana│5  │
└──────┴───┘
```

```css
table { border-header: none; }
```
```
┌──────┬───┐
│Name  │Qty│
│Apple │3  │
│Banana│5  │
└──────┴───┘
```

```css
table { border-columns: none; }
```
```
┌──────┬───┐
│Name  Qty│
├──────┼───┤
│Apple 3  │
│Banana5  │
└──────┴───┘
```

Note in that last example: the top/bottom border still draws the `┬`/`┴`
junction glyphs even though the column divider itself is gone (a minor,
harmless visual inconsistency — `border-columns: none` only removes the
vertical rule in the body, not the outer frame's own junction characters).

## Junction and corner glyph overrides

`border-top-mid`/`border-bottom-mid`/`border-left-mid`/`border-right-mid`/
`border-center` override the individual T-junction and cross-junction
characters a preset supplies — again, no real-CSS analog (CSS has no notion
of styling where a divider "joins" a border):

```css
table {
  border-top-mid: "╥"; border-bottom-mid: "╨";
  border-left-mid: "╠"; border-right-mid: "╣";
  border-center: "╫";
}
```
```
┌──────╥───┐
│Name  │Qty│
╠──────╫───╣
│Apple │3  │
│Banana│5  │
└──────╨───┘
```

`border-top-left-corner`/`border-top-right-corner`/
`border-bottom-left-corner`/`border-bottom-right-corner` override one outer
corner character each, the same literal-only model as the identically-named
block-element properties.

**A deviation from literal CSS semantics worth calling out explicitly:**
setting `border-left: none` (or `border-right: none`) doesn't just remove
that one vertical rule — it also clears that side's corner and every
internal junction glyph (header divider, row dividers) on every horizontal
line, not just the outer frame:

```css
table { border-left: none; }
```
```
──────┬───┐
Name  │Qty│
──────┼───┤
Apple │3  │
Banana│5  │
──────┴───┘
```

Real CSS's `border-left: none` never touches the top/bottom border at all —
in pixel rendering there's no "corner glyph" concept to begin with, since a
border is just where two independent lines happen to meet in space. In
ASCII art, though, leaving the `┌`/`├`/`└` corner and junction glyphs in
place with no vertical rule for them to connect to would look like a
dangling, disconnected character — so removing them is the deliberate,
intentional interpretation of "no left frame at all" for this renderer, not
an oversight. If you want a literal one-sided-rule look without touching the
rest of the frame, use the corner-override properties above to substitute a
plain fill character instead of `none`.

## `border-collapse: separate`

Everything above (border-style presets, edge toggles, junction/corner
glyphs) describes htmlterm's **default** table rendering: one shared frame
drawn from a preset, with cells never reading their own `border*` CSS at
all — which is also why `border-header`/`border-columns`/`border-rows`
exist as non-standard escape hatches (see "Edge toggles" above).

Setting `border-collapse: separate` on `<table>` switches to a completely
different, **opt-in** model: every `<th>`/`<td>` gets its own real,
independent border box — ordinary `border`/`border-style`/`border-color`
CSS on the cell itself, resolved exactly the same way as on any other
element — with `border-spacing` as the gap between adjacent cell boxes
(and between the table's own border and its outermost cells). This is real
CSS, not htmlterm-invented vocabulary: `border-header`/`border-columns`/
`border-rows` have no meaning here, since there's no shared frame to toggle
pieces of — you just don't set a border on a particular cell's edge,
the same way you would on any block element.

```css
table { border-collapse: separate; border-spacing: 1; }
td, th { border: solid; padding-left: 1; padding-right: 1; }
```
```

 ┌────────┐ ┌─────┐ 
 │ Name   │ │ Qty │ 
 └────────┘ └─────┘ 

 ┌────────┐ ┌─────┐ 
 │ Apple  │ │ 3   │ 
 └────────┘ └─────┘ 

 ┌────────┐ ┌─────┐ 
 │ Banana │ │ 5   │ 
 └────────┘ └─────┘ 

```

`border-spacing` takes one or two values — one applies to both axes; two
are horizontal then vertical (the opposite order from `gap`'s row-then-
column convention):

```css
border-spacing: 1;      /* 1 column and 1 row of spacing */
border-spacing: 2 1;    /* 2 columns horizontal, 1 row vertical */
border-spacing: 0;      /* cell borders touch directly, no gap */
```

Cells with no border of their own render with no box at all — row height
still equalizes across the row (a plain cell just gets blank padding to
match its bordered neighbor's height), the same as a mix of bordered and
unbordered block elements would:

```css
table { border-collapse: separate; border-spacing: 1; }
```
```html
<tr><td style="border:solid">bordered</td><td>plain</td></tr>
```
```

 ┌────────┐ plain 
 │bordered│       
 └────────┘       

```

`colspan`/`rowspan` are supported: a spanning cell's own border box covers
every column/row it spans, including the `border-spacing` gap(s) between
them (its border/background runs continuously through what would otherwise
be a gap — there's no separate divider drawn through the middle of a single
cell's own box, matching real CSS). `vertical-align`/`text-align` apply per
cell exactly as they do in the default model. The table's own
`border`/`padding`/`margin` still wrap the whole assembled grid, exactly
like any other block element.

**Known simplifications** (documented, not accidental):

- **Column-width allocation for border thickness uses one representative
  cell per column** (that column's header cell if the table has one, else
  the first row's cell there) — if cells within the same column set
  inconsistent border widths, their total rendered widths may not perfectly
  align. Matches how a real browser looks when a table's cell borders are
  genuinely inconsistent (an unusual authoring pattern).
- **A `colspan` cell's available content width doesn't reclaim the interior
  per-column border width** it swallows (only the `border-spacing` gap
  between the columns it spans) — its own box may end up very slightly
  narrower than the exact combined width of the columns below/above it,
  padded with blank space to stay rectangular. Only noticeable with
  unusually thick borders combined with colspan.
- **`border-collapse: collapse`** — the other half of real CSS's model
  (adjacent cell borders merging into shared lines via conflict
  resolution) is not implemented; `collapse` and unset both keep today's
  default shared-frame rendering unchanged.

## Margin and padding

`margin`/`padding` on `<table>` itself work exactly like any other block
element — including the case that's easy to assume doesn't apply to tables
at all:

```css
table { padding: 1; border-style: solid; }
```

Padding adds blank space *inside* the border, between the frame and the
outermost cells — this is genuinely how real CSS's `border-collapse:
separate` (the actual default, contrary to the common assumption that
tables have no meaningful padding) behaves too; it's just rarely used
because most real-world tables reach for `border-spacing`/cell padding
instead of table-level `padding`. `margin-left`/`margin-right`, including
`margin: auto` centering, work the same way they do for other block
elements (see CSS.md's `margin` entries) — and `auto`-centering a `<table>`
specifically is one of the few auto-margin cases real browsers support
without requiring extra tricks, which htmlterm matches.

`margin-top`/`margin-bottom` also work like any other block element:
collapsing with adjacent siblings' margins the same way (larger value wins),
whether the `<table>` is at the document root or nested inside another
block. All of this applies identically whether or not
`border-collapse: separate` is set — the table's own margin/padding always
wrap the assembled grid the same way, regardless of border model.

## Not supported

- **`border-collapse: collapse`** — see the previous section; real
  per-cell border-conflict resolution and junction-glyph synthesis aren't
  implemented, only `separate`.
- **The default (shared-frame) model has no `border-spacing`/per-cell
  border at all** — that's exactly what opting into
  `border-collapse: separate` gets you instead; see above.
- **Multi-line cell content combined with `white-space: nowrap`** — a
  `nowrap` cell is always clipped to one line (see `text-overflow`), never
  both non-wrapping and multi-line. Applies under both border models.

## See Also

- CSS.md's `border`, `margin`, `padding`, `width` entries — the same
  properties and value forms apply to `<table>`/`<th>`/`<td>` as to any
  other element, documented once there rather than duplicated here.
- `docs/RENDERING.md` for how table layout fits into the wider box-model
  pipeline.
