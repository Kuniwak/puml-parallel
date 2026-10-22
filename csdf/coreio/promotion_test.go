package coreio

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// location: its start state none means it does not exist; OPEN creates it
// and CLOSE deletes it (docs/PROMOTION.md).
const location = `@startuml
state "なし" as none
state "空き" as free
free : 在庫
[*] --> none
none --> free : OPEN ; ; $在庫' = 0
free --> free : DEPART ; $在庫 > 0 ; $在庫' = $在庫 - 1
free --> free : ARRIVE ; ; $在庫' = $在庫 + 1
free --> none : CLOSE ; $在庫 = 0
@enduml
`

// shipment: ORDER creates it waiting; SHIP ships it.
const shipment = `@startuml
state "なし" as none
state "待ち" as waiting
state "済み" as done
[*] --> none
none --> waiting : ORDER
waiting --> done : SHIP
@enduml
`

// files is an include that reads from a map of paths to sources.
func files(m map[string]string) Include {
	return func(path string) (string, error) {
		s, ok := m[path]
		if !ok {
			return "", fmt.Errorf("no file %s", path)
		}
		return s, nil
	}
}

func TestReadPromotion(t *testing.T) {
	loc, err := ReadAuthored(location)
	if err != nil {
		t.Fatal(err)
	}
	shp, err := ReadAuthored(shipment)
	if err != nil {
		t.Fatal(err)
	}
	include := files(map[string]string{"local/LOCATION.puml": location, "local/SHIPMENT.puml": shipment})
	cases := []struct {
		name, source string
		want         core.Promotion
	}{
		{
			name: "families and a sync, a picture's lines aside",
			source: `@startuml MOVE
' CSDF-IGNORE-BEGIN
title 移動
' CSDF-IGNORE-END
!define PROMOTED
state "稼働中" as running {
  state "locations : l ⇸ Location" as runningLocations <<promote>> {
    !include local/LOCATION.puml
  }
  state "shipments : s ⇸ Shipment" as runningShipments <<promote>> {
    !include local/SHIPMENT.puml
  }
}
[*] --> running

note as move
  sync MOVE : shipments(s).SHIP, locations(from).DEPART, locations(to).ARRIVE ; ${locations[from].在庫} > 1
end note
@enduml
`,
			want: core.Promotion{
				Modes: []string{"running"}, Init: "running",
				Families: []core.Family{
					{Map: "locations", ID: "l", Local: loc},
					{Map: "shipments", ID: "s", Local: shp},
				},
				Syncs: []core.Sync{
					{Event: "MOVE", Guard: mustParse(t, "${locations[from].在庫} > 1"), Targets: []core.Target{
						{Map: "shipments", ID: "s", Event: "SHIP"},
						{Map: "locations", ID: "from", Event: "DEPART"},
						{Map: "locations", ID: "to", Event: "ARRIVE"},
					}},
				},
			},
		},
		{
			name: "map declarations, an anchored note, a note for the picture, and ->> in a title",
			source: `@startuml
state "稼働中" as running {
  running : shipments ; s ⇸ Shipment
  state "shipments : s ->> Shipment" as runningShipments <<promote>> {
    !include "local/SHIPMENT.puml"
  }
}
[*] --> running
note as picture
  出荷は一件ずつ
  sync IGNORED : others(o)
end note
note bottom of runningShipments
  sync SHIP : shipments(s)
  出荷する
end note
@enduml
`,
			want: core.Promotion{
				Modes: []string{"running"}, Init: "running",
				Families: []core.Family{{Map: "shipments", ID: "s", Local: shp}},
				Syncs:    []core.Sync{{Event: "SHIP", Targets: []core.Target{{Map: "shipments", ID: "s", Event: "SHIP"}}}},
			},
		},
		{
			name: "a bare target takes the local event named like the sync",
			source: `@startuml
state "稼働中" as running {
  state "shipments : s ⇸ Shipment" as runningShipments <<promote>> {
    !include local/SHIPMENT.puml
  }
}
[*] --> running
note as n
  sync SHIP : shipments(s)
end note
@enduml
`,
			want: core.Promotion{
				Modes: []string{"running"}, Init: "running",
				Families: []core.Family{{Map: "shipments", ID: "s", Local: shp}},
				Syncs:    []core.Sync{{Event: "SHIP", Targets: []core.Target{{Map: "shipments", ID: "s", Event: "SHIP"}}}},
			},
		},
		{
			name: "a constrain on an event and its fields",
			source: `@startuml
state "稼働中" as running {
  state "shipments : s ⇸ Shipment" as runningShipments <<promote>> {
    !include local/SHIPMENT.puml
  }
}
[*] --> running
note bottom of runningShipments
  constrain ORDER?s ; @empty(shipments)
  一度に一件
end note
@enduml
`,
			want: core.Promotion{
				Modes: []string{"running"}, Init: "running",
				Families:   []core.Family{{Map: "shipments", ID: "s", Local: shp}},
				Constrains: []core.Constrain{{Event: core.Event{Channel: "ORDER", Fields: []core.Field{{Name: "s"}}}, Guard: mustParse(t, "@empty(shipments)")}},
			},
		},
		{
			name: "modes, a bodyless block, a frozen family, hand-written edges, the start post and an assumption",
			source: `@startuml
state "稼働中" as running {
  running : shipments ; s ⇸ Shipment
  running : locations ; l ⇸ Location
  state "shipments : s ⇸ Shipment" as runningShipments <<promote>> {
    !include local/SHIPMENT.puml
  }
  state "locations : l ⇸ Location" as runningLocations <<promote>> {
    !include local/LOCATION.puml
  }
}
state "縮退中" as degraded {
  degraded : shipments ; s ⇸ Shipment
  degraded : locations ; l ⇸ Location
  state "shipments : s ⇸ Shipment" as degradedShipments <<promote>>
}
[*] --> running : @empty(shipments') ∧ @empty(locations')
running --> degraded : HALT?why ; @empty(locations)
degraded --> running : RESUME
note as fair
  assume いずれ RESUME が起きる
end note
@enduml
`,
			want: core.Promotion{
				Modes: []string{"running", "degraded"}, Init: "running",
				InitPost: mustParse(t, "@empty(shipments') ∧ @empty(locations')"),
				Families: []core.Family{
					{Map: "shipments", ID: "s", Local: shp},
					{Map: "locations", ID: "l", Local: loc, Frozen: []string{"degraded"}},
				},
				Edges: []core.Edge{
					{Src: "running", Dst: "degraded", Event: core.Event{Channel: "HALT", Fields: []core.Field{{Name: "why"}}}, Guard: mustParse(t, "@empty(locations)")},
					{Src: "degraded", Dst: "running", Event: core.Event{Channel: "RESUME"}},
				},
				Assumes: []core.Assumption{{ID: "fair", Text: "いずれ RESUME が起きる"}},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			got, err := ReadPromotion(c.source, include)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got, sameClauses); diff != "" {
				t.Errorf("ReadPromotion (-want +got):\n%s", diff)
			}
		})
	}
}

// refusedYet ends the refusal of what the formula promotion does not read yet.
const refusedYet = "is not supported yet by the formula promotion (step 5½b of #76)"

// promoting is a global diagram of one family, shipments, with body after
// the block.
func promoting(body string) string {
	return fmt.Sprintf(`@startuml
state "稼働中" as running {
  state "shipments : s ⇸ Shipment" as rs <<promote>> {
    !include S.puml
  }
}
%s@enduml
`, body)
}

func TestReadPromotionRefuses(t *testing.T) {
	cases := []struct {
		name, source, want string
	}{
		{"a constrain with an empty guard",
			promoting("[*] --> running\nnote as n\n  constrain ORDER?s ;\nend note\n"), "empty guard"},
		{"a constrain on a malformed event",
			promoting("[*] --> running\nnote as n\n  constrain ORDER?? ; @empty(shipments)\nend note\n"), "constrain ORDER??"},
		{"a constrain with a malformed guard",
			promoting("[*] --> running\nnote as n\n  constrain ORDER?s ; @ne(s)\nend note\n"), "constrain ORDER?s"},
		{"a constrain without its guard",
			promoting("[*] --> running\nnote as n\n  constrain ORDER?s\nend note\n"), "constrain ORDER?s"},
		{"a variable of the global state that is no family's map",
			strings.Replace(promoting("[*] --> running\n"), "running {\n", "running {\n  running : 件数\n", 1), "a variable of the global state that is no family's map " + refusedYet},
		{"a mode holding no map of a family",
			promoting("state \"保守\" as maintenance\n[*] --> running\n"), "maintenance holds no map shipments"},
		{"a composite mode holding no map of a family",
			promoting("state \"保守\" as maintenance {\n}\n[*] --> running\n"), "maintenance holds no map shipments"},
		{"only bodyless blocks of a map",
			strings.Replace(promoting("[*] --> running\n"), "<<promote>> {\n    !include S.puml\n  }", "<<promote>>", 1), "no block of shipments includes its local diagram"},
		{"an anchored assume note",
			promoting("[*] --> running\nnote bottom of rs\n  assume いつか止まる\nend note\n"), "about the whole system"},
		{"a hand-written edge whose label does not parse",
			promoting("[*] --> running\nrunning --> running : a?? ; ; \n"), "line 8"},
		{"a hand-written edge into no state",
			promoting("[*] --> running\nrunning --> nowhere : a\n"), "line 8: running --> nowhere names nowhere"},
		{"a hand-written edge into the end",
			promoting("[*] --> running\nrunning --> [*]\n"), "no end edge"},
		{"a second sync of one event",
			promoting("[*] --> running\nnote as n\n  sync SHIP : shipments(s)\nend note\nnote as m\n  sync SHIP : shipments(s)\nend note\n"), "a second sync of SHIP"},
		{"a second block of one map",
			strings.Replace(promoting("[*] --> running\n"), "}\n}\n", "}\n  state \"shipments : t ⇸ Shipment\" as rt <<promote>> {\n    !include S.puml\n  }\n}\n", 1), "a second <<promote>> block of shipments"},
		{"!include_many",
			strings.Replace(promoting("[*] --> running\n"), "!include S.puml", "!include_many S.puml", 1), "!include_many"},
		{"!includesub",
			strings.Replace(promoting("[*] --> running\n"), "!include S.puml", "!includesub S.puml!x", 1), "!includesub"},
		{"an unterminated CSDF-IGNORE region",
			promoting("[*] --> running\n' CSDF-IGNORE-BEGIN\nskinparam x y\n"), "unterminated CSDF-IGNORE region"},
		{"a sync of an unknown map",
			promoting("[*] --> running\nnote as n\n  sync X : others(o)\nend note\n"), "others"},
		{"a block title without the instance's type",
			strings.Replace(promoting("[*] --> running\n"), "s ⇸ Shipment", "s", 1), "<map> : <ID> ⇸ <Type>"},
		{"a target that is not map(id)",
			promoting("[*] --> running\nnote as n\n  sync X : shipments\nend note\n"), "map(id)"},
		{"a missing include",
			strings.Replace(promoting("[*] --> running\n"), "S.puml", "T.puml", 1), "no file T.puml"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			_, err := ReadPromotion(c.source, files(map[string]string{"S.puml": shipment}))

			// Assert
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("ReadPromotion: got %v, want an error containing %q", err, c.want)
			}
		})
	}
}

func TestReadPromotionGivesWhatPromoteTakes(t *testing.T) {
	cases := []struct{ name, source string }{
		{"one mode and a sync", `@startuml
state "稼働中" as running {
  state "locations : l ⇸ Location" as rl <<promote>> {
    !include L.puml
  }
}
[*] --> running
note as n
  sync MOVE : locations(from).DEPART, locations(to).ARRIVE
end note
@enduml
`},
		{"two modes, a frozen family and an assumption", `@startuml
state "稼働中" as running {
  state "locations : l ⇸ Location" as rl <<promote>> {
    !include L.puml
  }
}
state "保守" as maint {
  maint : locations ; l ⇸ Location
}
[*] --> running
running --> maint : HALT ; @empty(locations)
maint --> running : RESUME
note as fair
  assume いずれ RESUME が起きる
end note
@enduml
`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			p, err := ReadPromotion(c.source, files(map[string]string{"L.puml": location}))
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			d, _, err := core.Promote(p)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Write(d); err != nil {
				t.Errorf("Write(Promote(p)): %v", err)
			}
		})
	}
}
