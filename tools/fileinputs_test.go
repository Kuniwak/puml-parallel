package tools

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
)

func TestValidateArgsAsFileInputs(t *testing.T) {
	type testCase struct {
		Args      []string
		WantNames []string // nil when an error is wanted.
	}

	testCases := map[string]testCase{
		"no argument is standard input (lower boundary value)": {
			Args:      nil,
			WantNames: []string{"-"},
		},
		"a file and standard input (representative value)": {
			Args:      []string{"commonopts.go", "-"},
			WantNames: []string{"commonopts.go", "-"},
		},
		`two "-" arguments are refused, because the second would read an exhausted stream (representative value)`: {
			Args: []string{"-", "-"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			spy := cli.SpyProcInout()
			spy.Stdin = strings.NewReader("@startuml\n@enduml\n")

			// Act
			inputs, err := ValidateArgsAsFileInputs(testCase.Args, spy.New())

			// Assert
			if testCase.WantNames == nil {
				if err == nil {
					t.Fatalf("want an error, got %v", inputs)
				}
				return
			}
			if err != nil {
				t.Fatalf("want nil, got %#v", err)
			}
			if len(inputs) != len(testCase.WantNames) {
				t.Fatalf("want %d inputs, got %d", len(testCase.WantNames), len(inputs))
			}
			for i, want := range testCase.WantNames {
				if inputs[i].Name != want {
					t.Errorf("want %s, got %s", want, inputs[i].Name)
				}
				if len(inputs[i].Bytes) == 0 {
					t.Errorf("want the bytes of %s, got none", want)
				}
			}
		})
	}
}
