package htmlterm

import (
	"reflect"
	"strings"
	"testing"
)

func TestExpandShorthand(t *testing.T) {
	tests := []struct {
		name string
		prop string
		val  string
		want map[string]string
	}{
		{
			name: "margin one value",
			prop: "margin",
			val:  "2",
			want: map[string]string{
				"margin-top":    "2",
				"margin-right":  "2",
				"margin-bottom": "2",
				"margin-left":   "2",
			},
		},
		{
			name: "padding two values",
			prop: "padding",
			val:  "1 3",
			want: map[string]string{
				"padding-top":    "1",
				"padding-right":  "3",
				"padding-bottom": "1",
				"padding-left":   "3",
			},
		},
		{
			name: "margin three values",
			prop: "margin",
			val:  "1 2 3",
			want: map[string]string{
				"margin-top":    "1",
				"margin-right":  "2",
				"margin-bottom": "3",
				"margin-left":   "2",
			},
		},
		{
			name: "padding four values",
			prop: "padding",
			val:  "1 2 3 4",
			want: map[string]string{
				"padding-top":    "1",
				"padding-right":  "2",
				"padding-bottom": "3",
				"padding-left":   "4",
			},
		},
		{
			name: "invalid arity falls back",
			prop: "margin",
			val:  "1 2 3 4 5",
			want: map[string]string{"margin": "1 2 3 4 5"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandShorthand(tc.prop, tc.val); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expandShorthand(%q, %q) = %#v, want %#v", tc.prop, tc.val, got, tc.want)
			}
		})
	}
}

func TestMergeInlineStyleTextDecoration(t *testing.T) {
	base := mergeInlineStyle(inlineStyle{}, map[string]string{"text-decoration": "underline line-through"})
	if !base.underline || !base.strike {
		t.Fatalf("combined decoration not applied: %#v", base)
	}

	reset := mergeInlineStyle(base, map[string]string{"text-decoration": "none"})
	if reset.underline || reset.strike {
		t.Fatalf("text-decoration:none did not reset flags: %#v", reset)
	}
}

func TestSplitANSITokens(t *testing.T) {
	osc := "\x1b]8;;https://example.com\x07"
	oscEnd := "\x1b]8;;\x07"
	text := "pre " + osc + "link text" + oscEnd + " post\t\x1b[31mred\x1b[0m"
	want := []string{
		"pre",
		osc + "link",
		"text" + oscEnd,
		"post",
		"\x1b[31mred\x1b[0m",
	}
	if got := splitANSITokens(text); !reflect.DeepEqual(got, want) {
		t.Fatalf("splitANSITokens() = %#v, want %#v", got, want)
	}
}

func TestStripANSI(t *testing.T) {
	osc := "\x1b]8;;https://example.com\x1b\\"
	oscEnd := "\x1b]8;;\x1b\\"
	text := "a\x1b[31mred\x1b[0m" + osc + "link" + oscEnd + "b"
	if got := stripANSI(text); got != "aredlinkb" {
		t.Fatalf("stripANSI() = %q, want %q", got, "aredlinkb")
	}
}

func TestToRoman(t *testing.T) {
	if got := toRoman(49, false); got != "xlix" {
		t.Fatalf("toRoman lower = %q, want %q", got, "xlix")
	}
	if got := toRoman(944, true); got != "CMXLIV" {
		t.Fatalf("toRoman upper = %q, want %q", got, "CMXLIV")
	}
}

func TestListItemPrefixWidthRoman(t *testing.T) {
	if got := listItemPrefixWidth("lower-roman", true, 8); got != len("viii. ") {
		t.Fatalf("lower-roman width = %d, want %d", got, len("viii. "))
	}
	if got := listItemPrefix("lower-roman", true, 8, 0); got != "viii. " {
		t.Fatalf("lower-roman prefix = %q, want %q", got, "viii. ")
	}
}

func TestSizeColumnsRespectsMaxAndShrinkMin(t *testing.T) {
	expanded := sizeColumns([]colConstraints{
		{natural: 2},
		{natural: 3, maxWidth: 4},
		{fixed: 5},
	}, 15, true)
	if !reflect.DeepEqual(expanded, []int{6, 4, 5}) {
		t.Fatalf("sizeColumns expand = %#v, want %#v", expanded, []int{6, 4, 5})
	}

	shrunk := sizeColumns([]colConstraints{
		{natural: 8, minWidth: 6},
		{natural: 7},
		{fixed: 5},
	}, 15, false)
	if !reflect.DeepEqual(shrunk, []int{6, 4, 5}) {
		t.Fatalf("sizeColumns shrink = %#v, want %#v", shrunk, []int{6, 4, 5})
	}
}

func TestAlignAndEdgeHelpersPreserveTrailingNewline(t *testing.T) {
	aligned := alignLines("a\nbb\n", "center", 4)
	if aligned != " a  \n bb \n" {
		t.Fatalf("alignLines() = %q, want %q", aligned, " a  \n bb \n")
	}

	edged := applyLineEdges("x\ny\n", "[", "]")
	if edged != "[x]\n[y]\n" {
		t.Fatalf("applyLineEdges() = %q, want %q", edged, "[x]\n[y]\n")
	}
}

func TestParseCSSCommaAndShorthand(t *testing.T) {
	rules, err := parseCSS("p, div { margin: 1 2; padding: 3; }")
	if err != nil {
		t.Fatalf("parseCSS() error = %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("parseCSS() returned %d rules, want 2", len(rules))
	}
	for _, r := range rules {
		if !strings.Contains("p div", r.selector) {
			t.Fatalf("unexpected selector %q", r.selector)
		}
		if r.decls["margin-left"] != "2" || r.decls["padding-bottom"] != "3" {
			t.Fatalf("unexpected decls for %q: %#v", r.selector, r.decls)
		}
	}
}
