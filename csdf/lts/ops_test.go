package lts

import (
	"strings"
	"testing"
)

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
			mustRename(t, seq("a", "b"), func(l string) []string {
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
			"one-to-many renaming offers every image",
			mustRename(t, seq("a"), func(string) []string { return []string{"c", "d"} }),
			fork("c", "d"),
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

// fork offers each of labels once, then stops.
func fork(labels ...string) *LTS {
	l := New(0)
	for _, a := range labels {
		l.Add(0, a, 1)
	}
	return l
}

func mustRename(t *testing.T, l *LTS, to func(string) []string) *LTS {
	t.Helper()
	r, err := Rename(l, to)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestRenameRefusesALabelWithoutAnImage(t *testing.T) {
	// Setup
	l := seq("a", "b")

	// Execute
	_, err := Rename(l, func(label string) []string {
		if label == "a" {
			return []string{"c"}
		}
		return nil
	})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "b") {
		t.Errorf("err = %v, want one naming b, which has no image", err)
	}
}
