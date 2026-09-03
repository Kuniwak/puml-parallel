package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
)

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("cannot write %s: %v", path, err)
	}
	return path
}

func TestValidateArgsAsTwoFilePathsReadsBothFiles(t *testing.T) {
	// Arrange
	first := writeTempFile(t, "a.puml", "first")
	second := writeTempFile(t, "b.puml", "second")
	spy := cli.SpyProcInout()

	// Act
	bs, err := ValidateArgsAsTwoFilePaths([]string{first, second}, spy.New())

	// Assert
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	if string(bs[0]) != "first" || string(bs[1]) != "second" {
		t.Errorf("want [first second], got [%s %s]", bs[0], bs[1])
	}
}

func TestValidateArgsAsTwoFilePathsReadsStdinForDash(t *testing.T) {
	// Arrange: "-" stands for standard input, in either position.
	file := writeTempFile(t, "a.puml", "from file")

	for name, args := range map[string][]string{
		"first is stdin":  {"-", file},
		"second is stdin": {file, "-"},
	} {
		t.Run(name, func(t *testing.T) {
			spy := cli.SpyProcInout()
			spy.Stdin = strings.NewReader("from stdin")

			// Act
			bs, err := ValidateArgsAsTwoFilePaths(args, spy.New())

			// Assert
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			wantFirst, wantSecond := "from stdin", "from file"
			if args[1] == "-" {
				wantFirst, wantSecond = "from file", "from stdin"
			}
			if string(bs[0]) != wantFirst || string(bs[1]) != wantSecond {
				t.Errorf("want [%s %s], got [%s %s]", wantFirst, wantSecond, bs[0], bs[1])
			}
		})
	}
}

func TestValidateArgsAsTwoFilePathsRejectsBadArgumentCounts(t *testing.T) {
	// Arrange: unlike the single-file tools there is no "read everything from
	// stdin" fallback, because two diagrams cannot be told apart in one stream.
	file := writeTempFile(t, "a.puml", "first")

	for name, args := range map[string][]string{
		"none":      {},
		"one":       {file},
		"three":     {file, file, file},
		"two stdin": {"-", "-"},
	} {
		t.Run(name, func(t *testing.T) {
			spy := cli.SpyProcInout()
			spy.Stdin = strings.NewReader("from stdin")

			// Act
			if _, err := ValidateArgsAsTwoFilePaths(args, spy.New()); err == nil {
				t.Error("want an error, got nil")
			}
		})
	}
}
