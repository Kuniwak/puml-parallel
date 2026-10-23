package core

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// promoted is Promote(p), failing t on an error.
func promoted(t *testing.T, p Promotion) *Diagram {
	t.Helper()
	d, _, err := Promote(p)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestEachOperationCarriesTheLocalStatesOfTheFamilies(t *testing.T) {
	keys := FamilyStates{Map: "keys", Init: "none", States: []string{"none", "held"}, Vars: map[string][]string{}}
	tests := map[string]struct {
		do   func(t *testing.T) (*Diagram, error)
		want []FamilyStates
	}{
		"Promote writes the local states of each family": {
			do: func(t *testing.T) (*Diagram, error) {
				d, _, err := Promote(Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{
					{Map: "locations", ID: "loc", Local: location(t)}, {Map: "doors", ID: "d", Local: door()},
				}})
				return d, err
			},
			want: []FamilyStates{
				{Map: "locations", Init: "none", States: []string{"none", "open"}, Vars: map[string][]string{"open": {"stock"}}},
				{Map: "doors", Init: "none", States: []string{"none", "shut", "wide"}, Vars: map[string][]string{}},
			},
		},
		"Restrict keeps those of the kept families only": {
			do: func(t *testing.T) (*Diagram, error) {
				d, _, err := Restrict(promoted(t, fitting(plainDoor())), []string{"doors", "keys"}, []string{"keys"})
				return d, err
			},
			want: []FamilyStates{keys},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			d, err := tt.do(t)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, d.Families); diff != "" {
				t.Errorf("Families (-want +got):\n%s", diff)
			}
		})
	}
}
