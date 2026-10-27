package core

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lts"
	"github.com/Kuniwak/puml-parallel/csdf/types"
)

func TestExpandRangesOverTheDeclaredTypes(t *testing.T) {
	// Setup: n and the field v are named apart from their type, Bit.
	d := typed(oneState(t, "c", []string{"n"}, "$n' = 0", loop(t, "c", ev("put", recv("v")), "", "$n' = $v")),
		map[string]types.Type{"n": types.Named("Bit")}, map[string][]types.Type{"put": {types.Named("Bit")}})
	env := Env{Types: map[string][]Value{"Bit": {0, 1}}, Atoms: bits().Atoms}

	// Execute
	got, err := Expand(d, env)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := handLTS(tr{0, lts.Tau, 1}, tr{1, "put.0", 1}, tr{1, "put.1", 2}, tr{2, "put.0", 1}, tr{2, "put.1", 2})
	if !lts.StableFailuresEqual(got, want) {
		t.Errorf("not stable-failures equivalent:\ngot  %#v\nwant %#v", got.Trans, want.Trans)
	}
}

func TestExpandRefusesToEnumerateAny(t *testing.T) {
	tests := map[string]*Diagram{
		"a variable of any the post leaves free": typed(oneState(t, "c", []string{"n"}, "", loop(t, "c", ev("go"), "", "")),
			map[string]types.Type{"n": types.AnyType}, nil),
		"a field of any received": typed(oneState(t, "c", nil, "", loop(t, "c", ev("put", recv("v")), "", "")),
			nil, map[string][]types.Type{"put": {types.AnyType}}),
	}
	for name, d := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := Expand(d, bits())

			// Assert
			if err == nil || !strings.Contains(err.Error(), "the type any has no values to range over") {
				t.Errorf("Expand: got %v, want a refusal to enumerate any", err)
			}
		})
	}
}
