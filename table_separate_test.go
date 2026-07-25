package htmlterm_test

import "testing"

// TestTableBorderCollapseSeparate covers `border-collapse: separate` — the
// opt-in per-cell-bordered rendering path (internal/render/table_separate.go)
// added alongside the legacy shared-frame table model. See docs/TABLES.md.
func TestTableBorderCollapseSeparate(t *testing.T) {
	runCases(t, []renderCase{
		{
			name: "basic: per-cell borders and border-spacing gap",
			css:  `table { border-collapse: separate; border-spacing: 1; } td, th { border: solid; padding-left: 1; padding-right: 1; }`,
			html: `<table><tr><th>Name</th><th>Qty</th></tr><tr><td>Apple</td><td>3</td></tr><tr><td>Banana</td><td>5</td></tr></table>`,
			want: "                    \n ┌────────┐ ┌─────┐ \n │ Name   │ │ Qty │ \n └────────┘ └─────┘ \n                    \n ┌────────┐ ┌─────┐ \n │ Apple  │ │ 3   │ \n └────────┘ └─────┘ \n                    \n ┌────────┐ ┌─────┐ \n │ Banana │ │ 5   │ \n └────────┘ └─────┘ \n                    \n",
		},
		{
			name: "border-spacing defaults to 0 when unset",
			css:  `table { border-collapse: separate; } td, th { border: solid; }`,
			html: `<table><tr><th>Name</th><th>Qty</th></tr><tr><td>Apple</td><td>3</td></tr></table>`,
			want: "┌─────┐┌───┐\n│Name ││Qty│\n└─────┘└───┘\n┌─────┐┌───┐\n│Apple││3  │\n└─────┘└───┘\n",
		},
		{
			name: "border-spacing: 0 explicitly touches adjacent cell borders",
			css:  `table { border-collapse: separate; border-spacing: 0; } td, th { border: solid; }`,
			html: `<table><tr><th>Name</th><th>Qty</th></tr><tr><td>Apple</td><td>3</td></tr></table>`,
			want: "┌─────┐┌───┐\n│Name ││Qty│\n└─────┘└───┘\n┌─────┐┌───┐\n│Apple││3  │\n└─────┘└───┘\n",
		},
		{
			name: "border-spacing two-value form is horizontal then vertical",
			css:  `table { border-collapse: separate; border-spacing: 2 1; } td, th { border: solid; }`,
			html: `<table><tr><th>Name</th><th>Qty</th></tr><tr><td>Apple</td><td>3</td></tr></table>`,
			want: "                  \n  ┌─────┐  ┌───┐  \n  │Name │  │Qty│  \n  └─────┘  └───┘  \n                  \n  ┌─────┐  ┌───┐  \n  │Apple│  │3  │  \n  └─────┘  └───┘  \n                  \n",
		},
		{
			name: "table's own border/padding/margin wrap the assembled grid",
			css:  `table { border-collapse: separate; border-spacing: 1; border: double; padding: 1; margin-left: 2; } td, th { border: solid; }`,
			html: `<table><tr><th>Name</th><th>Qty</th></tr><tr><td>Apple</td><td>3</td></tr></table>`,
			want: "  ╔═════════════════╗\n  ║                 ║\n  ║                 ║\n  ║  ┌─────┐ ┌───┐  ║\n  ║  │Name │ │Qty│  ║\n  ║  └─────┘ └───┘  ║\n  ║                 ║\n  ║  ┌─────┐ ┌───┐  ║\n  ║  │Apple│ │3  │  ║\n  ║  └─────┘ └───┘  ║\n  ║                 ║\n  ║                 ║\n  ╚═════════════════╝\n",
		},
		{
			name: "colspan cell spans columns and reclaims the interior gap",
			css:  `table { border-collapse: separate; border-spacing: 1; } td, th { border: solid; }`,
			html: `<table><tr><td colspan="2">wide</td></tr><tr><td>a</td><td>b</td></tr></table>`,
			want: "          \n ┌────┐   \n │wide│   \n └────┘   \n          \n ┌──┐ ┌─┐ \n │a │ │b│ \n └──┘ └─┘ \n          \n",
		},
		{
			name: "rowspan cell spans rows including the interior spacing gap",
			css:  `table { border-collapse: separate; border-spacing: 1; } td, th { border: solid; }`,
			html: `<table><tr><td rowspan="2">tall</td><td>a</td></tr><tr><td>b</td></tr></table>`,
			want: "            \n ┌────┐ ┌─┐ \n │tall│ │a│ \n │    │ └─┘ \n │    │     \n │    │ ┌─┐ \n │    │ │b│ \n └────┘ └─┘ \n            \n",
		},
		{
			name: "cells with no border of their own stay unbordered, row height still matches",
			css:  `table { border-collapse: separate; border-spacing: 1; }`,
			html: `<table><tr><td style="border:solid">bordered</td><td>plain</td></tr></table>`,
			want: "                  \n ┌────────┐ plain \n │bordered│       \n └────────┘       \n                  \n",
		},
		{
			name: "vertical-align/text-align still apply per cell",
			css:  `table { border-collapse: separate; border-spacing: 1; } td, th { border: solid; }`,
			html: `<table><tr><td style="vertical-align:middle">mid</td><td style="vertical-align:bottom">bot</td><td style="text-align:right">tall<br>cell<br>here</td></tr></table>`,
			want: "                    \n ┌───┐ ┌───┐ ┌────┐ \n │   │ │   │ │tall│ \n │mid│ │   │ │cell│ \n │   │ │bot│ │here│ \n └───┘ └───┘ └────┘ \n                    \n",
		},
		{
			name:  "width:100% expands columns proportionally",
			html:  `<table style="width:100%; border-collapse:separate; border-spacing:1"><tr><td style="border:solid">a</td><td style="border:solid">b</td></tr></table>`,
			width: 30,
			want:  "                              \n ┌───────────┐ ┌────────────┐ \n │a          │ │b           │ \n └───────────┘ └────────────┘ \n                              \n",
		},
	})
}
