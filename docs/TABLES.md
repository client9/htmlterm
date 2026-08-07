# Styling Tables

`<table>` is one of the more complicated parts of htmlterm. Real HTML table
CSS is already complicated, and rendering it as character-grid ASCII art adds
another layer. There is no pixel-perfect equivalent of a table border, only
a chosen set of glyphs. This page collects the rules for styling a table's
frame and cells in one place. CSS.md's table entries are one-line pointers
here.

**A bare `<table>` with no CSS at all renders borderless**,
matching a real browser exactly (`border-style`'s initial value is `none`;
`border-collapse`'s initial value is `separate`). There is no hidden
fallback "default look" here — every visible border comes from CSS you
actually wrote.

## Quick reference

| Property | Applies to | Notes |
|---|---|---|
| `border-style` | `<table>`, `<th>`/`<td>` | Whole-box preset, see below. Not a real-CSS property value set; htmlterm's own vocabulary |
| `border-top`/`-right`/`-bottom`/`-left` | `<table>`, `<th>`/`<td>` | Literal glyph or shorthand grammar, same as block elements |
| `border-top-left-corner` etc. | `<table>`, `<th>`/`<td>` | Corner glyph overrides |
| `border-top-junction`/`-right`/`-bottom`/`-left`/`-center` | `<table>`, `<th>`/`<td>` | Literal junction-glyph overrides (T-shapes/cross) under `border-collapse: collapse` — see "Junction and corner overrides" below |
| `border-top-left-junction` etc. | `<table>` only | Literal junction-glyph overrides (corner-shapes anywhere in the grid) under `border-collapse: collapse` — no cell-level equivalent (redundant with cell-level `border-*-corner`) |
| `border-color`, `border-*-color` | `<table>`, `<th>`/`<td>` | Whole-box / per-edge color |
| `margin`, `padding` | `<table>` | Work like any block element — see "Margin and padding" below |
| `width`/`min-width`/`max-width` | `<th>`/`<td>` | Column sizing. `box-sizing` isn't read here — see the note right below this table |
| `white-space`, `text-overflow` | `<th>`/`<td>` | Wrapping vs. truncation |
| `vertical-align` | `<th>`/`<td>` | Content placement within row height |
| `caption-side` | `<table>` | Caption above/below the table |
| `border-collapse: separate` | `<table>` | The real-CSS default, and what unset falls back to. Every `<th>`/`<td>` gets its own independent border box. See "`border-collapse: separate`" below |
| `border-collapse: collapse` | `<table>` | Adjacent cell and table borders merge into shared lines via conflict resolution. See "`border-collapse: collapse`" below |
| `border-spacing` | `<table>` | Gap between cell boxes under `border-collapse: separate`. Default `0` |

A `<th>`/`<td>`'s `box-sizing` isn't read at all. This is a compatibility gap
against spec, not a spec exception: real CSS applies `box-sizing` to table
cells the same as any other box. The column-width algorithm above resolves
a declared `width` through its own mechanism instead, one that already
matches neither real `box-sizing` value on its own. It's border-box-shaped
for padding, subtracted from the declared width. It's content-box-shaped
for border characters, added on top as separate overhead, per the border
unification model described throughout this page. See CSS.md's
`box-sizing` entry and COMPATIBILITY.md for the full account.

## Border-style presets

**Not a real CSS property value set.** Real CSS's `border-style` is a
per-edge line-style keyword (`solid`/`dashed`/`dotted`/`groove`/`ridge`/
`inset`/`outset`/`double`/`none`/`hidden`) — htmlterm's `border-style`
instead names a *complete glyph-set preset*
(`solid`/`rounded`/`heavy`/`double`/`markdown`/`standard`/`hidden`/`none`).
Only the *concept* of "solid"/"double"/"hidden"/"none" carries the name
over from real CSS. `rounded`/`heavy`/`markdown`/`standard` are
htmlterm-specific, and the overlapping names pick a specific box-drawing
character set rather than a line style. This is deliberate. Writing out a
dozen individual glyph properties for every border would be unusable, and
the realistic set of "looks" is small enough that a handful of named presets
covers it.
Individual `border-*` properties on the same element still override the
preset for that edge. This property is not table-specific. It is the same one
any block element uses, and a `<table>`/`<th>`/`<td>` box uses it like any
other bordered box.

The examples below all use `border-collapse: collapse` with the same
`border-style` set on the table and its cells. That is how you get a full
shared grid out of these presets, and the "`border-collapse: collapse`"
section below explains why a table-level preset alone does not reach into
its cells:

```css
table, td, th { border-style: solid; }
table { border-collapse: collapse; }
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
table, td, th { border-style: rounded; }
table { border-collapse: collapse; }
```
```
╭──────┬───╮
│Name  │Qty│
├──────┼───┤
│Apple │3  │
├──────┼───┤
│Banana│5  │
╰──────┴───╯
```

```css
table, td, th { border-style: heavy; }
table { border-collapse: collapse; }
```
```
┏━━━━━━┳━━━┓
┃Name  ┃Qty┃
┣━━━━━━╋━━━┫
┃Apple ┃3  ┃
┣━━━━━━╋━━━┫
┃Banana┃5  ┃
┗━━━━━━┻━━━┛
```

```css
table, td, th { border-style: double; }
table { border-collapse: collapse; }
```
```
╔══════╦═══╗
║Name  ║Qty║
╠══════╬═══╣
║Apple ║3  ║
╠══════╬═══╣
║Banana║5  ║
╚══════╩═══╝
```

```css
table, td, th { border-style: markdown; }
table { border-collapse: collapse; }
```
```
|Name  |Qty|
|Apple |3  |
|Banana|5  |
```

`markdown` only supplies left/right glyphs, so it never draws a row divider
on its own. That includes the header-separator row Markdown tables have. Add
an explicit `border-bottom` on the cells that need one if you want a divider.

```css
table, td, th { border-style: standard; }
table { border-collapse: collapse; }
```
```
Name  Qty
Apple 3  
Banana5  
```

`standard` supplies no glyphs on any edge at all. It is a documented no-op
preset, useful mainly as an explicit "no border" marker distinct from simply
not setting `border-style`. With no border glyphs anywhere, there is also no
`border-spacing` gap to space columns apart under `collapse`, which is a
`separate`-mode-only property. That is why `Banana` and `5` touch directly
above.

## Corner glyph overrides

`border-top-left-corner`/`border-top-right-corner`/
`border-bottom-left-corner`/`border-bottom-right-corner` override one outer
corner character each. This is a literal-glyph-only property, with no preset
keywords, and it uses the same model on `<table>`/`<th>`/`<td>` as on any
other block element:

```css
table {
  border-style: solid; border-spacing: 0;
  border-top-left-corner: '1'; border-top-right-corner: '2';
  border-bottom-left-corner: '3'; border-bottom-right-corner: '4';
}
```
```
1───2
│A  │
3───4
```

**`border-left: none`/`border-right: none` keep that side's corners** even
though the vertical rule itself disappears:

```css
table { border-style: solid; border-left: none; border-spacing: 0; }
```
```
┌─────┐
A  B  │
└─────┘
```

This matches how any other bordered block element handles `border-left:
none` in this renderer. A corner glyph is generated independently for each
corner from whichever of its two adjacent edges resolve to a real border.
Here, top and bottom still do, so the corner remains.

## `border-collapse: separate`

**The real-CSS default** (what unset `border-collapse` falls back to, since
`separate` *is* CSS's own initial value): every `<th>`/`<td>` gets its own
real, independent border box. Ordinary `border`/`border-style`/
`border-color` CSS on the cell itself resolves the same way as on any other
element, and `border-spacing` is the gap between adjacent cell boxes and
between the table's own border and its outermost cells. A table with zero
columns can still render borderless, because no CSS anywhere means no cell
gets a border box.

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

`border-spacing` takes one or two values. One applies to both axes; two are
horizontal then vertical, the opposite order from `gap`'s row-then-column
convention. **Default is `0`.** Real CSS defaults to `2px`, which is
imperceptible against a typical line-height. But a terminal "row" of spacing
is a full line, 100% of a row's height rather than the ~10% a pixel gap
would be against real text, so a literal `1`-character default would read as
a more prominent gap than the default it approximates. It would also look
like leftover border structure even with no border set anywhere:

```css
border-spacing: 0;      /* cell borders touch directly, no gap (the default) */
border-spacing: 1;      /* 1 column and 1 row of spacing */
border-spacing: 2 1;    /* 2 columns horizontal, 1 row vertical */
```

Cells with no border of their own render with no box at all. Row height
still equalizes across the row, because a plain cell gets blank padding to
match its bordered neighbor's height, the same as a mix of bordered and
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

`colspan`/`rowspan` are supported. A spanning cell's own border box covers
every column or row it spans, including the `border-spacing` gaps between
them. Its border and background run continuously through what would
otherwise be a gap, and no separate divider is drawn through the middle of a
single cell's own box, matching real CSS. `vertical-align` and `text-align`
apply per cell as they do outside a table. The table's own `border`,
`padding`, and `margin` still wrap the assembled grid, like any other block
element. A table's own `padding` works. See "Margin and padding" below.

**Known simplifications** (documented, not accidental):

- **Column-width allocation for border thickness uses one representative
  cell per column** (that column's header cell if the table has one, else the
  first row's cell there). If cells within the same column set inconsistent
  border widths, their total rendered widths may not align perfectly. That
  matches how a real browser looks when a table's cell borders are
  inconsistent, which is unusual.
- **A `colspan` cell's available content width doesn't reclaim the interior
  per-column border width** it swallows, only the `border-spacing` gap
  between the columns it spans. Its own box may end up slightly narrower
  than the combined width of the columns below or above it, padded with
  blank space to stay rectangular. This is only noticeable with thick borders
  combined with colspan.

## `border-collapse: collapse`

The other real CSS border model is when adjacent cell and table borders
merge into **shared grid lines** instead of each cell keeping its own
independent box. Real CSS's conflict-resolution algorithm (CSS 2.1
§17.6.2.1) picks a single winner for each shared line segment from whichever
elements border it. htmlterm implements the same idea, adapted for its style
vocabulary and lack of real border thickness:

1. **`hidden` always wins** — a `hidden` edge on either side suppresses that
   shared segment outright, regardless of what the other side sets.
2. **An edge with no border set (or explicit `none`) simply loses**. If only
   one side has a real border, it wins by default. If neither does, the
   segment is absent entirely and consumes no space at all. A collapsed-mode
   table with no borders anywhere is as borderless as a `separate`-mode one
   with none.
3. **Otherwise, style precedence decides**: `double` > `heavy` > `solid` >
   `rounded` > `markdown`/`standard`/any literal-glyph style (real CSS's own
   `border-width` "widest wins" tier is skipped — border thickness is a
   no-op deviation here, documented in COMPATIBILITY.md).
4. **A same-style tie goes to the cell over the table**. This matches real
   CSS's own "cell beats table" rule at the tail of its hierarchy. For two
   adjacent cells' own borders, a case real CSS does not precisely specify,
   the tie goes to the *earlier* element in reading order. The row above
   wins a horizontal tie, and the column to the left wins a vertical one.
   Verified against Firefox, Safari, and Chrome, all three converge on the
   same behavior.

**Worked example of that tie-break:**

```css
th { border-bottom: "═"; }
td { border-top: "╌"; }
```

Both are literal glyphs, which always share the same lowest precedence tier
regardless of glyph (rule 3). So at the boundary between a header row and
the body row below it, this is a tie, and rule 4 picks the *earlier* cell:
`th`'s `═` wins, matching how a real browser renders this. If you want the
*later* cell to win instead, only declare the edge on that side and leave the
earlier side's edge unset. With `th` no longer setting `border-bottom`,
rule 2 applies instead of rule 4, because only one side has a real border.

```css
/* th leaves border-bottom unset */
td { border-top: "╌"; }
```

Each grid vertex, where up to 4 segments meet, gets a junction glyph
synthesized from which of its arms are present and which style won each.
`solid`/`heavy`/`double` each have a complete box-drawing character set that
covers every combination. `rounded` reuses `solid`'s interior junctions and
uses its own glyphs only at the four outer corners. `markdown`/`standard`/
an unnamed literal-glyph style have no junction glyphs of their own, so a
vertex where one of them won renders as a **blank space** instead. That is
deliberate, not a gap to fill later. Borrowing a box-drawing character from
an unrelated style, say solid's `┼` in the middle of an otherwise-ASCII
`|`/`-` table, would look like an accidental mismatch, worse than no glyph at
all. See "Junction and corner overrides" below for how to supply a real
character there instead. When a vertex's arms won with different styles,
the highest-precedence style among them picks which style's junction table to
consult for that glyph. It is an approximation, not an attempt at an exact
mixed-weight Unicode character for every combination. For example, a real
single-weight vertical divider meeting a double-weight horizontal one would
want a mixed glyph like `╞`; this renders the whole vertex in double's own
glyph set instead.

```css
table { border-collapse: collapse; }
td, th { border: solid; }
```
```html
<table><tr><th colspan="2">Header</th></tr><tr><td>A</td><td>B</td></tr></table>
```
```
┌───────┐
│Header │
├───┬───┤
│A  │B  │
└───┴───┘
```

Note the header cell's own interior. Since it spans both columns, no divider
is drawn through the middle of its box even though the column boundary is
active below it, matching real CSS (`colspan`/`rowspan` suppress the
interior grid-line segments they cover; only a spanning cell's outer edges
participate in conflict resolution).

### How much width the grid lines take

A collapsed table's columns are sized against the space its **grid lines**
will occupy, one column per *active* vertical line. A line is active if any
row draws a border there. A boundary nobody borders consumes no space. There
are `columns + 1` boundaries, so an N-column table of fully bordered cells
spends N+1 columns on grid lines, not 2N. The border between two cells is a
single shared column that both sides resolved, and the outermost two come
from whichever of the table's or the edge cells' borders won there.

That count is exact, not estimated. Conflict resolution compares border
styles and widths, never sizes, so the whole grid can be resolved before any
column is measured. It matters in both directions. Sizing a collapsed table
as if each cell kept its own two borders (`separate`'s model) leaves a
`width: 100%` table one column short per interior boundary. Sizing it from
its cells' borders alone misses the two outer lines a table-level `border`
contributes, and the table then overruns its width, at full width and past
the terminal.

## Junction and corner overrides

`border-*-corner` (`border-top-left-corner`/`border-top-right-corner`/
`border-bottom-left-corner`/`border-bottom-right-corner`) works under
`border-collapse: collapse` on **both** `<table>` and `<th>`/`<td>`, the
same as every other border property. On the table, it is the table's own 4
outer corners:

```css
table {
  border-collapse: collapse; border-style: solid;
  border-top-left-corner: '1'; border-top-right-corner: '2';
  border-bottom-left-corner: '3'; border-bottom-right-corner: '4';
}
```
```
1───2
│A  │
3───4
```

On a cell, it means the same thing at cell scope. That cell's corner applies
wherever it lands. A single cell only has 4 corners total, so there is no
interior, non-corner position belonging to just one cell the way the whole
table has. No separate cell-level property is needed for this, and it is the
same `border-top-left-corner` you would set on any other bordered box.

For a glyph anywhere else in the grid, such as a T-junction, a cross, or a
corner-shape that is not the table's own outer corner, 9 properties supply a
literal override, one per arm-combination shape. They are named after
`border-top-left-corner`'s word order, with location first.
`border-top-junction`/`border-right-junction`/`border-bottom-junction`/
`border-left-junction` are the 4 T-shapes, named for which side of the table
the divider meets, not which arm is missing. `border-center-junction` is the
interior 4-arm cross, and `border-top-left-junction`/`border-top-right-junction`/
`border-bottom-left-junction`/`border-bottom-right-junction` are the corner
shapes. **The 5 T-shape/cross properties work on both `<table>` and `<th>`/
`<td>`**. A cell's own version applies only at its own edges, wherever that
shape actually occurs there. **The 4 corner-shape junction properties are
table-only**. Once cell-level `border-*-corner` covers "my own corner,
whichever shape lands there," a cell-level `border-top-left-junction` would
mean the same thing, so there is no separate property for it. At table
scope, the two remain different, one literal position versus every
occurrence of that shape, which is why both exist there.

```css
table { border-collapse: collapse; border-top: "="; border-top-junction: '+'; }
td { border-left: "|"; border-right: "|"; }
```
```html
<table><tr><td>A</td><td>B</td></tr></table>
```
```
 =+= 
|A|B|
```

This is the escape hatch for the "blank space" case above. A
`markdown`/`standard`/literal-glyph border gets a real junction character by
naming it directly, rather than the engine trying to guess one from an
unrelated style's glyph set. Each is a plain CSS property with quoted-string
values only, and there is no named-preset keyword support, matching
`border-*-corner`'s literal-only grammar. `unset`/`inherit`/`initial` all
work on them with no special handling, just like any other property. There is
currently no shorthand to set several at once. Write out the ones you need.

**Precedence, most specific first:**

1. A cell's own override, at its own corner or edge. Up to 4 cells can share
   one interior vertex. If more than one sets a conflicting override for the
   same shape there, the earlier cell in reading order wins, using the same
   rule that resolves same-type edge ties. See "`border-collapse: collapse`"
   above.
2. `border-*-corner` set on the table — only at the table's own true 4
   outer corners.
3. `border-*-junction` set on the table — wherever that shape occurs
   anywhere in the grid.
4. The computed glyph from whichever style actually won the surrounding
   edges (the pre-override behavior, unchanged).

That order matters for correctness, not just priority. An earlier version of
the table-level corner check read a table's own `border-style` preset's
corner glyph as a fallback even when a *cell's* higher-precedence style had
won the surrounding edges, producing a visibly mismatched corner, for
example a `rounded` corner around a `heavy` box if the table set `rounded`
but a cell's own `heavy` correctly won the edges. The corner/junction
override tiers now only consult an explicit literal value. The computed
fallback (tier 4) is the only place a *style's own* glyph comes from, and it
already reflects whichever style won.

**Real-world motivating example**: comfy-table-style ASCII tables (see
https://github.com/Nukesor/comfy-table) give their header/body divider a
distinct look — a seamless `=` run with `+` ends — while ordinary row
dividers use `|`/`-`/`+`. Both dividers share the exact same arm-shapes
(only which divider they are differs), so this is only reachable with
per-cell overrides:

```css
table {
  border-collapse: collapse; border: none;
  border-top: "-"; border-bottom: "-";
  border-center-junction: "+"; border-top-junction: "+"; border-bottom-junction: "+";
  border-left-junction: "|"; border-right-junction: "|";
  border-top-left-corner: "+"; border-top-right-corner: "+";
  border-bottom-left-corner: "+"; border-bottom-right-corner: "+";
}
th, td { border-left: "|"; border-right: "|"; }
th {
  border-bottom: "=";
  border-bottom-left-corner: "+"; border-bottom-right-corner: "+";
  border-center-junction: "="; border-bottom-junction: "=";
  border-left-junction: "+"; border-right-junction: "+";
}
td { border-bottom: "-"; }
```
```
+--+--+
|H1|H2|
+=====+
|a |b |
|--+--|
|c |d |
+--+--+
```

**Scope of what's compared, as currently implemented**: **cell-level,
table-level, and `<col>`/`<colgroup>`** `border` declarations are
considered. A `<col>`'s own border is resolved against the cells in its
column using the same style-precedence conflict rules as any other pair of
candidates. A `<col>`'s higher-precedence style, for example `double`, wins
over a cell's lower-precedence style, for example `solid`, even if the cell
also declares that exact edge. Style precedence outranks element-type
precedence, matching real CSS's own tier ordering. `<tr>`/`<thead>`/
`<tbody>`/`<tfoot>` borders are not consulted, even though real CSS's
collapse algorithm does include them in its full hierarchy. Those elements
are real DOM ancestors of their cells, unlike `<col>`/`<colgroup>`, which sit
outside the row/cell tree and need this resolution to reach their column's
cells at all. Border on `tr`/`thead`/`tbody`/`tfoot` also has no effect under
real CSS's `border-collapse: separate`. Most real-world markup styles cell
borders directly for that cross-browser-inconsistency reason, so it was not
judged worth the added conflict-resolution complexity. The legacy HTML
`border`/`cellpadding`/`cellspacing` presentational attributes aren't read
either. Use the CSS properties above.

**A real-CSS deviation worth calling out**: under `border-collapse:
collapse`, the table's own `padding` has no effect, matching real CSS. Only
the table's own `margin` and its own `border` apply at the table level, and
the border is merged into the grid like any other cell's rather than acting
as a separate wrapping frame.

## Margin and padding

`margin`/`padding` on `<table>` itself work like any other block element
under `border-collapse: separate` (the default), including the case that's
easy to assume does not apply to tables at all:

```css
table { padding: 1; border-style: solid; }
```

Padding adds blank space inside the border, between the frame and the
outermost cells. This is how real CSS's `border-collapse: separate` model
behaves too, but it is rarely used because most real-world tables reach for
`border-spacing` or cell padding instead of table-level `padding`.
`margin-left`/`margin-right`, including `margin: auto` centering, work the
same way they do for other block elements, see CSS.md's `margin` entries.
Auto-centering a `<table>` is one of the few auto-margin cases real browsers
support without extra tricks, which htmlterm matches. Under
`border-collapse: collapse`, `padding` on the table itself has no effect.
Only `margin` does.

`margin-top`/`margin-bottom` also work like any other block element:
collapsing with adjacent siblings' margins the same way (larger value wins),
whether the `<table>` is at the document root or nested inside another
block, under either border model.

## Not supported

- **`border-collapse: collapse`'s conflict resolution doesn't consult
  `<tr>`/`<thead>`/`<tbody>`/`<tfoot>` `border`** (`<col>`/`<colgroup>` are
  consulted, via conflict resolution against their column's cells); see
  above.
- **The legacy HTML `border`/`cellpadding`/`cellspacing` presentational
  attributes** aren't read — use `border-collapse`/`border-spacing`/cell
  `padding` instead.
- **Multi-line cell content combined with `white-space: nowrap`** — a
  `nowrap` cell is always clipped to one line (see `text-overflow`), never
  both non-wrapping and multi-line. Applies under both border models.
- **The 4 corner-shape `border-*-junction` properties have no cell-level
  equivalent** — table-only; see "Junction and corner overrides" above for
  why (a cell-level version would be redundant with cell-level
  `border-*-corner`).
- **No `border-junctions` shorthand** for setting several junction
  properties at once — a possible future addition, not built yet.

## See Also

- CSS.md's `border`, `margin`, `padding`, `width` entries. The same
  properties and value forms apply to `<table>`/`<th>`/`<td>` as to any
  other element, documented once there rather than duplicated here.
- `docs/RENDERING.md` for how table layout fits into the wider box-model
  pipeline.
