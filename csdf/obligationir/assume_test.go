package obligationir

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/google/go-cmp/cmp"
)

const assumeSource = `@startuml
state "a" as a
a: n ; Nat
[*] --> a : n = 10
a --> a : tau ; n > 0 ; n' = n - 1
note as fair
  assume n reaches 0 eventually
end note
@enduml
`

func TestBuildLivelockFreeCarriesTheAssumptions(t *testing.T) {
	ir := BuildLivelockFree(csdf.MustParse(assumeSource))

	want := []IRAssumption{{ID: "fair", Text: "n reaches 0 eventually", Line: 7}}
	if diff := cmp.Diff(want, ir.Assumptions); diff != "" {
		t.Error(diff)
	}
}

func TestBuildLivelockFreeAssumptionsAreNeverNull(t *testing.T) {
	ir := BuildLivelockFree(csdf.MustParse("@startuml\n[*] --> a\na --> a : tau\n@enduml\n"))
	bs, err := json.Marshal(ir)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !strings.Contains(string(bs), `"assumptions":[]`) {
		t.Errorf("want \"assumptions\":[] in the JSON, got %s", bs)
	}
}

func TestBuildRefinementCarriesEachSidesAssumptions(t *testing.T) {
	spec := csdf.MustParse("@startuml\n[*] --> s\ns --> s : a\n@enduml\n")
	impl := csdf.MustParse(assumeSource)

	ir := BuildRefinement(IRRefinementModeStableFailure, spec, impl)

	if len(ir.Spec.Assumptions) != 0 {
		t.Errorf("Spec assumptions = %v, want none", ir.Spec.Assumptions)
	}
	want := []IRAssumption{{ID: "fair", Text: "n reaches 0 eventually", Line: 7}}
	if diff := cmp.Diff(want, ir.Impl.Assumptions); diff != "" {
		t.Error(diff)
	}
	bs, _ := json.Marshal(ir.Spec)
	if !strings.Contains(string(bs), `"assumptions":[]`) {
		t.Errorf("want \"assumptions\":[] on a side without assumptions, got %s", bs)
	}
}

func TestAssumptionNameIsSideQualifiedAndMangled(t *testing.T) {
	if got := SideSingle.AssumptionName("fair"); got != "assume_fair" {
		t.Errorf("SideSingle.AssumptionName(fair) = %q", got)
	}
	if got := SideImpl.AssumptionName("公平性"); !strings.HasPrefix(got, "assume_") || !strings.HasSuffix(got, "_I") || strings.Contains(got, "公平性") {
		t.Errorf("SideImpl.AssumptionName(公平性) = %q, want a mangled name with the side suffix", got)
	}
}

func TestLivelockFreeNameTableListsAMangledAssumptionID(t *testing.T) {
	ir := BuildLivelockFree(csdf.MustParse("@startuml\n[*] --> a\na --> a : tau\nnote as 公平性\n  assume x\nend note\n@enduml\n"))
	found := false
	for _, e := range LivelockFreeNameTable(ir) {
		found = found || e.Original == "公平性"
	}
	if !found {
		t.Errorf("LivelockFreeNameTable() = %v, want the assumption id 公平性 among the mangled names", LivelockFreeNameTable(ir))
	}
}
