package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lint"
	"github.com/google/go-cmp/cmp"
)

func TestWriteTSVWritesAHeaderAndOneRecordPerFinding(t *testing.T) {
	// Arrange
	findings := []lint.Finding{{
		File:      "a.puml",
		StartLine: 3,
		EndLine:   3,
		RuleID:    "syntax-error",
		Severity:  lint.SeverityError,
		Message:   "unexpected syntax",
	}}
	want := strings.Join([]string{
		"file\tstart_line\tend_line\trule\tseverity\tmessage",
		"a.puml\t3\t3\tsyntax-error\tERROR\tunexpected syntax",
		"",
	}, "\n")

	// Act
	var sb strings.Builder
	if err := lint.WriteTSV(&sb, findings); err != nil {
		t.Fatal(err)
	}

	// Assert
	if diff := cmp.Diff(want, sb.String()); diff != "" {
		t.Error(diff)
	}
}
