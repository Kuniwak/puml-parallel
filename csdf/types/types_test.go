package types

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := map[string]struct {
		text    string
		want    string
		wantErr string
	}{
		"a named type":                  {text: "ID", want: "ID"},
		"a Unicode name":                {text: "会員ID", want: "会員ID"},
		"any":                           {text: "any", want: "any"},
		"a map type":                    {text: "K ⇸ V", want: "K ⇸ V"},
		"a map type written in ASCII":   {text: "K->>V", want: "K ⇸ V"},
		"a map type nests to the right": {text: "K ⇸ L ⇸ V", want: "K ⇸ L ⇸ V"},
		"a map type as a key":           {text: "(K ⇸ L) ⇸ V", want: "(K ⇸ L) ⇸ V"},
		"redundant parentheses":         {text: "((K)) ⇸ (V)", want: "K ⇸ V"},
		"the empty text":                {text: "", wantErr: "no name of a type at 0"},
		"no identifier":                 {text: "a-b", wantErr: `"-b" follows the type`},
		"an unclosed parenthesis":       {text: "(K ⇸ V", wantErr: "a ( is not closed"},
		"a missing value type":          {text: "K ⇸", wantErr: "no name of a type at 5"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := Parse(tt.text)

			// Assert
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Parse(%q) = %v, %v; want an error containing %q", tt.text, got, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.text, err)
			}
			if got.String() != tt.want {
				t.Errorf("Parse(%q) = %s, want %s", tt.text, got, tt.want)
			}
		})
	}
}

func TestKinds(t *testing.T) {
	tests := map[string]struct {
		t           Type
		isMap, any_ bool
	}{
		"a named type": {t: Named("K")},
		"any":          {t: AnyType, any_: true},
		"a map type":   {t: MapOf(Named("K"), AnyType), isMap: true},
		"no type":      {t: Type{}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			isMap, isAny := tt.t.IsMap(), tt.t.IsAny()

			// Assert
			if isMap != tt.isMap || isAny != tt.any_ {
				t.Errorf("IsMap, IsAny = %v, %v; want %v, %v", isMap, isAny, tt.isMap, tt.any_)
			}
		})
	}
}

func TestKeyAndVal(t *testing.T) {
	// Setup
	m := MapOf(Named("K"), MapOf(Named("L"), AnyType))

	// Execute
	key, val := m.Key(), m.Val()

	// Assert
	if key.String() != "K" || val.String() != "L ⇸ any" {
		t.Errorf("Key, Val = %s, %s; want K, L ⇸ any", key, val)
	}
	if !Named("K").Key().IsZero() || !Named("K").Val().IsZero() {
		t.Errorf("a named type has a key or a value type")
	}
}

func TestEqual(t *testing.T) {
	tests := map[string]struct {
		a, b Type
		want bool
	}{
		"one name":                            {a: Named("K"), b: Named("K"), want: true},
		"two names":                           {a: Named("K"), b: Named("L")},
		"any and AnyName":                     {a: AnyType, b: Named(AnyName), want: true},
		"one map type":                        {a: MapOf(Named("K"), Named("V")), b: MapOf(Named("K"), Named("V")), want: true},
		"map types of two value types":        {a: MapOf(Named("K"), Named("V")), b: MapOf(Named("K"), Named("W"))},
		"a map type and a named type":         {a: MapOf(Named("K"), Named("V")), b: Named("K ⇸ V")},
		"nested to the right and to the left": {a: MapOf(Named("K"), MapOf(Named("L"), Named("V"))), b: MapOf(MapOf(Named("K"), Named("L")), Named("V"))},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got := tt.a.Equal(tt.b)

			// Assert
			if got != tt.want {
				t.Errorf("%s.Equal(%s) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestATypeInJSONIsItsText(t *testing.T) {
	// Setup
	m := MapOf(MapOf(Named("K"), Named("L")), Named("V"))

	// Execute
	raw, err := json.Marshal([]Type{m})
	if err != nil {
		t.Fatal(err)
	}
	var back []Type
	err = json.Unmarshal(raw, &back)

	// Assert
	if err != nil || string(raw) != `["(K ⇸ L) ⇸ V"]` || !back[0].Equal(m) {
		t.Errorf("json: %s, %v, read back %v", raw, err, back)
	}
}

func TestFields(t *testing.T) {
	// Execute
	got := Fields([]Type{Named("K"), MapOf(Named("K"), Named("V"))})

	// Assert
	if want := " : K.(K ⇸ V)"; got != want {
		t.Errorf("Fields = %q, want %q", got, want)
	}
}
