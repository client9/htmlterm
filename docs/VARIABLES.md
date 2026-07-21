# CSS Custom Properties (`--foo` / `var()`) — Design & Implementation Plan

Status: **proposed, not implemented.** This document captures the evaluation
and plan for adding CSS custom properties to `internal/cssengine`. No code
has been written yet — this is the spec to implement against.

---

## Why this is worth doing

`internal/cssengine` already has most of the required machinery:

- `Cascade.Direct` / `Resolve` / `PseudoElement` are the *only* three entry
  points `internal/render` uses (`internal/render/cascade.go`), so var()
  support can be fully contained inside `cssengine` — zero changes needed
  in `internal/render`, `document`, or `tui`.
- `mergeCascade`/`!important` already treat every property name generically;
  `--foo` needs no new cascade-priority logic, just two special-cases
  (below).
- The existing `content:` mini-language (`attr()`, `counter()`,
  `open-quote`) already proves out "scan a value string for function-call
  tokens and substitute" as a pattern in this codebase.
- `splitCSSComponentValues`/`consumeCSSQuotedToken` in `css.go` already
  provide paren/quote-aware tokenizing that a `var()` scanner can reuse.

## Where the real cost is

1. **Custom property names are case-sensitive**, unlike every other
   property this engine handles. `commitDecl`/`parseDeclarationsWithImportance`
   in `css.go` currently `strings.ToLower()` every property name — this must
   special-case `--*` to preserve case (`--Foo` and `--foo` are distinct
   properties per spec).
2. **Inheritance is currently a fixed whitelist**
   (`inheritableProps` in `cascade.go`). Custom properties inherit
   unconditionally, by name (any `--*`), so the inheritance loop needs a
   `strings.HasPrefix(prop, "--")` fallback alongside the whitelist.
3. **Timing matters for performance.** `Resolve(n)` already walks ancestors
   once, calling `Direct(anc)` per ancestor. If var()-substitution needed
   its own ancestor walk *inside* `Direct()`, every `Direct(anc)` call in
   that existing walk would trigger another O(depth) walk — O(depth²) per
   node on deeply nested trees. Keeping substitution at the
   `Resolve()`/`PseudoElement()` level (after the existing ancestor loop has
   already flattened the custom-prop environment into a single map) avoids
   this — it costs nothing beyond what's already computed today.
4. **Cycles.** `--a: var(--b); --b: var(--a);` must not hang or
   stack-overflow. Needs a visited-set guard during a fixed-point resolution
   pass over the `--*` subset of a node's declarations.
5. **A structural gap that must be documented, not "solved".**
   `expandShorthand` runs once at *parse time*, before any per-node var()
   resolution is possible. So `margin: var(--gap) var(--gap)` (one var()
   per shorthand slot) works fine — each token is opaque to the whitespace
   splitter. But `margin: var(--sides)` expecting `--sides: 1 2 3 4` to fan
   out into four independent sides *cannot* work: the shorthand expander
   already collapsed to the single-token "all sides same" branch before the
   var ever has a value, so after substitution all four sides get the same
   literal string `"1 2 3 4"`. This is the same category of accepted
   limitation as the existing two-token `border: <width> <style>` gap
   (`css.go`, see the `border` case comment) and should be documented in
   `CSS.md` the same way.

## Scope decision: how far to chase inheritance accuracy

Three options were considered for how `Direct()` (used standalone by
`internal/render/counter.go` for `counter-reset`/`counter-increment`, which
are non-inherited properties by spec) should see custom properties:

| Option | Behavior | Cost |
|---|---|---|
| **A — Pragmatic (recommended)** | var() fully inherits through `Resolve()`/`PseudoElement()` (the paths render actually uses for element styling). `Direct()` only resolves var() against custom props declared on the *same* element — `counter-reset: var(--n)` works if `--n` is set on that element, not if only an ancestor sets it. | No perf risk, small diff. |
| B — Full ancestor-awareness everywhere | `Direct()` also does its own ancestor walk for custom props, so `counter-reset`/`counter-increment` see inherited vars too. | O(depth²) worst case unless memoization is added — meaningfully more surface area. |
| C — Skip counter properties entirely | Don't touch `Direct()` at all; var() never resolves in `counter-reset`/`counter-increment`, even same-element. | Simplest diff, but a plausible use case (variable-driven list start offset) silently no-ops. |

**Recommendation: Option A.** It covers the overwhelmingly common case
(theming colors, spacing, borders — all reached through `Resolve()`) at
zero performance risk, and still covers the same-element counter case,
which is the one part of Option C's gap that's cheap to close. This
document assumes Option A below; revisit if profiling ever shows it
matters.

## Design

### New file: `internal/cssengine/customprops.go`

```go
// isCustomProp reports whether name is a CSS custom property ("--foo").
func isCustomProp(name string) bool

// substituteVarTokens scans val for var(<name>[, <fallback>]) occurrences
// and replaces each with lookup(name)'s value, or the (recursively
// substituted) fallback text if lookup reports the name isn't defined.
// Paren/quote-aware: a fallback may itself contain nested var() calls,
// commas, and quoted strings. Per spec, only the first top-level comma
// inside var(...) is syntactic (ident/fallback separator) — everything
// after it up to the matching ')' is fallback text, including further
// commas.
func substituteVarTokens(val string, lookup func(name string) (string, bool)) string

// resolveCustomProps fixed-point-resolves the "--*" subset of raw against
// itself (custom properties may reference other custom properties),
// returning a map of fully-substituted custom-property values. Uses a
// visited-set to break cycles: a cyclic or otherwise unresolvable
// reference resolves to "" for that reference (approximates the spec's
// "guaranteed-invalid value" without implementing full invalid-at-
// computed-value-time fallback-to-inherited/initial semantics).
func resolveCustomProps(raw map[string]string) map[string]string
```

### `css.go` changes

In `commitDecl` and `parseDeclarationsWithImportance`, skip
`strings.ToLower(propBuf.String())` when the trimmed property name starts
with `--`; keep it for every other property. Value casing is already
preserved today (no change needed there).

### `cascade.go` changes

**`Resolve(n)`:**
1. `result := c.Direct(n)` — unchanged first line.
2. Existing ancestor-inheritance loop: broaden the copy condition from
   `inheritableProps[prop]` to `inheritableProps[prop] || isCustomProp(prop)`.
   This is the entire "custom properties inherit" implementation — it
   reuses the loop that's already walking ancestors for every other
   inheritable property, so there's no new traversal.
3. After that loop, `result` holds n's own + inherited custom properties
   (still raw/unsubstituted) plus all its cascaded normal properties (also
   still possibly containing raw `var()` text). Run:
   ```go
   resolvedVars := resolveCustomProps(result) // only touches "--*" keys
   for prop, val := range result {
       result[prop] = substituteVarTokens(val, func(name string) (string, bool) {
           v, ok := resolvedVars[name]
           return v, ok
       })
   }
   ```
   Substitution is applied to *every* property, inherited or not — only the
   custom-property *values* need inheritance; var() usage on a
   non-inherited property (e.g. `margin-top: var(--gap)`, set directly on
   `n`) must still resolve, since `--gap` is already in `result` (own or
   inherited) by this point.

**`PseudoElement(n, which)`:** after computing `decls` as today (unchanged),
call `Resolve(n)` and filter its result to `--*` keys — these are already
fully resolved (no leftover `var()`), so this is a single substitution
pass with no fixed-point/cycle logic needed:
```go
env := customPropSubset(c.Resolve(n))
for prop, val := range decls {
    decls[prop] = substituteVarTokens(val, func(name string) (string, bool) {
        v, ok := env[name]
        return v, ok
    })
}
```
This makes `content: var(--icon, "★ ")` resolve *before* render's own
`attr()`/`counter()` content-tokenizer ever sees the value — no changes
needed in `internal/render`'s content-parsing code, since by the time it
runs, `content`'s value is just an ordinary literal string.

**`Direct(n)`:** stays as today, plus one same-element-only substitution
pass at the end (no ancestor walk — this is the Option A scope cut):
```go
own := customPropSubset(result) // just this node's own "--*" decls
resolved := resolveCustomProps(own)
for prop, val := range result {
    result[prop] = substituteVarTokens(val, func(name string) (string, bool) {
        v, ok := resolved[name]
        return v, ok
    })
}
```

### `CSS.md` changes

New "Custom Properties (Variables)" section covering:
- Declaration syntax (`--name: value;`), case-sensitivity.
- `var(--name)` / `var(--name, fallback)` usage, fallback syntax (only the
  first comma is syntactic).
- Inheritance behavior (matches normal CSS: cascades and inherits, nearest
  declaration wins).
- The shorthand fan-out limitation (`margin: var(--sides)` does not expand
  into 4 independent sides) with a worked example, cross-referenced next to
  the existing `border: <width> <style>` limitation for consistency.
- The `Direct()`-only-sees-same-element-vars gap: `counter-reset`/
  `counter-increment` don't see *inherited* custom properties, only ones
  declared on that same element.
- Cycle behavior: a cyclic or undefined reference (with no fallback)
  resolves to empty string, not an error.

## Testing plan (`cssengine_internal_test.go`)

- Case-sensitivity: `--Foo` and `--foo` are distinct properties, and
  neither is lowercased by the parser.
- Basic substitution: `:root { --x: red; } p { color: var(--x); }`.
- Inheritance: descendant with no direct `--x` picks up an ancestor's
  value; a more specific descendant override wins over further-out
  ancestors (nearest-declaration-wins).
- Fallback: `var(--undefined, blue)` uses the fallback; `var(--defined,
  blue)` ignores it.
- Nested `var()` inside a fallback: `var(--a, var(--b, green))`.
- Chained custom properties: `--a: var(--b); --b: red;` resolves `--a` to
  `red` through the chain.
- Cycle detection terminates and resolves to empty string, not a hang:
  `--a: var(--b); --b: var(--a);`.
- `!important` on a `--x` declaration participates in the cascade like any
  other property (reuse the existing important-tier test pattern already in
  the file).
- `::before { content: var(--icon, "»") }` resolves before content-token
  parsing sees it.
- Same-element-only gap for `Direct()`: a test that explicitly asserts
  `counter-reset: var(--n)` resolves when `--n` is set on the *same*
  element, and explicitly does *not* resolve when `--n` is only set on an
  ancestor — pin the documented limitation so a future change to `Direct()`
  is a deliberate decision, not an accidental regression either way.
- Shorthand fan-out gap: assert `margin: var(--sides)` with
  `--sides: 1 2 3 4` produces the documented (not the "ideal") result, for
  the same reason.

## Non-goals for this pass

- `calc()` — separate, larger feature; not required for var() to be useful
  on its own (most theming use cases are direct substitution: colors,
  widths, single spacing values).
- `@property` registration/typing — custom properties are treated as
  untyped string substitution throughout, consistent with how this engine
  already treats every other property value (no type-checking at parse
  time).
- Full spec-accurate "invalid at computed-value time" fallback (falling
  back to the inherited value for inherited properties, or the property's
  initial value for non-inherited ones, when a var() reference is
  unresolvable and has no fallback). This implementation resolves an
  unresolvable reference to empty string instead, which is simpler and
  matches how this codebase already treats other malformed CSS values
  (silently ignored/no-effect) per `CSS.md`'s stated philosophy.
