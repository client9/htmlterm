package render

import (
	"regexp"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

var popupAnsiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripPopupANSI(s string) string { return popupAnsiRe.ReplaceAllString(s, "") }

func TestSelectPopupComposition(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + `><option value="a">Apple</option><option value="b" selected>Banana</option><option value="c">Cherry</option></select>`
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	want := "[ Banana ▾]\n  Apple             \n▸ Banana            \n  Cherry            "
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}

	// Every rendered line must carry the reverse-video SGR wrapper for the
	// popup rows (not the closed control's own first line) — this is the
	// only per-cell visual distinction the popup gets, per RENDERING.md.
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4:\n%q", len(lines), result.Output)
	}
	for i, line := range lines[1:] {
		if !strings.Contains(line, "\x1b[7m") || !strings.Contains(line, "\x1b[27m") {
			t.Errorf("popup line %d missing reverse-video wrapper: %q", i+1, line)
		}
	}
}

func TestSelectPopupOptionsGetSyntheticPositions(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + `><option>Apple</option><option>Banana</option></select>`
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	var selectNode, appleNode *html.Node
	for n := range result.Positions {
		switch n.Data {
		case "select":
			selectNode = n
		case "option":
			if selectOptionLabel(n) == "Apple" {
				appleNode = n
			}
		}
	}
	if selectNode == nil {
		t.Fatalf("no Rect recorded for <select>")
	}
	selRect := result.Positions[selectNode]
	appleRect, ok := result.Positions[appleNode]
	if !ok {
		t.Fatalf("no Rect recorded for the first <option>")
	}
	if appleRect.Row != selRect.Row+selRect.Height {
		t.Errorf("first option Row = %d, want %d (directly below the select)", appleRect.Row, selRect.Row+selRect.Height)
	}
	if appleRect.Col != selRect.Col {
		t.Errorf("first option Col = %d, want %d (aligned with the select)", appleRect.Col, selRect.Col)
	}
}

func TestSelectPopupClosedRendersNoExtraLines(t *testing.T) {
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(`<select><option>Apple</option><option selected>Banana</option></select>`)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	got := stripPopupANSI(result.Output)
	if got != "[ Banana ▾]" {
		t.Errorf("got %q, want %q", got, "[ Banana ▾]")
	}
}

func TestSelectPopupGrowsDocumentWhenNoRoomBelow(t *testing.T) {
	src := `<select ` + defaultSelectOpenAttr + `><option>Apple</option><option>Banana</option><option>Cherry</option></select>`
	e, err := New(Options{Width: 20})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	result, err := e.RenderHTML(src)
	if err != nil {
		t.Fatalf("RenderHTML: %v", err)
	}
	lines := strings.Split(result.Output, "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want 4 (1 closed control + 3 options):\n%q", len(lines), result.Output)
	}
}
