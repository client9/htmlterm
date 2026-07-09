package htmlterm

import (
	"github.com/charmbracelet/colorprofile"
	"github.com/client9/htmlterm/internal/render"
)

// SizeAutomatic is the zero value for Options.Width/Options.Height: track the
// terminal's current size for that dimension. Plain Renderer.Render/Document
// have no terminal to query, so it's inert there (behaves like SizeNatural
// for Height; Width has no natural fallback and is simply left at 0, same as
// before this constant existed). Loop is what actually resolves it — see
// Loop.Run and Document.SetSize — querying the terminal once at startup and
// again on every SIGWINCH, keeping whichever of Width/Height is
// SizeAutomatic live for the life of the Loop. It is the zero value
// deliberately, matching the rest of Options: a caller who never mentions
// Width/Height and drives rendering through Loop gets automatic sizing
// without writing anything.
const SizeAutomatic = 0

// SizeNatural, valid for Options.Height only, means "don't constrain height
// at all" — the document renders at whatever line count its content
// produces, with no clipping or padding (today's behavior, before Height
// existed). There is no equivalent for Width: wrapping always needs a
// concrete column count, so a "natural width" isn't a meaningful concept
// here the way it is for height.
const SizeNatural = -1

// Options configures a Renderer.
type Options struct {
	CSS               string               // additional stylesheet layered above built-in UA defaults
	Width             int                  // terminal column count; affects wrapping, tables, percentage widths
	Height            int                  // content-box line count the whole document is clipped/padded to; zero value is SizeAutomatic (see there); SizeNatural (-1) leaves it unconstrained
	IgnoreDocumentCSS bool                 // if true, <style> elements and style= attributes in HTML are ignored
	Profile           colorprofile.Profile // color profile; zero value (NoTTY) auto-detects from environment
	NoOSC8Links       bool                 // if true, OSC 8 hyperlink sequences are not emitted for <a> elements
	MaxBlankLines     int                  // if > 0, collapses runs of blank lines to at most this many; <pre> content is not affected
	StripHiddenInline bool                 // if true, elements hidden via their own inline style= (display:none, visibility:hidden, opacity:0, zero height/max-height with overflow:hidden) are removed before rendering; independent of IgnoreDocumentCSS
}

// Renderer renders HTML+CSS to terminal strings.
//
// A Renderer can be reused for multiple Render calls, including concurrent
// calls. Per-document state is built fresh for each render.
type Renderer struct {
	engine *render.Engine
}

// New parses opts.CSS and returns a Renderer.
func New(opts Options) (*Renderer, error) {
	engine, err := render.New(renderOptions(opts))
	if err != nil {
		return nil, err
	}
	return &Renderer{engine: engine}, nil
}

// Render parses htmlStr and returns a styled terminal string.
func (r *Renderer) Render(htmlStr string) (string, error) {
	result, err := r.engine.RenderHTML(htmlStr)
	return result.Output, err
}

// ConsumeANSI returns the index just past the escape sequence starting at
// runes[i] (runes[i] must be '\x1b'), recognizing the CSI and OSC forms
// htmlterm's own renderer emits. Exported so a consumer decoding htmlterm's
// rendered ANSI output back into another representation (e.g. tui's
// cell-bridge, painting cells into a tcell.Screen) can tokenize with the
// exact same rules the renderer used to produce it, rather than maintaining
// a second, potentially drifting implementation.
func ConsumeANSI(runes []rune, i int) int {
	return render.ConsumeANSI(runes, i)
}

func renderOptions(opts Options) render.Options {
	return render.Options{
		CSS:               opts.CSS,
		Width:             opts.Width,
		Height:            opts.Height,
		IgnoreDocumentCSS: opts.IgnoreDocumentCSS,
		Profile:           opts.Profile,
		NoOSC8Links:       opts.NoOSC8Links,
		MaxBlankLines:     opts.MaxBlankLines,
		StripHiddenInline: opts.StripHiddenInline,
	}
}
