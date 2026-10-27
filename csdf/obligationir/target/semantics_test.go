package target_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v3/csdf/obligationir"
	"github.com/Kuniwak/puml-parallel/v3/csdf/obligationir/target"
)

func read(t *testing.T, src string) *core.Diagram {
	t.Helper()
	d, err := coreio.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestARefinementMeansWhatCSPSays holds the Isabelle encoding to the CSP each
// construct means, written by hand rather than read off the generator.
func TestARefinementMeansWhatCSPSays(t *testing.T) {
	testCases := map[string]struct {
		Impl string
		Want string
	}{
		"a state with no edge is STOP": {
			Impl: "@startuml\nstate \"a\" as a\nstate \"b\" as b\n[*] --> a\na --> b : go\n@enduml\n",
			Want: "\"procfun (I_b) =\n     STOP\"",
		},
		"an edge no value enables offers the events of an empty set": {
			Impl: "@startuml\nstate \"a\" as a\na : n ; n\n[*] --> a : @eq(n', n')\na --> a : go ; @ne(n, n) ; @eq(n', n)\n@enduml\n",
			Want: `(? ev:{ev. ev = Alphabet Ev_go \<and> (\<exists>(v_n_u27u::val). type_n v_n_u27u \<and> csdf_wf v_n_u27u \<and> ((\<not> (v_n = v_n)) \<and> v_n_u27u = v_n))} -> `,
		},
		"an end edge is SKIP when its guard holds, and STOP otherwise": {
			Impl: "@startuml\nstate \"a\" as a\n[*] --> a\na --> a : go\na --> [*] : 終わってよい\n@enduml\n",
			Want: "\n     [+]\n     (IF (atom__u7d42u_u308fu_u3063u_u3066u_u3088u_u3044u) THEN SKIP ELSE STOP)\"",
		},
	}
	spec := "@startuml\nstate \"s\" as s\n[*] --> s\ns --> s : go\n@enduml\n"

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			var sb strings.Builder

			// Execute
			err := target.CompileDiagrams(&sb, obligationir.IRRefinementModeStableFailure, read(t, spec), read(t, tc.Impl), target.NameIsabelle)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(sb.String(), tc.Want) {
				t.Errorf("no %q in\n%s", tc.Want, sb.String())
			}
		})
	}
}

func TestARefinementRefusesAChannelOfTwoArities(t *testing.T) {
	// Setup: go carries one field in the spec and none in the impl.
	spec := "@startuml\nstate \"s\" as s\n[*] --> s\ns --> s : go?k\nnote as channels\n  channel go : k\nend note\n@enduml\n"
	impl := "@startuml\nstate \"a\" as a\n[*] --> a\na --> a : go\n@enduml\n"

	// Execute
	_, err := obligationir.BuildRefinement(obligationir.IRRefinementModeTrace, read(t, spec), read(t, impl))

	// Assert
	if err == nil || !strings.Contains(err.Error(), "the channel go has") {
		t.Errorf("err = %v, want the channel refused", err)
	}
}

func TestARefinementProvesALemmaBothSidesNeedOnce(t *testing.T) {
	// Setup: both sides start from an empty map.
	src := "@startuml\nstate \"a\" as a\na : m ; m\n[*] --> a : @empty(m')\n@enduml\n"
	var sb strings.Builder

	// Execute
	err := target.CompileDiagrams(&sb, obligationir.IRRefinementModeTrace, read(t, src), read(t, src), target.NameIsabelle)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(sb.String(), "lemma csdf_lemma_"); n != 1 {
		t.Errorf("%d lemmas, want the one of @empty once\n%s", n, sb.String())
	}
}
