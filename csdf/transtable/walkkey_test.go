package transtable

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Two walks that share a key are taken for one, and the second is dropped, so
// walks that differ must never share one.
func TestWalkKeyTellsWalksApart(t *testing.T) {
	type walk struct {
		State csdf.StateID
		Steps []tauStep
	}
	type testCase struct {
		A, B walk
	}

	testCases := map[string]testCase{
		"one guard holding a control character, and two guards": {
			A: walk{State: "D", Steps: []tauStep{{guard: "a\x00b"}}},
			B: walk{State: "D", Steps: []tauStep{{guard: "a"}, {guard: "b"}}},
		},
		"a guard, and a postcondition spelled the same": {
			A: walk{State: "D", Steps: []tauStep{{guard: "p"}}},
			B: walk{State: "D", Steps: []tauStep{{post: "p"}}},
		},
		"one postcondition holding a control character, and two postconditions": {
			A: walk{State: "D", Steps: []tauStep{{post: "p\x00q"}}},
			B: walk{State: "D", Steps: []tauStep{{post: "p"}, {post: "q"}}},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			a := walkKey(testCase.A.State, testCase.A.Steps)
			b := walkKey(testCase.B.State, testCase.B.Steps)

			// Assert
			if a == b {
				t.Errorf("want different keys, got %q for both", a)
			}
		})
	}
}
