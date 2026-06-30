package htmlterm

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// inlineStyle is the accumulated text style passed down through inline rendering.
type inlineStyle struct {
	lg        lipgloss.Style
	hasLG     bool
	underline bool
	strike    bool
}

func (s inlineStyle) has() bool { return s.hasLG || s.underline || s.strike }

func (s inlineStyle) render(text string) string {
	if !s.has() {
		return text
	}
	if s.hasLG {
		text = s.lg.Render(text)
	}
	if s.underline {
		text = "\x1b[4m" + text + "\x1b[24m"
	}
	if s.strike {
		text = "\x1b[9m" + text + "\x1b[29m"
	}
	return text
}

// extractInlineStyle builds an inlineStyle from a resolved CSS declaration map.
func extractInlineStyle(decls map[string]string) inlineStyle {
	return mergeInlineStyle(inlineStyle{}, decls)
}

// mergeInlineStyle overlays the visual text properties from decls onto base.
func mergeInlineStyle(base inlineStyle, decls map[string]string) inlineStyle {
	s := base
	for prop, val := range decls {
		switch prop {
		case "color":
			s.lg = s.lg.Foreground(lipgloss.Color(val))
			s.hasLG = true
		case "background-color":
			s.lg = s.lg.Background(lipgloss.Color(val))
			s.hasLG = true
		case "font-weight":
			switch val {
			case "bold":
				s.lg = s.lg.Bold(true)
				s.hasLG = true
			case "normal":
				s.lg = s.lg.Bold(false)
			}
		case "font-style":
			switch val {
			case "italic":
				s.lg = s.lg.Italic(true)
				s.hasLG = true
			case "normal":
				s.lg = s.lg.Italic(false)
			}
		case "text-decoration":
			switch val {
			case "none", "normal":
				s.underline = false
				s.strike = false
			default:
				if strings.Contains(val, "underline") {
					s.underline = true
				}
				if strings.Contains(val, "line-through") {
					s.strike = true
				}
			}
		}
	}
	return s
}
