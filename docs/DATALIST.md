# `<datalist>` autocomplete

`<datalist>` supplies suggestions to a text `<input>`. The element itself is
never visible; what it produces is a filtered popup under the field as the
user types. That popup is not a real box in the layout tree, so it has its own
rules for which CSS applies, the same way `<select>`'s dropdown does (see
`docs/RENDERING.md`'s "Popups / z-order"). This page covers both; CSS.md's own
`datalist` entry is a one-line pointer here.

```html
<input list="fruits">
<datalist id="fruits">
  <option value="apple">
  <option value="apricot">
  <option value="banana">
</datalist>
```

## Quick reference

| Property | On `datalist` (the popup) | Per `<option>` (a row) |
|---|---|---|
| `background-color`, `color` | ✅ | ✅ (falls back to the popup's own value) |
| `border`, `border-style`, `border-*-color` | ✅ (around the whole list) | not supported |
| `padding` | ✅ (inside the border, around the whole list) | not supported |
| `margin-left`, `margin-top` | ✅ (shifts the popup relative to the field) | n/a |
| `margin-right`, `margin-bottom` | no effect (nothing follows a floating overlay to push against) | n/a |
| `width`, `min-width`, `max-width` | ✅ (overrides the natural width) | not supported (every row shares the popup's width) |
| `display` | already `none` from the UA sheet; the element never renders inline | n/a |
| `:hover` | n/a | ✅ — matches the arrow-key-highlighted row, not real mouse hover |
| `[disabled]` | n/a | ✅ — a disabled option is never suggested |

The popup is at least as wide as the field it drops out of, and grows to its
widest label beyond that. Width is measured in terminal **columns**, not runes,
so a double-width label (CJK, emoji) sizes the popup to the room it actually
needs and every row is padded to that same column width, which is what keeps
the highlight bar rectangular rather than ragged.

`:hover` is the same repurposed pseudo-class `<select>`'s popup uses, driven by
the same marker, so one `option:hover` rule styles both:

```css
datalist       { border-style: rounded; }
option:hover   { background-color: #4477cc; color: #ffffff; }
```

## Matching

Suggestions are filtered by a **case-insensitive prefix** test against each
option's `value`. Typing `ap` offers `apple` and `Apricot`, not `pineapple`.
An empty field matches everything, which is what makes ArrowDown on an untouched
field offer the whole list.

Chrome uses a substring test for datalist and Firefox a prefix one; prefix is
the less surprising of the two for a terminal autocomplete, where the list has
no scrollbar of its own. See `COMPATIBILITY.md`.

Only `internal/render` draws the popup; it never decides what belongs in it.
The `document` layer marks the matching options with a reserved attribute and
the renderer draws exactly those, so the matching rule exists in one place
rather than being duplicated across the two packages the way
`selectOptionNodes` is.

## A row's label

An option's row text is its `label` attribute, else its text content, else its
`value`. The `value` fallback is what makes the ordinary `<option value="x">`
form, with no text at all, render as anything: `<select>`'s own label function
stops at text content, because a `<select>` option practically always has some.

Whatever the row displays, picking it always inserts the option's `value`.

## Keys and clicks

| | |
|---|---|
| typing | filters and opens the popup; closes it when nothing matches |
| `ArrowDown` | opens a closed popup, or moves the highlight down |
| `ArrowUp` | moves the highlight up; does **not** open a closed popup |
| `Enter` | picks the highlighted row; with nothing highlighted, submits the form as usual |
| `Escape` | closes the popup, leaving the typed text alone |
| click a row | picks it |
| click elsewhere | dismisses the popup |
| focus leaving the field | dismisses the popup |

The highlight is clamped rather than wrapping, matching `<select>`'s. From a
freshly opened popup, one `ArrowDown` lands on the first row and one `ArrowUp`
on the last, so a single press always selects something.

Escape unwinds one layer per press: with a popup open inside a modal
`<dialog>`, the first Escape closes the popup and the second closes the dialog.

Picking a suggestion fires `"input"` and then `"change"`, in that order,
because it is both an edit and a commit. That differs from `<select>`'s
`confirmSelectPopup`, which fires `"change"` alone: a `<select>` has no
intermediate edit state to report, while a text entry does, and a caller
listening to `"input"` to track keystrokes would otherwise miss the largest
value change of all.

## Not supported

- **A second column for `<option label>`.** A browser can show the value and
  the label side by side; a row here shows one string (see "A row's label").
- **Substring matching**, or any author-selectable match rule.
- **`list` on a non-text `<input>`.** HTML also allows it on `range`, `color`,
  and the date and time types, which have no type-specific UI here at all (see
  `COMPATIBILITY.md`).
- **Per-option `border`, `padding`, or `width`.** Every row shares the popup's
  own, matching `<select>`'s popup.
- **`<optgroup>` inside a `<datalist>`.** HTML doesn't allow it either.
- **A scrollbar on a long list.** Every match is drawn; a list longer than the
  room below the field is clipped at the bottom under a fixed `Options.Height`,
  or grows the document when the height is natural.

## See also

- **`docs/SELECT.md`** — the other popup, which shares this one's box-drawing
  primitives and its `option:hover` marker.
- **`docs/RENDERING.md`**, "Popups / z-order" — the compositing model both use.
- **`COMPATIBILITY.md`** — the gaps above, stated against spec.
