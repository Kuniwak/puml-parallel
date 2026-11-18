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
