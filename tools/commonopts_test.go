package tools

import (
	"bytes"
	"flag"
	"log/slog"
	"reflect"
	"testing"
)

func TestParseCommonOptions(t *testing.T) {
	type testCase struct {
		Args     []string
		Expected *CommonOptions
	}

	testCases := map[string]testCase{
		"empty (boundary value)": {
			Args:     []string{},
			Expected: NewCommonOptionsDefault(),
		},
		"--debug (representative value)": {
			Args: []string{"--debug"},
			Expected: &CommonOptions{
				LogLevel: slog.LevelDebug,
			},
		},
		"--silent (representative value)": {
			Args: []string{"--silent"},
			Expected: &CommonOptions{
				LogLevel: slog.LevelError,
			},
		},
		"--silent --debug (representative value)": {
			Args: []string{"--debug", "--silent"},
			Expected: &CommonOptions{
				LogLevel: slog.LevelDebug,
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			flags := flag.NewFlagSet("test", flag.ContinueOnError)
			buf := &bytes.Buffer{}
			flags.SetOutput(buf)

			// Act
			var commonRawOpts CommonRawOptions
			DeclareCommonOptions(flags, &commonRawOpts)

			if err := flags.Parse(testCase.Args); err != nil {
				t.Log(buf.String())
				t.Fatalf("want nil, got %#v", err)
			}

			// Assert
			commonOpts, err := ValidateCommonOptions(&commonRawOpts)
			if err != nil {
				t.Log(buf.String())
				t.Fatalf("want nil, got %#v", err)
			}

			if !reflect.DeepEqual(testCase.Expected, commonOpts) {
				t.Error(testCase.Expected, commonOpts)
			}
		})
	}
}
