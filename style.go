package htmlterm

import (
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
)

// inlineStyle is the accumulated text style passed down through inline rendering.
type inlineStyle struct {
	fg        color.Color // nil = unset
	bg        color.Color // nil = unset
	opacity   float64     // 0 = unset, treat as 1.0
	bold      bool
	italic    bool
	underline bool
	strike    bool
}

func (s inlineStyle) has() bool {
	return s.fg != nil || s.bg != nil || s.bold || s.italic || s.underline || s.strike
}

func (s inlineStyle) render(text string, p colorprofile.Profile) string {
	if !s.has() {
		return text
	}
	opacity := s.opacity
	if opacity == 0 {
		opacity = 1.0
	}
	var st ansi.Style
	if s.fg != nil {
		c := s.fg
		if opacity < 1 {
			c = applyOpacity(c, opacity)
		}
		if cc := p.Convert(c); cc != nil {
			st = st.ForegroundColor(cc)
		}
	}
	if s.bg != nil {
		c := s.bg
		if opacity < 1 {
			c = applyOpacity(c, opacity)
		}
		if cc := p.Convert(c); cc != nil {
			st = st.BackgroundColor(cc)
		}
	}
	if len(st) > 0 {
		text = st.Styled(text)
	}
	if s.bold {
		text = "\x1b[1m" + text + "\x1b[22m"
	}
	if s.italic {
		text = "\x1b[3m" + text + "\x1b[23m"
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
			if c := parseCSSColor(val); c != nil {
				s.fg = c
			}
		case "background-color":
			if c := parseCSSColor(val); c != nil {
				s.bg = c
			}
		case "opacity":
			if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
				s.opacity = max(0.0, min(1.0, f))
			}
		case "font-weight":
			switch val {
			case "bold":
				s.bold = true
			case "normal":
				s.bold = false
			}
		case "font-style":
			switch val {
			case "italic":
				s.italic = true
			case "normal":
				s.italic = false
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
