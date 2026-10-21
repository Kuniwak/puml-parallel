package clause

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNormalize(t *testing.T) {
	tests := map[string]struct {
		f    Formula
		want And
	}{
		"flattens nested conjunctions": {
			f:    and(and(atom(Text("a")), and(atom(Text("b")))), atom(Text("c"))),
			want: and(atom(Text("a")), atom(Text("b")), atom(Text("c"))),
		},
		"a lone conjunct is a conjunction": {
			f:    atom(Text("a")),
			want: and(atom(Text("a"))),
		},
		"merges adjacent text and drops empty text": {
			f:    atom(Text("a"), Text(""), Text("b"), hole(pre("x")), Text("")),
			want: and(atom(Text("ab"), hole(pre("x")))),
		},
		"normalizes an ∃ body": {
			f:    Exists{Hint: "v", Body: and(and(atom(Text("a"))))},
			want: and(Exists{Hint: "v", Body: and(atom(Text("a")))}),
		},
		"no arguments are no arguments": {
			f:    atom(hole(App{Fn: "f", Args: []Term{}})),
			want: and(atom(hole(App{Fn: "f"}))),
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got := Normalize(tt.f)

			// Assert
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Normalize() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAlphaEqual(t *testing.T) {
	tests := map[string]struct {
		a, b Formula
		want bool
	}{
		"hints carry no meaning": {
			a:    Exists{Hint: "v", Body: and(Builtin{Op: OpEq, Args: []Term{Bound{0}, pre("x")}})},
			b:    Exists{Hint: "w", Body: and(Builtin{Op: OpEq, Args: []Term{Bound{0}, pre("x")}})},
			want: true,
		},
		"up to the normal form": {
			a:    and(and(atom(Text("a"), Text("b"))), atom(hole(App{Fn: "f"}))),
			b:    and(atom(Text("ab")), atom(hole(App{Fn: "f", Args: []Term{}}))),
			want: true,
		},
		"a bound variable is not a free one": {
			a:    Exists{Hint: "x", Body: and(Builtin{Op: OpEq, Args: []Term{Bound{0}, pre("x")}})},
			b:    Exists{Hint: "x", Body: and(Builtin{Op: OpEq, Args: []Term{pre("x"), pre("x")}})},
			want: false,
		},
		"free names matter": {
			a:    atom(hole(pre("x"))),
			b:    atom(hole(pre("y"))),
			want: false,
		},
		"so do primes": {
			a:    atom(hole(pre("x"))),
			b:    atom(hole(post("x"))),
			want: false,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got := AlphaEqual(tt.a, tt.b)

			// Assert
			if got != tt.want {
				t.Errorf("AlphaEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}
