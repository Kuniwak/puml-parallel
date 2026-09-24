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
		Cond  Cond
		Posts []csdf.Predicate
	}
	type testCase struct {
		A, B walk
	}

	testCases := map[string]testCase{
		"one guard holding a control character, and two guards": {
			A: walk{State: "D", Cond: Cond{{Pred: "a\x00b"}}},
			B: walk{State: "D", Cond: Cond{{Pred: "a"}, {Pred: "b"}}},
		},
		"a guard, and its negation": {
			A: walk{State: "D", Cond: Cond{{Pred: "g"}}},
			B: walk{State: "D", Cond: Cond{{Pred: "g", Negated: true}}},
		},
		"a guard, and a postcondition spelled the same": {
			A: walk{State: "D", Cond: Cond{{Pred: "p"}}},
			B: walk{State: "D", Posts: []csdf.Predicate{"p"}},
		},
		"one postcondition holding a control character, and two postconditions": {
			A: walk{State: "D", Posts: []csdf.Predicate{"p\x00q"}},
			B: walk{State: "D", Posts: []csdf.Predicate{"p", "q"}},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			a := walkKey(testCase.A.State, testCase.A.Cond, testCase.A.Posts)
			b := walkKey(testCase.B.State, testCase.B.Cond, testCase.B.Posts)

			// Assert
			if a == b {
				t.Errorf("want different keys, got %q for both", a)
			}
		})
	}
}
