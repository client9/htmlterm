package cssengine

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// TestDisabledSelectorCascadesFromFieldset checks that :disabled matches a
// control nested in a <fieldset disabled>, mirroring HTML's fieldset-
// disabling algorithm the same way :disabled already cascades an <option>'s
// state from its containing <optgroup> (see
// select_popup_internal_test.go's TestSelectPopupOptionDisabledSelectorCascadesFromOptgroup
// for that case). IsFieldsetDisabled is exported so document.go's
// DispatchClick, isFormFocusable, and isEditable share this exact same
// predicate rather than keeping an independent copy.
func TestDisabledSelectorCascadesFromFieldset(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(
		`<fieldset disabled><input id="direct"><legend><input id="in-legend"></legend></fieldset>` +
			`<input id="outside">`,
	))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	group := ParseSelectorGroup(":disabled")

	direct := findElementByID(doc, "direct")
	if direct == nil {
		t.Fatal(`input#direct not found`)
	}
	if !group.Match(direct, Markers{}) {
		t.Error(":disabled did not match a control inside a disabled <fieldset>")
	}

	inLegend := findElementByID(doc, "in-legend")
	if inLegend == nil {
		t.Fatal(`input#in-legend not found`)
	}
	if group.Match(inLegend, Markers{}) {
		t.Error(":disabled matched a control inside the fieldset's first <legend>, want the legend exemption to apply")
	}

	outside := findElementByID(doc, "outside")
	if outside == nil {
		t.Fatal(`input#outside not found`)
	}
	if group.Match(outside, Markers{}) {
		t.Error(":disabled matched a control outside the disabled <fieldset> entirely")
	}
}

// TestDisabledSelectorFieldsetLegendExemptionIsPerFieldset checks that a
// nested fieldset's own legend only exempts that fieldset's own disabling:
// an outer disabled fieldset still applies, matching HTML's algorithm,
// which walks every ancestor fieldset independently rather than stopping at
// the first legend found.
func TestDisabledSelectorFieldsetLegendExemptionIsPerFieldset(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(
		`<fieldset disabled><fieldset disabled><legend><input id="in"></legend></fieldset></fieldset>`,
	))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	n := findElementByID(doc, "in")
	if n == nil {
		t.Fatal(`input#in not found`)
	}
	if !ParseSelectorGroup(":disabled").Match(n, Markers{}) {
		t.Error(":disabled did not match a control inside the inner fieldset's legend but still inside an outer disabled fieldset")
	}
}

// TestDisabledSelectorDoesNotMatchNonFormContentInFieldset checks that
// :disabled only ever matches a "listed" form-associated element (see
// isFieldsetDisablable): a disabled <fieldset> doesn't disable, and
// :disabled must not match, plain content nested inside it that was never
// disableable to begin with, matching real HTML's fieldset-disabling
// algorithm, which only ever reaches listed elements.
func TestDisabledSelectorDoesNotMatchNonFormContentInFieldset(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(
		`<fieldset disabled><p id="p">text</p><a id="a" href="#">link</a><div id="d">block</div></fieldset>`,
	))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	group := ParseSelectorGroup(":disabled")
	for _, id := range []string{"p", "a", "d"} {
		n := findElementByID(doc, id)
		if n == nil {
			t.Fatalf("element #%s not found", id)
		}
		if group.Match(n, Markers{}) {
			t.Errorf(":disabled matched #%s (a %s), which isn't a form-associated element and should be unaffected by the fieldset's disabled attribute", id, n.Data)
		}
	}
}
