package main

// Example is one hand-picked, hand-written showcase entry. These are not
// test cases: a test's job is to isolate one behavior as tightly as
// possible, while an example's job is to look like something a real
// document would contain. Pulling this list from internal/render's test
// tables was considered and rejected for exactly that reason — most of
// those are single-property permutations (padding-left:1, padding-left:2,
// ...), illustrative one at a time but repetitive as a gallery.
type Example struct {
	Slug        string // filename stem: docs/examples/<Slug>.md and .svg
	Title       string
	Description string
	CSS         string // additional stylesheet; "" if the example only needs inline style=
	HTML        string
	Width       int // terminal columns; 0 defaults to 40
}

var examples = []Example{
	{
		Slug:        "inline-text-styling",
		Title:       "Inline text styling",
		Description: "color, background-color, font-weight, font-style, and text-decoration on inline elements.",
		HTML: `<p>` +
			`<span style="color:#ff5f5f">colored</span>, ` +
			`<b>bold</b>, ` +
			`<i>italic</i>, ` +
			`<u>underlined</u>, ` +
			`<s>struck through</s>, and ` +
			`<span style="color:#1e1e1e;background-color:#ffd700">highlighted</span>.` +
			`</p>`,
		Width: 44,
	},
	{
		Slug:        "table-with-header-colors",
		Title:       "A table with a colored header row",
		Description: "border-collapse:collapse plus per-cell border and padding, and background-color/color on the header cells, the pattern a status table or changelog commonly wants.",
		CSS: `
table { border-collapse: collapse; }
th, td { border: 1px solid #888888; padding-left: 1; padding-right: 1; }
th { color: #ffffff; background-color: #005f87; }
`,
		HTML: `<table>
<tr><th>Check</th><th>Status</th></tr>
<tr><td>build</td><td style="color:#00d700;font-weight:bold">passing</td></tr>
<tr><td>lint</td><td style="color:#ff0000;font-weight:bold">failing</td></tr>
<tr><td>docs</td><td style="font-style:italic">pending</td></tr>
</table>`,
		Width: 26,
	},
	{
		Slug:        "table-border-styles",
		Title:       "Named border-style presets",
		Description: "border-style is a terminal-native extension (CSS.md), not a real CSS keyword set: solid, rounded, heavy, and double each pick a different box-drawing character set.",
		HTML: `<table style="border-style:solid; border-spacing:0"><tr><td>solid</td></tr></table>
<table style="border-style:rounded; border-spacing:0"><tr><td>rounded</td></tr></table>
<table style="border-style:heavy; border-spacing:0"><tr><td>heavy</td></tr></table>
<table style="border-style:double; border-spacing:0"><tr><td>double</td></tr></table>`,
		Width: 16,
	},
	{
		Slug:        "ordered-and-unordered-lists",
		Title:       "Lists",
		Description: "list-style-type on an ordered and an unordered list, including a custom start value.",
		HTML: `<ol start="3">
<li>third</li>
<li>fourth</li>
</ol>
<ul style="list-style-type:square">
<li>square marker</li>
<li>another item</li>
</ul>`,
		Width: 28,
	},
	{
		Slug:        "flex-row-with-gap",
		Title:       "A flex row with gap and colored items",
		Description: "display:flex, gap, and per-item background-color, the shape of a tag list or status strip.",
		HTML: `<div style="display:flex; gap:1">
<span style="background-color:#005f87;color:#ffffff;padding-left:1;padding-right:1">core</span>
<span style="background-color:#5f8700;color:#ffffff;padding-left:1;padding-right:1">stable</span>
<span style="background-color:#af5f00;color:#ffffff;padding-left:1;padding-right:1">v2</span>
</div>`,
		Width: 22,
	},
	{
		Slug:        "hyperlink-and-overflow",
		Title:       "OSC 8 hyperlinks and text-overflow",
		Description: "a link renders as a real OSC 8 terminal hyperlink, clickable in a supporting terminal, and text-overflow:ellipsis truncates a fixed-width cell instead of wrapping it.",
		HTML: `<p>See <a href="https://example.com/ci/run/482">the build log</a> for details.</p>
<table style="border-spacing:0"><tr><td style="white-space:nowrap; text-overflow:ellipsis; width:16">A very long status message</td></tr></table>`,
		Width: 40,
	},
}
