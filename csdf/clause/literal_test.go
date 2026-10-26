package clause

import (
	"testing"
)

func TestParseLiteralIsTheCanonicalText(t *testing.T) {
	tests := map[string]struct {
		text string
		want string
	}{
		"an integer":                                    {text: "42", want: "42"},
		"a negative integer":                            {text: "-7", want: "-7"},
		"an integer beyond 64 bits":                     {text: "123456789012345678901234567890", want: "123456789012345678901234567890"},
		"minus zero is the integer zero":                {text: "-0", want: "0"},
		"a float with a point":                          {text: "1.0", want: "1.0"},
		"a float with an exponent":                      {text: "1e2", want: "100.0"},
		"a float is written shortest":                   {text: "0.10", want: "0.1"},
		"a large float keeps its exponent":              {text: "1e21", want: "1e+21"},
		"minus zero is a float of its own":              {text: "-0.0", want: "-0.0"},
		"a float below the smallest is zero":            {text: "1e-400", want: "0.0"},
		"a string":                                      {text: `"abc"`, want: `"abc"`},
		"a string in Unicode":                           {text: `"在庫"`, want: `"在庫"`},
		"an escape is decoded and written plain":        {text: `"\u00e9\t"`, want: `"é\t"`},
		"a surrogate pair is one character":             {text: `"\ud83d\ude00"`, want: `"😀"`},
		"a semicolon is escaped, which ends a clause":   {text: `"a;b"`, want: `"a\u003bb"`},
		"a slash is escaped, which may start a comment": {text: `"/'"`, want: `"\/'"`},
		"true":                     {text: "true", want: "true"},
		"false":                    {text: "false", want: "false"},
		"an array keeps its order": {text: "[2, 1, 2]", want: "[2,1,2]"},
		"an empty array":           {text: "[ ]", want: "[]"},
		"an object sorts its keys": {text: `{"b": 1, "a": [true]}`, want: `{"a":[true],"b":1}`},
		"an empty object":          {text: "{}", want: "{}"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := ParseLiteral(tt.text)

			// Assert
			if err != nil {
				t.Fatalf("ParseLiteral(%q) error = %v", tt.text, err)
			}
			if got.Text != tt.want {
				t.Errorf("ParseLiteral(%q) = %s, want %s", tt.text, got.Text, tt.want)
			}
		})
	}
}

func TestParseLiteralRefuses(t *testing.T) {
	tests := map[string]struct {
		text string
		want string
	}{
		"null":                        {text: "null", want: "null is no value"},
		"NaN":                         {text: "NaN", want: "expected a literal"},
		"a float beyond binary64":     {text: "1e400", want: "beyond the floats"},
		"a leading zero":              {text: "01", want: "a number does not start with 0"},
		"a plus sign":                 {text: "+1", want: "expected a literal"},
		"a point without digits":      {text: "1.", want: "expected a digit"},
		"an unterminated string":      {text: `"abc`, want: "an unterminated string"},
		"a control character":         {text: "\"a\nb\"", want: "a control character"},
		"an unknown escape":           {text: `"\q"`, want: `an unknown escape \q`},
		"a lone surrogate":            {text: `"\ud800"`, want: "a lone surrogate"},
		"a duplicate key":             {text: `{"a": 1, "a": 2}`, want: `the key "a" twice`},
		"a key that is no string":     {text: `{1: 2}`, want: "expected a string"},
		"a trailing comma":            {text: "[1, ]", want: "expected a literal"},
		"text after the literal":      {text: "1 2", want: "unexpected"},
		"a name starting with digits": {text: "1st", want: "a name does not start with a digit"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := ParseLiteral(tt.text)

			// Assert
			assertErrorContains(t, err, tt.want)
		})
	}
}
