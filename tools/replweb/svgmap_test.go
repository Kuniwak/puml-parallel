package replweb

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/google/go-cmp/cmp"
)

// svgOfTwoStates is what PlantUML writes for a diagram whose start edge is
// on its 4th line and whose edges are on its 5th and 6th, the attributes of
// the links in more than one order.
const svgOfTwoStates = `<svg xmlns="http://www.w3.org/2000/svg"><?plantuml 1.2026.10?><g>
<g class="entity" data-qualified-name="idle" id="ent0002" data-source-line="1"><rect/></g>
<g class="entity" data-qualified-name="busy" id="ent0003" data-source-line="2"><rect/></g>
<g class="link" data-entity-1="ent0001" data-entity-2="ent0002" data-source-line="3" id="lnk4"><path/></g>
<g id="lnk5" data-source-line="4" class="link" data-entity-2="ent0003" data-entity-1="ent0002"><path/></g>
<g class="link" data-entity-1="ent0003" data-entity-2="ent0002" id="lnk6" data-source-line="5"><path/></g>
</g></svg>`

func TestMapSVGMapsStatesAndEdgesByTheirSourceLines(t *testing.T) {
	// Setup
	d := &core.Diagram{
		States:   []string{"idle", "busy"},
		Init:     "idle",
		InitLine: 4,
		Edges: []core.Edge{
			{Src: "idle", Dst: "busy", Line: 5},
			{Src: "busy", Dst: "idle", Line: 6},
		},
	}

	// Execute
	got, err := MapSVG([]byte(svgOfTwoStates), d)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := Highlights{
		States: map[string]string{"idle": "ent0002", "busy": "ent0003"},
		Links:  map[int]string{5: "lnk5", 6: "lnk6"},
		Start:  "lnk4",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("MapSVG mismatch (-want +got):\n%s", diff)
	}
}

func TestMapSVGLeavesOutAnEdgeOfNoLink(t *testing.T) {
	// Setup
	d := &core.Diagram{
		States: []string{"idle", "busy"},
		Init:   "idle",
		Edges:  []core.Edge{{Src: "idle", Dst: "busy", Line: 0}},
	}

	// Execute
	got, err := MapSVG([]byte(svgOfTwoStates), d)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := Highlights{States: map[string]string{}, Links: map[int]string{}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("MapSVG mismatch (-want +got):\n%s", diff)
	}
}

func TestMapSVGRefusesWhatIsNoXML(t *testing.T) {
	// Execute
	_, err := MapSVG([]byte(`<svg><g class="link"`), &core.Diagram{})

	// Assert
	if err == nil {
		t.Error("want an error, got nil")
	}
}

// svgOfPromotion is what PlantUML writes for a mode m whose <<promote>>
// block b draws a local diagram of the states none and on, with two
// parallel edges from none to on, one back, and a controller edge from m
// to itself read on the 9th line; the local links carry lines of the file
// the block includes, one of them the same as the controller's.
const svgOfPromotion = `<svg xmlns="http://www.w3.org/2000/svg"><g>
<g class="cluster" data-qualified-name="m" id="ent0001" data-source-line="3"><rect/></g>
<g class="entity" data-qualified-name="m.b" id="ent0002" data-source-line="5"><rect/></g>
<g class="entity" data-qualified-name="m.b.none" id="ent0003" data-source-line="1"><rect/></g>
<g class="entity" data-qualified-name="m.b.on" id="ent0004" data-source-line="2"><rect/></g>
<g class="start_entity" data-qualified-name="m.b..start.b" id="ent0005" data-source-line="3"><ellipse/></g>
<g class="link" data-entity-1="ent0005" data-entity-2="ent0003" id="lnk6" data-source-line="3"><path/></g>
<g class="link" data-entity-1="ent0003" data-entity-2="ent0004" id="lnk7" data-source-line="4"><path/></g>
<g class="link" data-entity-1="ent0003" data-entity-2="ent0004" id="lnk8" data-source-line="5"><path/></g>
<g class="link" data-entity-1="ent0004" data-entity-2="ent0003" id="lnk9" data-source-line="8"><path/></g>
<g class="start_entity" data-qualified-name=".start." id="ent0010" data-source-line="7"><ellipse/></g>
<g class="link" data-entity-1="ent0010" data-entity-2="ent0001" id="lnk11" data-source-line="7"><path/></g>
<g class="link" data-entity-1="ent0001" data-entity-2="ent0001" id="lnk12" data-source-line="8"><path/></g>
</g></svg>`

func diagramOfPromotion() *core.Diagram {
	return &core.Diagram{
		States: []string{"m"},
		Init:   "m",
		Families: []core.FamilyStates{
			{Map: "xs", Init: "none", States: []string{"none", "on"}, Blocks: map[string]string{"m": "b"}},
		},
		Edges: []core.Edge{
			{Src: "m", Dst: "m", Moves: []core.Move{{Map: "xs", Edge: 0, Src: "none", Dst: "on", Parallel: 0}}},
			{Src: "m", Dst: "m", Moves: []core.Move{{Map: "xs", Edge: 1, Src: "none", Dst: "on", Parallel: 1}}},
			{Src: "m", Dst: "m", Moves: []core.Move{{Map: "xs", Edge: 2, Src: "on", Dst: "none", Parallel: 0}}},
			{Src: "m", Dst: "m", Line: 9},
		},
	}
}

func TestMapSVGMapsTheLocalDiagramsOfAPromotion(t *testing.T) {
	// Execute
	got, err := MapSVG([]byte(svgOfPromotion), diagramOfPromotion())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := Highlights{
		States: map[string]string{"m": "ent0001"},
		Links:  map[int]string{9: "lnk12"},
		Start:  "lnk11",
		Families: map[string]FamilyHighlights{
			"xs": {
				States: map[string]string{"none": "ent0003", "on": "ent0004"},
				Edges:  map[int]string{0: "lnk7", 1: "lnk8", 2: "lnk9"},
				Start:  "lnk6",
				Block:  "ent0002",
			},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("MapSVG mismatch (-want +got):\n%s", diff)
	}
}

func TestMapSVGLeavesOutAFamilyItsBlocksDoNotDraw(t *testing.T) {
	// Setup
	d := diagramOfPromotion()
	d.Families[0].Blocks = map[string]string{"m": "elsewhere"}

	// Execute
	got, err := MapSVG([]byte(svgOfPromotion), d)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got.Families["xs"]; ok {
		t.Errorf("want no highlights of xs, got %v", got.Families["xs"])
	}
}
