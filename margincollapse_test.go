package htmlterm_test

import "testing"

// TestMarginCollapse covers CSS margin-collapsing shapes specifically (as
// opposed to htmlterm_test.go's/layout_test.go's broader block-layout
// coverage): parent/first-child and parent/last-child collapse-through, what
// blocks it (border/padding/height/::before/::after content), multi-level
// chains, and that non-block-container consumers (table cells, list items,
// display:contents splices) never leak a stray blank line from a child's
// margin they don't collapse.
func TestMarginCollapse(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "parent+first-child margin-top collapses through an open top edge",
			html: `first line<div><h2 style="margin-top:1">Heading</h2></div>`,
			want: "first line\n\nHeading\n",
		},
		{
			name: "parent+last-child margin-bottom collapses through an open bottom edge",
			html: `<div><h2 style="margin-bottom:1">Heading</h2></div>after`,
			want: "Heading\n\nafter",
		},
		{
			name: "border-top blocks margin-top collapse, margin still applies inside",
			html: `<div style="border-style:solid"><p style="margin-top:1">a</p></div>`,
			want: "┌──────────────────────────────────────┐\n│                                      │\n│a                                     │\n│                                      │\n└──────────────────────────────────────┘\n",
		},
		{
			name: "border-bottom blocks margin-bottom collapse, margin still applies inside",
			html: `<div style="border-style:solid"><p style="margin-bottom:1">a</p></div>`,
			want: "┌──────────────────────────────────────┐\n│a                                     │\n│                                      │\n└──────────────────────────────────────┘\n",
		},
		{
			name: "padding-top blocks margin-top collapse, margin still applies inside",
			html: `<div style="padding-top:1"><p style="margin-top:1">a</p></div>after`,
			want: "                                        \n\na\n\nafter",
		},
		{
			name: "padding-bottom blocks margin-bottom collapse, margin still applies inside",
			html: `<div style="padding-bottom:1"><p style="margin-bottom:1">a</p></div>after`,
			want: "a\n\n                                        \nafter",
		},
		{
			name: "explicit height blocks margin-top collapse, margin still applies inside",
			html: `<div style="height:3"><p style="margin-top:1">a</p></div>after`,
			want: "\na\n\nafter",
		},
		{
			name: "multi-level chain collapses through consecutive open edges both directions",
			html: `first<div><div><h2 style="margin-top:2;margin-bottom:1">Nested</h2></div></div>after`,
			want: "first\n\n\nNested\n\nafter",
		},
		{
			name: "::before content on the parent blocks margin-top collapse",
			css:  `div::before { content: "X "; }`,
			html: `<div><h2 style="margin-top:1">no collapse</h2></div>`,
			want: "X \n\nno collapse\n",
		},
		{
			name: "table cell with a first block child does not leak a leading blank line",
			html: `<table><tr><td><h2 style="margin-top:1">first in cell</h2></td></tr></table>`,
			want: "┌─────────────┐\n│first in cell│\n└─────────────┘\n",
		},
		{
			name: "list item with a loose paragraph does not leak a leading blank line",
			html: `<ul><li><p style="margin-top:1">item</p></li></ul>`,
			want: "    • item\n",
		},
		{
			name: "display:contents splice does not leak a leading blank line at document start",
			html: `<div style="display:contents"><p>one</p><p>two</p></div>`,
			want: "one\n\ntwo\n",
		},
		{
			name: "adjacent siblings collapse via max, not sum",
			html: `<p style="margin-bottom:3">a</p><p style="margin-top:1">b</p>`,
			want: "a\n\n\n\nb\n\n",
		},
	})
}
