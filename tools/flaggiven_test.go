package tools

import (
	"flag"
	"io"
	"testing"
)

func TestFlagGiven(t *testing.T) {
	type testCase struct {
		Args []string
		Want bool
	}

	testCases := map[string]testCase{
		"given":                        {Args: []string{"-mode", "b"}, Want: true},
		"given as its default":         {Args: []string{"-mode", "a"}, Want: true},
		"left at its default":          {Args: []string{}, Want: false},
		"another flag given, not this": {Args: []string{"-other"}, Want: false},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			flags := flag.NewFlagSet("test", flag.ContinueOnError)
			flags.SetOutput(io.Discard)
			flags.String("mode", "a", "")
			flags.Bool("other", false, "")
			if err := flags.Parse(testCase.Args); err != nil {
				t.Fatalf("want nil, got %v", err)
			}

			// Act
			got := FlagGiven(flags, "mode")

			// Assert
			if got != testCase.Want {
				t.Errorf("want %t, got %t", testCase.Want, got)
			}
		})
	}
}
