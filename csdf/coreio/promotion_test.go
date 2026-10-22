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
				State: "running",
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
				State:    "running",
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
				State:    "running",
				Families: []core.Family{{Map: "shipments", ID: "s", Local: shp}},
				Syncs:    []core.Sync{{Event: "SHIP", Targets: []core.Target{{Map: "shipments", ID: "s", Event: "SHIP"}}}},
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
		{"a post on the start edge",
			promoting("[*] --> running : 何か\n"), "a post on the start edge " + refusedYet},
		{"a hand-written edge of the global diagram",
			promoting("[*] --> running\nrunning --> running : a\n"), "a hand-written edge of the global diagram " + refusedYet},
		{"a constrain",
			promoting("[*] --> running\nnote as n\n  constrain a ; b\nend note\n"), "constrain " + refusedYet},
		{"a global assume note",
			promoting("[*] --> running\nnote as n\n  assume いつか止まる\nend note\n"), "an assume note in the global diagram " + refusedYet},
		{"a second global state",
			promoting("state \"保守\" as maintenance\n[*] --> running\n"), "a second global state " + refusedYet},
		{"a second composite global state",
			promoting("state \"保守\" as maintenance {\n}\n[*] --> running\n"), "a second global state " + refusedYet},
		{"a bodyless block",
			strings.Replace(promoting("[*] --> running\n"), "<<promote>> {\n    !include S.puml\n  }", "<<promote>>", 1), "a bodyless <<promote>> block " + refusedYet},
		{"a variable of the global state that is no family's map",
			strings.Replace(promoting("[*] --> running\n"), "running {\n", "running {\n  running : 件数\n", 1), "a variable of the global state that is no family's map " + refusedYet},
		{"a second sync of one event",
			promoting("[*] --> running\nnote as n\n  sync SHIP : shipments(s)\nend note\nnote as m\n  sync SHIP : shipments(s)\nend note\n"), "a second sync of SHIP"},
		{"a second block of one map",
			strings.Replace(promoting("[*] --> running\n"), "}\n}\n", "}\n  state \"shipments : t ⇸ Shipment\" as rt <<promote>> {\n    !include S.puml\n  }\n}\n", 1), "a second <<promote>> block of shipments"},
		{"!include_many",
			strings.Replace(promoting("[*] --> running\n"), "!include S.puml", "!include_many S.puml", 1), "!include_many"},
		{"!includesub",
			strings.Replace(promoting("[*] --> running\n"), "!include S.puml", "!includesub S.puml!x", 1), "!includesub"},
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
	// Setup
	source := `@startuml
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
`
	p, err := ReadPromotion(source, files(map[string]string{"L.puml": location}))
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	d, err := core.Promote(p)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Write(d); err != nil {
		t.Errorf("Write(Promote(p)): %v", err)
	}
}
