package ident

import (
	"regexp"
	"strings"
	"testing"
)

func TestIs(t *testing.T) {
	tests := map[string]struct {
		s    string
		want bool
	}{
		"ASCII letters":           {s: "abc", want: true},
		"an underscore":           {s: "a_b", want: true},
		"digits":                  {s: "x1", want: true},
		"a digit first":           {s: "1st", want: true},
		"Japanese":                {s: "在庫", want: true},
		"the empty string":        {s: "", want: false},
		"a hyphen":                {s: "a-b", want: false},
		"a space":                 {s: "a b", want: false},
		"a prime":                 {s: "x'", want: false},
		"a dot":                   {s: "m.f", want: false},
		"a Japanese full stop":    {s: "在庫。", want: false},
		"a fullwidth space":       {s: "在　庫", want: false},
		"a combining mark alone":  {s: "́", want: false},
		"an ideographic numeral":  {s: "〇", want: true},
		"an emoji":                {s: "😀", want: false},
		"a hyphen at the end":     {s: "x-", want: false},
		"an underscore alone":     {s: "_", want: true},
		"a digit alone":           {s: "1", want: true},
		"fullwidth Latin letters": {s: "ＡＢ", want: true},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got := Is(tt.s)

			// Assert
			if got != tt.want {
				t.Errorf("Is(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestPatternAgreesWithIs(t *testing.T) {
	// Setup
	re := regexp.MustCompile(`^` + Pattern + `$`)
	for _, s := range []string{"abc", "a_b", "1st", "在庫", "", "a-b", "a b", "x'", "〇", "😀", "ＡＢ"} {
		t.Run(s, func(t *testing.T) {
			// Execute
			got := re.MatchString(s)

			// Assert
			if want := Is(s); got != want {
				t.Errorf("Pattern matches %q = %v, Is = %v", s, got, want)
			}
		})
	}
}

func TestCheckRefusesWhatNamesNothing(t *testing.T) {
	for s, want := range map[string]string{
		"null": "null is a reserved word",
		"9a":   "does not start with a digit, as 9a",
		"a-b":  "is not an identifier",
	} {
		if err := Check(s); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Check(%q) = %v, want an error containing %q", s, err, want)
		}
	}
	if err := Check("s0"); err != nil {
		t.Errorf("Check(s0) = %v, want nil", err)
	}
}
