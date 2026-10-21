package promote_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
)

// A move takes stock out of one location and into another in one step: two
// instances of the family of locations take it, each in its own role.
var moveLocals = map[string]string{
	"local/SHIPMENT.puml": `@startuml SHIPMENT
state "なし" as shNone
state "計画済み" as shPlanned
state "移動済み" as shMoved
[*] --> shNone
shNone --> shPlanned : PLAN
shPlanned --> shMoved : MOVE?数量
@enduml
`,
	"local/LOCATION.puml": `@startuml LOCATION
state "なし" as locNone
state "稼働中" as locOpen
locOpen : 在庫
[*] --> locNone
locNone --> locOpen : OPEN ; ; 在庫' = 0
locOpen --> locOpen : DEPART?数量 ; 在庫 ≥ 数量 ; 在庫' = 在庫 - 数量
locOpen --> locOpen : ARRIVE?数量 ; ; 在庫' = 在庫 + 数量
@enduml
`,
}

const moveGlobal = `@startuml STOCK
state "稼働中" as running {
  running : shipments ; 出荷ID ⇸ Shipment
  running : locations ; 拠点ID ⇸ Location

  state "shipments : 出荷ID ⇸ Shipment" as runningShipments <<promote>> {
    !include local/SHIPMENT.puml
  }
  state "locations : 拠点ID ⇸ Location" as runningLocations <<promote>> {
    !include local/LOCATION.puml
  }
}

[*] --> running

note as sync1
  sync MOVE : shipments(出荷ID), locations(from).DEPART, locations(to).ARRIVE
end note
@enduml
`

func TestExpandMovesTwoInstancesOfOneFamilyInOneSync(t *testing.T) {
	// Execute
	x, diags := expand(t, moveGlobal, moveLocals)

	// Assert
	if x == nil {
		t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
	}
	want := "running --> running : MOVE?出荷ID?from?to?数量 ; " +
		"出荷ID ∈ dom shipments ∧ shipments(出荷ID) ∈ 〈計画済み〉 ∧ " +
		"from ∈ dom locations ∧ locations(from) ∈ 〈稼働中〉 ∧ 在庫 ≥ 数量 ∧ " +
		"to ∈ dom locations ∧ locations(to) ∈ 〈稼働中〉 ∧ from ≠ to ; " +
		"shipments' = shipments ⊕ {出荷ID ↦ 〈移動済み〉} ∧ " +
		"locations₁ = locations ⊕ {from ↦ 〈稼働中〉} ∧ 在庫' = 在庫 - 数量 ∧ " +
		"locations' = locations₁ ⊕ {to ↦ 〈稼働中〉} ∧ 在庫' = 在庫 + 数量"
	got := x.String()
	if !strings.Contains(got, want) {
		t.Errorf("promote.Expand() missing line:\n%s\ngot:\n%s", want, got)
	}
	for _, own := range []string{" : DEPART", " : ARRIVE"} {
		if strings.Contains(got, own) {
			t.Errorf("promote.Expand() kept %q of its own:\n%s", own, got)
		}
	}
}

func TestExpandSaysWhichInstanceEachClauseOfATwiceMovedFamilyIsAbout(t *testing.T) {
	// Execute
	x, diags := expand(t, moveGlobal, moveLocals)

	// Assert
	if x == nil {
		t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
	}
	var ids, distinct []string
	for _, e := range x.Edges {
		if e.Edge.Event.Channel() != "MOVE" {
			continue
		}
		for _, c := range append(e.Guard, e.Post...) {
			if c.Kind == promote.ClauseLocal && c.Map == "locations" {
				ids = append(ids, c.ID)
			}
			if c.Kind == promote.ClauseDistinct {
				distinct = c.IDs
			}
		}
	}
	if diff := cmp.Diff([]string{"from", "from", "to"}, ids); diff != "" {
		t.Errorf("the IDs the local clauses of locations are about mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"from", "to"}, distinct); diff != "" {
		t.Errorf("the IDs of the distinct clause mismatch (-want +got):\n%s", diff)
	}
}

func TestExpandRefusesASyncThatNamesOneInstanceTwice(t *testing.T) {
	cases := map[string]struct {
		directive string
		want      string
	}{
		"one ID for two roles": {
			directive: "sync MOVE : shipments(出荷ID), locations(from).DEPART, locations(from).ARRIVE",
			want:      `error: line 17: this sync names locations(from) twice; two occurrences of one map are two instances, so give them two IDs`,
		},
		"one role twice": {
			directive: "sync MOVE : shipments(出荷ID), locations(from).DEPART, locations(to).DEPART",
			want:      `error: line 17: this sync takes "DEPART" of "locations" twice; give each occurrence of one map its own local event`,
		},
		"a role no local diagram has": {
			directive: "sync MOVE : shipments(出荷ID), locations(from).LEAVE, locations(to).ARRIVE",
			want:      `error: line 17: local/LOCATION.puml has no edge on "LEAVE"`,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			global := strings.Replace(moveGlobal, "sync MOVE : shipments(出荷ID), locations(from).DEPART, locations(to).ARRIVE", c.directive, 1)

			// Execute
			x, diags := expand(t, global, moveLocals)

			// Assert
			if x != nil {
				t.Errorf("promote.Expand() expansion = %q, want none", x.String())
			}
			if !hasDiagnostic(diags, c.want) {
				t.Errorf("promote.Expand() diagnostics = %v, want one of them to be %q", diags, c.want)
			}
		})
	}
}

func TestParseGlobalReadsTheLocalEventOfASyncTarget(t *testing.T) {
	// Execute
	g, err := promote.ParseGlobal(moveGlobal)

	// Assert
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}
	want := []promote.MapRef{{Map: "shipments", Param: "出荷ID"}, {Map: "locations", Param: "from", Event: "DEPART"}, {Map: "locations", Param: "to", Event: "ARRIVE"}}
	if diff := cmp.Diff(want, g.Syncs[0].Targets); diff != "" {
		t.Errorf("sync targets mismatch (-want +got):\n%s", diff)
	}
}
