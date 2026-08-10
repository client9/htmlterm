package render

// DefaultStylesheet is the built-in default stylesheet, of lowest priority,
// with Options.CSS and Options.Stylesheets layered above it. Re-exported by
// the root package as htmlterm.DefaultStylesheet, so callers can inspect it
// or diff their own overrides against it. Parsed once per Engine by New.
//
// This is the user-agent sheet, the terminal analogue of a browser's own
// default styles. It carries only what a rule can express: display types,
// font weight and style, margins and padding, borders, generated content.
// Anything needing engine behavior instead of a declaration is not here and
// cannot be moved here by adding a rule.
//
// Two categories in particular are handled in Go, not CSS:
//
//   - Head-level elements never reach the CSS cascade at all. render.go's
//     isSkippedContentElement drops <style>, <script>, <meta>, <link>, and
//     <template> wherever they appear, and its root dispatch skips <head>
//     outright, which covers <title>. A `display: none` rule for any of them
//     would be dead weight, so there isn't one.
//   - Form controls and widgets get their structure from their renderers
//     (formcontrol.go, select_popup.go, table*.go). The rules below set only
//     the parts a stylesheet owns, such as a control's outer display type or
//     a button's bracket affixes.
const DefaultStylesheet = `
/* border-box everywhere, the same reset Bootstrap's Reboot and
   sindresorhus/modern-normalize both ship, moved down into this engine's own
   UA layer. box-sizing's real spec initial value is content-box; this rule
   just means nothing sees it without asking, matching how those resets
   behave in a browser. Override on any selector to opt back into content-box
   for that element. See docs/proposals/BOX_SIZING.md. */
*, ::before, ::after   { box-sizing: border-box; }
table                   { display: table; }
[hidden], [aria-hidden=true] { display: none; }
p, blockquote, pre, h1, h2, h3, h4, h5, h6, div, section, article, header, footer, main, nav, aside, hgroup, search { display: block; }
dl, dt, dd, figure, figcaption  { display: block; }
address, details, summary, caption, noscript { display: block; }
address  { font-style: italic; }
summary  { font-weight: bold; }
/* Disclosure marker and collapse. summary::before is the marker glyph, an
   ordinary generated-content rule like button's own bracket affixes below,
   overridden to the open glyph only for the summary that's a direct child
   of an [open] details. The third rule is the actual collapse: a closed
   details with a summary to click on hides every other direct child, using
   the same [hidden]-driven display:none path any other element already
   gets. A details with no summary child is left alone, fully expanded,
   since there would be no way to reopen it. See COMPATIBILITY.md. */
summary::before                                     { content: "▶ "; }
details[open] > summary::before                     { content: "▼ "; }
details:has(> summary):not([open]) > :not(summary)  { display: none; }
/* dialog rides the same attribute-gated display:none path details does, one
   rule for the box and one for the closed state. The border and padding are
   this engine's translation of a browser UA sheet's own
   "border: solid; padding: 1em", using the cell counts textarea already
   settled on rather than inventing a second convention. A modal dialog picks
   up position/z-index/centering from the :modal rule further down; an open
   non-modal one is an ordinary in-flow block. See docs/DIALOG.md. */
dialog                  { display: block; border-style: solid; padding-left: 1; padding-right: 1; }
dialog:not([open])      { display: none; }
/* The top layer, expressed entirely in properties this engine already has.
   position:fixed hands the dialog to outofflow.go, which takes it out of
   normal flow and anchors it to the viewport; the maximum z-index puts it
   above every other positioned element; margin:auto centers it on both axes
   (see resolveAbsoluteAxis). Nothing composites a modal specially, unlike
   <select>'s popup. :modal matches via the reserved marker attribute
   ShowModal sets; see cssengine.Cascade's ModalAttr. */
dialog:modal            { position: fixed; z-index: 2147483647; margin: auto; }
caption  { text-align: center; }
p                       { margin-bottom: 1; }
h1, h2, h3, h4, h5, h6 { font-weight: bold; }
th                      { font-weight: bold; }
dt                      { font-weight: bold; }
strong, b               { font-weight: bold; }
em, i, dfn              { font-style: italic; }
samp, var, cite, figcaption { font-style: italic; }
a                       { text-decoration: underline; }
/*a > table               { text-decoration: none; } */
u, ins                  { text-decoration: underline; }
pre                     { white-space: pre; }
ul, ol, menu            { padding-left: 4; }
dd                      { padding-left: 4; }
dl                      { margin-bottom: 1; }
blockquote              { border-left: "│"; border-left-color: #555555; padding-left: 1; padding-right: 2; }
s, del                  { text-decoration: line-through; }
kbd                     { font-weight: bold; }
mark                    { background-color: #cc9900; color: #000000; }
small                   { color: #888888; }
sup                     { text-transform: superscript; }
sub                     { text-transform: subscript; }
q::before               { content: open-quote; }
q::after                { content: close-quote; }
img::before             { content: attr(alt); }
abbr[title]::after      { content: " (" attr(title) ")"; }
/* The three explicit "none"s keep hr a single rule under an author's own
   border-style preset. Without them, a preset's unclaimed bottom/left/right
   edges would fill in around this rule's own border-top, turning what
   should stay one line into a lopsided two-line box. Style hr's line with
   border-top (a literal glyph, or the per-edge preset form, e.g.
   border-top: double), not border-style; see CSS.md's hr entry. */
hr                      { display: block; border-top: "─"; border-right: none; border-bottom: none; border-left: none; }
form                    { display: block; }
fieldset                { display: block; border-style: solid; padding: 1; margin-bottom: 1; }
legend                  { display: block; font-weight: bold; }
input, button, select   { display: inline-block; }
progress, meter         { display: inline-block; width: 20; height: 1; }
textarea                { display: block; border-style: solid; padding-left: 1; padding-right: 1; }
button::before          { content: "[ "; }
button::after           { content: " ]"; }
::scrollbar             { width: 1ch; }
::scrollbar-x           { height: 1; }
`
