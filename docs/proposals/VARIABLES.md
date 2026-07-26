# CSS Custom Properties (`--foo` / `var()`) — Design & Implementation Plan

Status: **implemented**, in `internal/cssengine/customprops.go` plus the
`Direct`/`Resolve`/`PseudoElement` changes in `internal/cssengine/cascade.go`
described below. This document remains the design record and rationale —
see CSS.md's "Custom Properties (Variables)" section for the user-facing
reference. One refinement emerged during implementation that's folded into
the "Design" section below: `Direct()` cannot reuse `substituteVarTokens`
(the same "final" substitution `Resolve()`/`PseudoElement()` use) for its
own-element-only pass, because that function collapses an unresolved
reference to its fallback (or `""`) — destructive behavior that's correct
once the full ancestor-aware environment is known, but wrong at `Direct()`
time, when a reference to an ancestor-only custom property must survive
untouched for `Resolve()` to resolve later. `substituteKnownVarTokens` is
the non-destructive variant used there instead (see its doc comment in
`customprops.go`).

---

## Why this is worth doing

`internal/cssengine` already has most of the required machinery:

- `Cascade.Direct` / `Resolve` / `PseudoElement` are the *only* three entry
  points `internal/render` uses (`internal/render/cascade.go`), so almost
  all of var() support is contained inside `cssengine`. The one exception:
  `PseudoElement` needs its caller's already-resolved custom-prop
  environment passed in (see "Design" below, to avoid a redundant
  `Resolve(n)` call), which does mean a small, mechanical signature change
  threading through `internal/render/cascade.go`'s three wrappers and their
  call sites — not "zero changes in `internal/render`," but still nothing
  in `document` or `tui`.
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
3. **Timing matters for performance, but this codebase's existing baseline
   is already more expensive than it looks.** `Cascade.Cache` only memoizes
   `Direct` (`cascade.go`'s `Cache map[*html.Node]map[string]string` field,
   populated solely inside `Direct`). `Resolve(n)` itself is *not* memoized:
   it recurses via nested `c.Resolve(anc)` calls up the ancestor chain
   (`getParentResolved`), and every level of that recursion re-runs its own
   ~16-entry `inheritableProps` fill loop and `forcedAbsent` allocation from
   scratch. `Direct(anc)` is cheap thanks to the cache, but that surrounding
   inheritance bookkeeping is not cached at all, so a full render pass that
   calls `Resolve` on every node already costs at least O(N·D) (worse — up
   to O(N²) — on deep/linear trees), independent of variables.
   var()-substitution should still live at the `Resolve()`/`PseudoElement()`
   level rather than inside `Direct()` (doing it inside `Direct()` would add
   a *second*, unmemoized ancestor walk per node, on top of the one above —
   true O(depth²) new cost, not just riding along on existing cost). But
   doing it in `Resolve()` should be described honestly: it adds a fixed
   amount of substitution work at every level of an already non-cheap,
   already-repeated recursion, not "nothing extra." Two concrete
   consequences for the design below:
   - The `Resolve()` inheritance fill-in loop must be fixed correctly (see
     "Design" below) — a naive edit that only changes a loop *condition*
     without changing what the loop iterates over is easy to write, compiles
     fine, and silently does nothing.
   - `PseudoElement()`'s own var-environment lookup must not add a *second*
     full `Resolve(n)` on top of the one its caller already performs (every
     call site already computes `nDecls := r.resolveDecls(n)` right next to
     its `pseudoElemDecls(n, ...)` call — see `internal/render/inline.go`
     around lines 130/143) — see "Design" below.
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
2. Existing ancestor-inheritance loop (`cascade.go:167-175`) *cannot* just
   have its copy condition widened in place — it is:
   ```go
   for prop := range inheritableProps {
       if _, exists := result[prop]; !exists && !forcedAbsent[prop] {
           if v, ok := pr[prop]; ok {
               result[prop] = v
           }
       }
   }
   ```
   `prop` is only ever drawn from the fixed `inheritableProps` map's keys —
   a custom property name like `--brand` never occurs as a loop variable,
   so widening the *condition* inside the loop body (`inheritableProps[prop]
   || isCustomProp(prop)`) is a no-op: that body is simply never reached
   with `prop == "--brand"`. This would compile and look right while
   silently breaking the most common variables use case (a descendant that
   never redeclares `--brand` at all, e.g. `:root { --brand: blue; } p {
   color: var(--brand); }` with no `--brand` on `p`) — the earlier keyword
   loop already handles the *explicit* `--x: inherit` case correctly (it
   iterates `direct`'s own keys generically), so only the *implicit*
   no-declaration-at-all path is at risk here.

   The correct fix adds a **second loop that iterates the parent's own
   resolved keys**, not `inheritableProps`:
   ```go
   for prop := range inheritableProps { ... } // unchanged, as above
   for prop, v := range pr {
       if !isCustomProp(prop) {
           continue
       }
       if _, exists := result[prop]; !exists && !forcedAbsent[prop] {
           result[prop] = v
       }
   }
   ```
   This is still a small, contained diff and still reuses the
   already-computed `pr` (`getParentResolved()`) rather than triggering any
   new ancestor walk — just not by editing the existing loop's condition
   alone.
3. After both loops, `result` holds n's own + inherited custom properties
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

**`PseudoElement(n, which)`:** must **not** call `c.Resolve(n)` itself to
build its var-lookup environment — every real call site already computes
`nDecls := r.resolveDecls(n)` immediately before calling `pseudoElemDecls(n,
...)` (e.g. `internal/render/inline.go` around lines 130/143, similarly in
`block.go`/`list.go`). Since `Resolve` is not memoized (only `Direct` is,
via `Cache`), an internal `c.Resolve(n)` call here would silently double
the cost of resolving `n` for every element carrying a `::before`/
`::after`/`::marker`/`::scrollbar*` — a second full ancestor-inheritance
recursion purely to re-derive custom-prop values the caller already has.

Instead, `PseudoElement` takes the caller's already-resolved environment
as a parameter:
```go
func (c Cascade) PseudoElement(n *html.Node, which string, env map[string]string) map[string]string {
    // ...compute decls as today (unchanged)...
    for prop, val := range decls {
        decls[prop] = substituteVarTokens(val, func(name string) (string, bool) {
            v, ok := env[name]
            return v, ok
        })
    }
    return decls
}
```
Callers pass `customPropSubset(nDecls)` (the `Resolve(n)` result they
already computed for their own purposes) rather than the whole map, so
`PseudoElement` never needs to know how `env` was derived — this is a
substitution pass with no fixed-point/cycle logic of its own, since `env`'s
values are already fully resolved by the time `Resolve(n)` produced them.
This does touch `internal/render/cascade.go`'s three thin wrappers
(`pseudoElemDecls` and its call sites), a small, contained exception to
this proposal's "zero changes needed outside `cssengine`" framing in
"Why this is worth doing" above — worth calling out explicitly rather than
silently dropping that guarantee.

This makes `content: var(--icon, "★ ")` resolve *before* render's own
`attr()`/`counter()` content-tokenizer ever sees the value — no changes
needed in `internal/render`'s content-*parsing* code itself, since by the
time it runs, `content`'s value is just an ordinary literal string.

**`Direct(n)`:** stays as today, plus one same-element-only substitution
pass at the end (no ancestor walk — this is the Option A scope cut):
```go
if own := customPropSubset(result); len(own) > 0 {
    resolvedOwn := resolveCustomProps(own)
    for prop, val := range result {
        result[prop] = substituteKnownVarTokens(val, resolvedOwn)
    }
}
```
Note this uses `substituteKnownVarTokens`, **not** `substituteVarTokens` —
this is the one place the two must not be interchanged. `substituteVarTokens`
(used by `Resolve`/`PseudoElement`, above) collapses an unresolved reference
to its fallback or `""`, which is correct once the full ancestor-aware
environment is known, but `Direct()` has no ancestor context at all: it
cannot tell "genuinely undefined anywhere" apart from "defined further up
the tree." Since `Direct(n)`'s result seeds `Resolve(n)` (`direct :=
c.Direct(n)`), using the destructive `substituteVarTokens` here would
permanently erase any `var()` referencing an ancestor-only custom property
— e.g. `margin-top: var(--gap)` where `--gap` is only declared on an
ancestor — before `Resolve()` ever got a chance to look further up the tree,
silently breaking inheritance for `var()` on every ordinary property, not
just the counter-reset/counter-increment case this cut is meant for.
`substituteKnownVarTokens` instead leaves any reference not found in `own`
completely untouched (name, fallback, and all), so `Resolve()`'s later,
fully ancestor-aware pass still sees the literal `var(...)` text and can
resolve it correctly.

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
