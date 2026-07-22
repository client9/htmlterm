package render

import (
	"reflect"
	"testing"
)

func TestParseSymbolsArgs(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   []string
		wantOk bool
	}{
		{name: "three quoted strings", raw: `symbols("🟥" "🟨" "🟦")`, want: []string{"🟥", "🟨", "🟦"}, wantOk: true},
		{name: "single quoted string", raw: `symbols("*")`, want: []string{"*"}, wantOk: true},
		{name: "mixed quote styles", raw: `symbols('a' "b")`, want: []string{"a", "b"}, wantOk: true},
		{name: "leading/trailing whitespace tolerated", raw: `  symbols( "a"  "b" )  `, want: []string{"a", "b"}, wantOk: true},
		{name: "case-insensitive function name", raw: `SYMBOLS("a" "b")`, want: []string{"a", "b"}, wantOk: true},
		{name: "not a symbols() token", raw: `disc`, want: nil, wantOk: false},
		{name: "other function token", raw: `counter(x)`, want: nil, wantOk: false},
		{name: "empty argument list", raw: `symbols()`, want: nil, wantOk: false},
		{name: "missing closing paren", raw: `symbols("a"`, want: nil, wantOk: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseSymbolsArgs(tt.raw)
			if ok != tt.wantOk {
				t.Fatalf("parseSymbolsArgs(%q) ok = %v, want %v", tt.raw, ok, tt.wantOk)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseSymbolsArgs(%q) = %v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}
