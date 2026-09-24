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

func TestReadFileInputs(t *testing.T) {
	type testCase struct {
		Args      []string
		WantNames []string // nil when an error is wanted.
	}

	testCases := map[string]testCase{
		"no argument (lower boundary value)": {
			Args:      nil,
			WantNames: []string{},
		},
		"two files (representative value)": {
			Args:      []string{"commonopts.go", "command.go"},
			WantNames: []string{"commonopts.go", "command.go"},
		},
		`"-" is refused, because these inputs cannot come from standard input (representative value)`: {
			Args: []string{"-"},
		},
		"a missing file (representative value)": {
			Args: []string{"missing.go"},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Act
			inputs, err := ReadFileInputs(testCase.Args)

			// Assert
			if testCase.WantNames == nil {
				if err == nil {
					t.Errorf("want an error, got %#v", inputs)
				}
				return
			}
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			names := make([]string, 0, len(inputs))
			for _, input := range inputs {
				names = append(names, input.Name)
			}
			if len(names) != len(testCase.WantNames) {
				t.Fatalf("want %v, got %v", testCase.WantNames, names)
			}
			for i := range names {
				if names[i] != testCase.WantNames[i] {
					t.Errorf("want %v, got %v", testCase.WantNames, names)
				}
			}
		})
	}
}

func TestReadFileOrStdin(t *testing.T) {
	type testCase struct {
		Path    string
		WantErr bool
	}

	testCases := map[string]testCase{
		`"-" is standard input (representative value)`: {Path: "-"},
		"a file (representative value)":                {Path: "commonopts.go"},
		"a missing file (representative value)":        {Path: "missing.go", WantErr: true},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			spy := cli.SpyProcInout()
			spy.Stdin = strings.NewReader("@startuml\n@enduml\n")

			// Act
			bs, err := ReadFileOrStdin(testCase.Path, spy.New())

			// Assert
			if testCase.WantErr {
				if err == nil {
					t.Fatalf("want an error, got %q", bs)
				}
				return
			}
			if err != nil {
				t.Fatalf("want nil, got %#v", err)
			}
			if len(bs) == 0 {
				t.Errorf("want the bytes of %s, got none", testCase.Path)
			}
		})
	}
}
