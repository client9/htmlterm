package htmlterm

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// declsToStyle converts a resolved CSS declaration map to a lipgloss.Style.
func declsToStyle(decls map[string]string) lipgloss.Style {
	s := lipgloss.NewStyle()
	for prop, val := range decls {
		switch prop {
		case "color":
			s = s.Foreground(lipgloss.Color(val))
		case "background-color":
			s = s.Background(lipgloss.Color(val))
		case "font-weight":
			switch val {
			case "bold":
				s = s.Bold(true)
			case "normal":
				s = s.Bold(false)
			}
		case "font-style":
			switch val {
			case "italic":
				s = s.Italic(true)
			case "normal":
				s = s.Italic(false)
			}
		case "text-align":
			switch val {
			case "right":
				s = s.Align(lipgloss.Right)
			case "center":
				s = s.Align(lipgloss.Center)
			case "left":
				s = s.Align(lipgloss.Left)
			}
		}
	}
	return s
}

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
