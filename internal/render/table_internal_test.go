package render

import (
	"testing"

	"github.com/client9/htmlterm/internal/cssengine"
)

// TestNamedTableStyleMatchesBorderStyleVocabulary keeps borderStylePresets
// (this package) and cssengine.BorderStyleNames (internal/cssengine/css.go)
// in exact sync. The two lists exist because of the package layering:
// cssengine needs the bare name set to classify the border and
// border-<edge> shorthands' <style> component, well before rendering is
// involved, while only render can own what each name actually draws. A name
// present in one but not the other is a bug either way: missing from
// cssengine.BorderStyleNames, that name's shorthand value would silently
// misclassify as a color instead of a style; missing from
// borderStylePresets, border-style: <name> would silently draw no border at
// all instead of the intended preset. This test turns either mistake into an
// immediate failure instead of a silent drift.
func TestNamedTableStyleMatchesBorderStyleVocabulary(t *testing.T) {
	for name := range cssengine.BorderStyleNames {
		if _, ok := namedTableStyle(name); !ok {
			t.Errorf("cssengine.BorderStyleNames has %q, but namedTableStyle doesn't recognize it; add a borderStylePresets entry for it in table.go", name)
		}
	}
	for name := range borderStylePresets {
		if !cssengine.BorderStyleNames[name] {
			t.Errorf("borderStylePresets has %q, but cssengine.BorderStyleNames doesn't; add it there too, or the border shorthand will misclassify it as a color", name)
		}
	}
}
