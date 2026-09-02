package obligationir

import "testing"

func TestMangle(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "an identifier is left alone", in: "availableProducts", want: "availableProducts"},
		{name: "an event with parentheses", in: "choose(product)", want: "choose_u28_product_u29_"},
		{name: "a space", in: "pay now", want: "pay_u20_now"},
		{name: "a hyphen in an id", in: "vm-idle", want: "vm_u2d_idle"},
		{name: "an underscore doubles", in: "a_b", want: "a__b"},
		{name: "a leading digit", in: "1st", want: "u_1st"},
		{name: "a reserved word", in: "end", want: "end_"},
		{name: "a name ending in an underscore is not a reserved word", in: "end_", want: "end__"},
		{name: "a non-ASCII letter", in: "商品", want: "u__u5546__u54c1_"},
		{name: "the empty name", in: "", want: "u_"},
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
		"", "_", "__", "u_", "u", "a", "a_", "_a", "1st", "u_1st", "end", "end_",
		"a b", "a_b", "a__b", "a-b", "choose(product)", "choose_u28_product_u29_",
		"商品", "u__u5546__u54c1_",
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

// TestMangleProducesIdentifiers pins the character set both provers accept:
// an ASCII letter followed by ASCII letters, digits and underscores.
func TestMangleProducesIdentifiers(t *testing.T) {
	for _, in := range []string{"", "1st", "choose(product)", "vm-idle", "商品", "end", "\n"} {
		got := Mangle(in)
		if got == "" || !isASCIILetter(rune(got[0])) {
			t.Errorf("Mangle(%q) = %q, want an ASCII letter first", in, got)
			continue
		}
		for _, r := range got[1:] {
			if !isASCIILetter(r) && !('0' <= r && r <= '9') && r != '_' {
				t.Errorf("Mangle(%q) = %q contains %q", in, got, r)
				break
			}
		}
	}
}

func TestIsMangled(t *testing.T) {
	if IsMangled("availableProducts") {
		t.Error(`IsMangled("availableProducts") = true, want false`)
	}
	if !IsMangled("choose(product)") {
		t.Error(`IsMangled("choose(product)") = false, want true`)
	}
}
