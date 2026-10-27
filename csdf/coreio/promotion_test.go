package coreio

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/types"
)

// location: its start state none means it does not exist; OPEN creates it
// and CLOSE deletes it (docs/PROMOTION.md).
const location = `@startuml
state "なし" as none
state "空き" as free
free : 在庫 ; 在庫
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
  state "locations : (l : 場所ID) ⇸ Location" as runningLocations <<promote>> {
    !include local/LOCATION.puml
  }
  state "shipments : (s : 出荷ID) ⇸ Shipment" as runningShipments <<promote>> {
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
				Labels: map[string]string{"running": "稼働中"},
				Modes:  []string{"running"}, Init: "running",
				Families: []core.Family{
					{Map: "locations", ID: "l", Key: types.Named("場所ID"), Val: types.Named("Location"), Local: loc},
					{Map: "shipments", ID: "s", Key: types.Named("出荷ID"), Val: types.Named("Shipment"), Local: shp},
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
  running : shipments ; 出荷ID ⇸ Shipment
  state "shipments : (s : 出荷ID) ->> Shipment" as runningShipments <<promote>> {
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
				Labels: map[string]string{"running": "稼働中"},
				Modes:  []string{"running"}, Init: "running",
				Families: []core.Family{{Map: "shipments", ID: "s", Key: types.Named("出荷ID"), Val: types.Named("Shipment"), Local: shp}},
				Syncs:    []core.Sync{{Event: "SHIP", Targets: []core.Target{{Map: "shipments", ID: "s", Event: "SHIP"}}}},
			},
		},
		{
			name: "a bare target takes the local event named like the sync",
			source: `@startuml
state "稼働中" as running {
  state "shipments : (s : 出荷ID) ⇸ Shipment" as runningShipments <<promote>> {
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
				Labels: map[string]string{"running": "稼働中"},
				Modes:  []string{"running"}, Init: "running",
				Families: []core.Family{{Map: "shipments", ID: "s", Key: types.Named("出荷ID"), Val: types.Named("Shipment"), Local: shp}},
				Syncs:    []core.Sync{{Event: "SHIP", Targets: []core.Target{{Map: "shipments", ID: "s", Event: "SHIP"}}}},
			},
		},
		{
			name: "variables of the global state that are no family's map are the controller's",
			source: `@startuml
state "稼働中" as running {
  running : shipments ; 出荷ID ⇸ Shipment
  running : 件数 ; 件数
  state "shipments : (s : 出荷ID) ⇸ Shipment" as runningShipments <<promote>> {
    !include local/SHIPMENT.puml
  }
}
state "保守" as maint
maint : shipments ; 出荷ID ⇸ Shipment
maint : 理由 ; 理由
[*] --> running : @empty(shipments') ∧ $件数' = 0
running --> maint : HALT ; ; $理由' = 停止
maint --> running : RESUME ; ; $件数' = 0
@enduml
`,
			want: core.Promotion{
				Labels: map[string]string{"maint": "保守", "running": "稼働中"},
				Modes:  []string{"running", "maint"}, Init: "running",
				Vars:     map[string][]string{"running": {"件数"}, "maint": {"理由"}},
				Types:    map[string]types.Type{"件数": types.Named("件数"), "理由": types.Named("理由")},
				InitPost: mustParse(t, "@empty(shipments') ∧ $件数' = 0"),
				Families: []core.Family{{Map: "shipments", ID: "s", Key: types.Named("出荷ID"), Val: types.Named("Shipment"), Local: shp, Frozen: []string{"maint"}}},
				Edges: []core.Edge{
					{Src: "running", Dst: "maint", Event: core.Event{Channel: "HALT"}, Post: mustParse(t, "$理由' = 停止")},
					{Src: "maint", Dst: "running", Event: core.Event{Channel: "RESUME"}, Post: mustParse(t, "$件数' = 0")},
				},
			},
		},
		{
			name: "a constrain on an event and its fields",
			source: `@startuml
state "稼働中" as running {
  state "shipments : (s : 出荷ID) ⇸ Shipment" as runningShipments <<promote>> {
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
				Labels: map[string]string{"running": "稼働中"},
				Modes:  []string{"running"}, Init: "running",
				Families:   []core.Family{{Map: "shipments", ID: "s", Key: types.Named("出荷ID"), Val: types.Named("Shipment"), Local: shp}},
				Constrains: []core.Constrain{{Event: core.Event{Channel: "ORDER", Fields: []core.Field{{Name: "s"}}}, Guard: mustParse(t, "@empty(shipments)")}},
			},
		},
		{
			name: "modes, a bodyless block, a frozen family, hand-written edges, the start post and an assumption",
			source: `@startuml
state "稼働中" as running {
  running : shipments ; 出荷ID ⇸ Shipment
  running : locations ; 場所ID ⇸ Location
  state "shipments : (s : 出荷ID) ⇸ Shipment" as runningShipments <<promote>> {
    !include local/SHIPMENT.puml
  }
  state "locations : (l : 場所ID) ⇸ Location" as runningLocations <<promote>> {
    !include local/LOCATION.puml
  }
}
state "縮退中" as degraded {
  degraded : shipments ; 出荷ID ⇸ Shipment
  degraded : locations ; 場所ID ⇸ Location
  state "shipments : (s : 出荷ID) ⇸ Shipment" as degradedShipments <<promote>>
}
[*] --> running : @empty(shipments') ∧ @empty(locations')
running --> degraded : HALT?why ; @empty(locations)
degraded --> running : RESUME
note as channels
  channel HALT : 理由
end note
note as fair
  assume いずれ RESUME が起きる
end note
@enduml
`,
			want: core.Promotion{
				Labels: map[string]string{"degraded": "縮退中", "running": "稼働中"},
				Modes:  []string{"running", "degraded"}, Init: "running",
				InitPost: mustParse(t, "@empty(shipments') ∧ @empty(locations')"),
				Families: []core.Family{
					{Map: "shipments", ID: "s", Key: types.Named("出荷ID"), Val: types.Named("Shipment"), Local: shp},
					{Map: "locations", ID: "l", Key: types.Named("場所ID"), Val: types.Named("Location"), Local: loc, Frozen: []string{"degraded"}},
				},
				Edges: []core.Edge{
					{Src: "running", Dst: "degraded", Event: core.Event{Channel: "HALT", Fields: []core.Field{{Name: "why"}}}, Guard: mustParse(t, "@empty(locations)")},
					{Src: "degraded", Dst: "running", Event: core.Event{Channel: "RESUME"}},
				},
				Assumes:  []core.Assumption{{ID: "fair", Text: "いずれ RESUME が起きる"}},
				Channels: map[string][]types.Type{"HALT": {types.Named("理由")}},
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

// promoting is a global diagram of one family, shipments, with body after
// the block.
func promoting(body string) string {
	return fmt.Sprintf(`@startuml
state "稼働中" as running {
  state "shipments : (s : 出荷ID) ⇸ Shipment" as rs <<promote>> {
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
		{"a variable declared for a state that is no mode",
			promoting("[*] --> running\nnowhere : 件数 ; 件数\n"), "unexpected"},
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
		{"a global end edge",
			promoting("[*] --> running\nrunning --> [*]\n"), "a global end edge is not supported yet"},
		{"the blocks of one map writing its ID apart",
			promoting("state \"保守\" as maintenance {\n  state \"shipments : (t : 出荷ID) ⇸ Shipment\" as ms <<promote>>\n}\n[*] --> running\n"), "write its ID as s and as t"},
		{"a second sync of one event",
			promoting("[*] --> running\nnote as n\n  sync SHIP : shipments(s)\nend note\nnote as m\n  sync SHIP : shipments(s)\nend note\n"), "a second sync of SHIP"},
		{"a second block of one map",
			strings.Replace(promoting("[*] --> running\n"), "}\n}\n", "}\n  state \"shipments : (t : 出荷ID) ⇸ Shipment\" as rt <<promote>> {\n    !include S.puml\n  }\n}\n", 1), "a second <<promote>> block of shipments"},
		{"!include_many",
			strings.Replace(promoting("[*] --> running\n"), "!include S.puml", "!include_many S.puml", 1), "!include_many"},
		{"!includesub",
			strings.Replace(promoting("[*] --> running\n"), "!include S.puml", "!includesub S.puml!x", 1), "!includesub"},
		{"an unterminated CSDF-IGNORE region",
			promoting("[*] --> running\n' CSDF-IGNORE-BEGIN\nskinparam x y\n"), "unterminated CSDF-IGNORE region"},
		{"a sync of an unknown map",
			promoting("[*] --> running\nnote as n\n  sync X : others(o)\nend note\n"), "others"},
		{"a block title without the instance's type",
			strings.Replace(promoting("[*] --> running\n"), "(s : 出荷ID) ⇸ Shipment", "s", 1), "<map> : (<ID> : <Key>) ⇸ <Type>"},
		{"a block title giving the IDs no type",
			strings.Replace(promoting("[*] --> running\n"), "(s : 出荷ID) ⇸ Shipment", "s ⇸ Shipment", 1), `its title "shipments : s ⇸ Shipment" gives the IDs no type; write it as "shipments : (s : <type of the IDs>) ⇸ Shipment"`},
		{"a map declared of another type than its block's",
			promoting("running : shipments ; 場所ID ⇸ Shipment\n[*] --> running\n"), "running declares the map shipments of the type 場所ID ⇸ Shipment, but its <<promote>> block gives it the type 出荷ID ⇸ Shipment"},
		{"a controller variable with no type",
			promoting("running : 件数\n[*] --> running\n"), "the variable 件数 of running has no type"},
		{"a hand-written edge on an undeclared channel with fields",
			promoting("[*] --> running\nrunning --> running : HALT?why\n"), "the channel HALT carries fields, and no channel note declares their types"},
		{"a target that is not map(id)",
			promoting("[*] --> running\nnote as n\n  sync X : shipments\nend note\n"), "map(id)"},
		{"a mode whose ID is a reserved word",
			promoting("state \"保守\" as null\n[*] --> running\n"), "null is a reserved word"},
		{"a controller variable starting with a digit",
			promoting("running : 1件数 ; 1件数\n[*] --> running\n"), "does not start with a digit, as 1件数"},
		{"a promote block whose ID is a reserved word",
			strings.Replace(promoting("[*] --> running\n"), "(s : 出荷ID)", "(true : 出荷ID)", 1), "true is a reserved word"},
		{"a promote block whose map starts with a digit",
			strings.Replace(promoting("[*] --> running\n"), "\"shipments : (s", "\"2shipments : (s", 1), "does not start with a digit, as 2shipments"},
		{"a sync of an event that is a reserved word",
			promoting("[*] --> running\nnote as n\n  sync false : shipments(s)\nend note\n"), "false is a reserved word"},
		{"a sync target whose ID starts with a digit",
			promoting("[*] --> running\nnote as n\n  sync SHIP : shipments(0s)\nend note\n"), "does not start with a digit, as 0s"},
		{"a note ID that is a reserved word",
			promoting("[*] --> running\nnote as null\n  sync SHIP : shipments(s)\nend note\n"), "null is a reserved word"},
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

func TestReadPromotionNamesTheLineOfABadEdgeOnce(t *testing.T) {
	// Execute
	_, err := ReadPromotion(promoting("[*] --> running\nrunning --> running : a??\n"), files(map[string]string{"S.puml": shipment}))

	// Assert
	if err == nil || strings.Count(err.Error(), "line 8") != 1 {
		t.Errorf("ReadPromotion: got %v, want an error naming line 8 once", err)
	}
}

func TestReadPromotionGivesWhatPromoteTakes(t *testing.T) {
	cases := []struct{ name, source string }{
		{"one mode and a sync", `@startuml
state "稼働中" as running {
  state "locations : (l : 場所ID) ⇸ Location" as rl <<promote>> {
    !include L.puml
  }
}
[*] --> running
note as n
  sync MOVE : locations(from).DEPART, locations(to).ARRIVE
end note
@enduml
`},
		{"controller variables", `@startuml
state "稼働中" as running {
  running : 件数 ; 件数
  state "locations : (l : 場所ID) ⇸ Location" as rl <<promote>> {
    !include L.puml
  }
}
state "保守" as maint
maint : locations ; 場所ID ⇸ Location
maint : 件数 ; 件数
[*] --> running : $件数' = 0
running --> maint : HALT ; $件数 = 0 ; $件数' = 1
maint --> running : RESUME ; ; $件数' = $件数
@enduml
`},
		{"two modes, a frozen family and an assumption", `@startuml
state "稼働中" as running {
  state "locations : (l : 場所ID) ⇸ Location" as rl <<promote>> {
    !include L.puml
  }
}
state "保守" as maint {
  maint : locations ; 場所ID ⇸ Location
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
