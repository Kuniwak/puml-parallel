package clause

import "testing"

func TestFresh(t *testing.T) {
	tests := map[string]struct {
		avoid []string
		hint  string
		want  string
	}{
		"the hint when it is free":       {avoid: []string{"x"}, hint: "v", want: "v"},
		"a number after a taken hint":    {avoid: []string{"v"}, hint: "v", want: "v2"},
		"the least number not taken":     {avoid: []string{"v", "v2", "v3"}, hint: "v", want: "v4"},
		"after a hint ending in a digit": {avoid: []string{"x1"}, hint: "x1", want: "x1_2"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			avoid := map[string]bool{}
			for _, a := range tt.avoid {
				avoid[a] = true
			}

			// Execute
			got := Fresh(avoid, tt.hint)

			// Assert
			if got != tt.want {
				t.Errorf("Fresh(%v, %q) = %q, want %q", tt.avoid, tt.hint, got, tt.want)
			}
		})
	}
}
