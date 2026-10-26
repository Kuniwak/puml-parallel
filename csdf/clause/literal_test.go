package clause

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseLiteralIsTheCanonicalText(t *testing.T) {
	tests := map[string]struct {
		text string
		want string
	}{
		"an integer":                                  {text: "42", want: "42"},
		"a negative integer":                          {text: "-7", want: "-7"},
		"an integer beyond 64 bits":                   {text: "123456789012345678901234567890", want: "123456789012345678901234567890"},
		"a float with a point":                        {text: "1.0", want: "1.0"},
		"a float with an exponent":                    {text: "1e2", want: "100.0"},
		"a float is written shortest":                 {text: "0.10", want: "0.1"},
		"a large float keeps its exponent":            {text: "1e21", want: "1e+21"},
		"a float below the smallest is zero":          {text: "1e-400", want: "0.0"},
		"minus zero is zero":                          {text: "-0", want: "0"},
		"minus zero as a float is zero":               {text: "-0.0", want: "0.0"},
		"minus zero with an exponent is zero":         {text: "-0e5", want: "0.0"},
		"a negative float below the smallest is zero": {text: "-1e-400", want: "0.0"},
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

func TestCanonicalValueReadsNegativeZeroAsZero(t *testing.T) {
	tests := map[string]struct {
		value any
		want  string
	}{
		"a float64":        {value: math.Copysign(0, -1), want: "0.0"},
		"a number as text": {value: json.Number("-0.0"), want: "0.0"},
		"in an array":      {value: []any{math.Copysign(0, -1)}, want: "[0.0]"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := CanonicalValue(tt.value)

			// Assert
			if err != nil || got != tt.want {
				t.Fatalf("CanonicalValue() = %q, %v, want %q", got, err, tt.want)
			}
		})
	}
}

func TestUnsignedZerosWritesEveryNegativeZeroAsZero(t *testing.T) {
	tests := map[string]struct {
		value any
		want  any
	}{
		"a float64":                    {value: math.Copysign(0, -1), want: 0.0},
		"an integer as text":           {value: json.Number("-0"), want: json.Number("0")},
		"a float as text":              {value: json.Number("-0.0"), want: json.Number("0.0")},
		"a float with an exponent":     {value: json.Number("-0e5"), want: json.Number("0.0")},
		"another number kept as typed": {value: json.Number("-1.50"), want: json.Number("-1.50")},
		"in an array":                  {value: []any{json.Number("-0.0")}, want: []any{json.Number("0.0")}},
		"in an object":                 {value: map[string]any{"k": json.Number("-0")}, want: map[string]any{"k": json.Number("0")}},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			got := UnsignedZeros(tt.value)

			// Assert
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Fatalf("UnsignedZeros() mismatch (-want +got):\n%s", diff)
			}
			if f, ok := got.(float64); ok && math.Signbit(f) {
				t.Fatalf("UnsignedZeros() = -0.0")
			}
		})
	}
}

func TestCanonicalValueRefusesAFloatThatIsNotFinite(t *testing.T) {
	tests := map[string]float64{"NaN": math.NaN(), "infinity": math.Inf(1), "negative infinity": math.Inf(-1)}
	for name, v := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := CanonicalValue(v)

			// Assert
			assertErrorContains(t, err, "no finite float")
		})
	}
}
