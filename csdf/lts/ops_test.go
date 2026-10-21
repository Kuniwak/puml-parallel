package lts

import "testing"

func seq(labels ...string) *LTS {
	l := New(0)
	for i, a := range labels {
		l.Add(i, a, i+1)
	}
	return l
}

func TestOperatorsAgreeWithTheirCSPLaws(t *testing.T) {
	cases := []struct {
		name      string
		got, want *LTS
	}{
		{
			"hiding a turns a → b → STOP into b → STOP",
			Hide(seq("a", "b"), func(l string) bool { return l == "a" }),
			seq("b"),
		},
		{
			"renaming a to c",
			Rename(seq("a", "b"), func(l string) []string {
				if l == "a" {
					return []string{"c"}
				}
				return []string{l}
			}),
			seq("c", "b"),
		},
		{
			"a → b → STOP [|{b}|] b → STOP is a → b → STOP",
			Parallel([]*LTS{seq("a", "b"), seq("b")}, []func(string) bool{
				func(l string) bool { return l == "a" || l == "b" },
				func(l string) bool { return l == "b" },
			}),
			seq("a", "b"),
		},
		{
			"a → STOP [|{a, b}|] b → STOP deadlocks",
			Parallel([]*LTS{seq("a"), seq("b")}, []func(string) bool{
				func(l string) bool { return l == "a" || l == "b" },
				func(l string) bool { return l == "a" || l == "b" },
			}),
			New(0),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			equal := StableFailuresEqual(c.got, c.want)

			// Assert
			if !equal {
				t.Errorf("not stable-failures equivalent: %#v", c.got.Trans)
			}
		})
	}
}

func TestOneToManyRenamingOffersEveryImage(t *testing.T) {
	// Setup
	want := New(0)
	want.Add(0, "c", 1)
	want.Add(0, "d", 1)

	// Execute
	got := Rename(seq("a"), func(string) []string { return []string{"c", "d"} })

	// Assert
	if !StableFailuresEqual(got, want) {
		t.Errorf("not stable-failures equivalent: %#v", got.Trans)
	}
}
