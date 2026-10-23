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
	doors := FamilyStates{Map: "doors", Init: "none", States: []string{"none", "shut"}, Vars: map[string][]string{}}
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
		"Hide keeps them": {
			do: func(t *testing.T) (*Diagram, error) {
				return Hide(promoted(t, fitting(plainDoor())), []string{"INSTALL"})
			},
			want: []FamilyStates{doors, keys},
		},
		"Compose renames a map the left one also has, in its family as in its variables": {
			do: func(t *testing.T) (*Diagram, error) {
				return Compose(promoted(t, fitting(plainDoor())), promoted(t, fitting(plainDoor())), nil)
			},
			want: []FamilyStates{doors, keys, underMap(doors, "doors2"), underMap(keys, "keys2")},
		},
		"ComposeAll carries those of each": {
			do: func(t *testing.T) (*Diagram, error) {
				return ComposeAll([]*Diagram{
					promoted(t, fitting(plainDoor())),
					promoted(t, Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{{Map: "locations", ID: "loc", Local: location(t)}}}),
					promoted(t, Promotion{Modes: []string{"G"}, Init: "G", Families: []Family{{Map: "shut", ID: "d", Local: door()}}}),
				}, nil)
			},
			want: []FamilyStates{doors, keys,
				{Map: "locations", Init: "none", States: []string{"none", "open"}, Vars: map[string][]string{"open": {"stock"}}},
				{Map: "shut", Init: "none", States: []string{"none", "shut", "wide"}, Vars: map[string][]string{}}},
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
			if err := d.familiesWellFormed(); err != nil {
				t.Error(err)
			}
		})
	}
}

// underMap is f of the map to.
func underMap(f FamilyStates, to string) FamilyStates {
	f.Map = to
	return f
}

func TestEachOperationCopiesTheLocalStatesOfTheFamilies(t *testing.T) {
	tests := map[string]func(d *Diagram) (*Diagram, error){
		"Restrict": func(d *Diagram) (*Diagram, error) {
			out, _, err := Restrict(d, []string{"doors", "keys"}, []string{"doors", "keys"})
			return out, err
		},
		"Hide":    func(d *Diagram) (*Diagram, error) { return Hide(d, nil) },
		"Compose": func(d *Diagram) (*Diagram, error) { return Compose(&Diagram{States: []string{"a"}, Init: "a"}, d, nil) },
	}
	for name, do := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			d := promoted(t, fitting(location(t)))
			want := promoted(t, fitting(location(t))).Families

			// Execute
			out, err := do(d)
			if err != nil {
				t.Fatal(err)
			}
			for i := range out.Families {
				out.Families[i].States[0] = "changed"
				for s := range out.Families[i].Vars {
					out.Families[i].Vars[s][0] = "changed"
				}
				out.Families[i].Vars["changed"] = nil
			}

			// Assert
			if diff := cmp.Diff(want, d.Families); diff != "" {
				t.Errorf("the families of the operand (-want +got):\n%s", diff)
			}
		})
	}
}
