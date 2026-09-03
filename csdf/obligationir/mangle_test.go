package obligationir

import (
	"strings"
	"testing"
)

func TestMangle(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "an identifier is left alone", in: "availableProducts", want: "availableProducts"},
		{name: "an event with parentheses", in: "choose(product)", want: "choose_u28uproduct_u29u"},
		{name: "a space", in: "pay now", want: "pay_u20unow"},
		{name: "a hyphen in an id", in: "vm-idle", want: "vm_u2duidle"},
		{name: "an underscore doubles", in: "a_b", want: "a__b"},
		{name: "a leading digit is left alone; the prefix supplies the letter", in: "1st", want: "1st"},
		{name: "a non-ASCII letter", in: "商品", want: "_u5546u_u54c1u"},
		{name: "the empty name", in: "", want: ""},
		// Isabelle rejects a name ending in "_", so a trailing one is escaped
		// rather than doubled.
		{name: "an underscore inside doubles", in: "s0_s1", want: "s0__s1"},
		{name: "a trailing underscore is escaped", in: "a_", want: "a_u5fu"},
		{name: "only the last underscore is escaped", in: "a__", want: "a___u5fu"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Mangle(tt.in); got != tt.want {
				t.Errorf("Mangle(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestMangleIsInjective pins that no two distinct names share an identifier: a
// collision would silently identify two events or two variables.
func TestMangleIsInjective(t *testing.T) {
	names := []string{
		"", "_", "__", "u_", "u", "a", "a_", "_a", "1st", "end", "end_",
		"a b", "a_b", "a__b", "a-b", "choose(product)", "choose_u28uproduct_u29u",
		"商品", "a_u5fu", "s0_s1", "s0__s1",
	}
	seen := make(map[string]string, len(names))
	for _, n := range names {
		m := Mangle(n)
		if prev, ok := seen[m]; ok {
			t.Errorf("Mangle(%q) = Mangle(%q) = %q", prev, n, m)
		}
		seen[m] = n
	}
}

// TestMangleProducesIdentifierCharacters pins the character set both provers
// accept. Mangle alone may still start with a digit, which is why every emitted
// identifier carries a category prefix; TestVarName covers one such prefix.
func TestMangleProducesIdentifierCharacters(t *testing.T) {
	for _, in := range []string{"", "1st", "choose(product)", "vm-idle", "商品", "end", "\n", "a_", "a__", "_"} {
		got := Mangle(in)
		for _, r := range got {
			isLetter := ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
			if !isLetter && !('0' <= r && r <= '9') && r != '_' {
				t.Errorf("Mangle(%q) = %q contains %q", in, got, r)
				break
			}
		}
		// Isabelle rejects a name ending in "_" outright ("Bad name").
		if strings.HasSuffix(got, "_") {
			t.Errorf("Mangle(%q) = %q ends in an underscore", in, got)
		}
	}
}

// TestVarName pins that a state variable never reaches the theory under the name
// the diagram gave it. A diagram may call a variable "step", "init" or "and", and
// an unprefixed binder of that name shadows the constant being defined or is a
// keyword outright.
func TestVarName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "an ordinary name", in: "availableProducts", want: "v_availableProducts"},
		{name: "a name the generator declares itself", in: "step", want: "v_step"},
		{name: "an Isabelle keyword", in: "and", want: "v_and"},
		{name: "a Lean keyword", in: "def", want: "v_def"},
		{name: "a leading digit", in: "1st", want: "v_1st"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := VarName(tt.in); got != tt.want {
				t.Errorf("VarName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsMangled(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "a name that survives verbatim", in: "availableProducts", want: false},
		{name: "a name that has to be encoded", in: "choose(product)", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMangled(tt.in); got != tt.want {
				t.Errorf("IsMangled(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
