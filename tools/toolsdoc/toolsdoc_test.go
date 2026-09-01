package toolsdoc

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/toolsdoc/toolsdoctest"
	"github.com/google/go-cmp/cmp"
)

func stubEntries() []Entry {
	return []Entry{
		{Name: "alpha", Summary: "the first tool.", Run: toolsdoctest.StubHelp("Usage: alpha\n")},
		{Name: "beta", Summary: "the second tool.", Run: toolsdoctest.StubHelp("Usage: beta\n"), Subs: []tools.Subcommand{
			{Name: "one", Description: "the first subcommand", CommandFunc: toolsdoctest.StubHelp("Usage: beta one\n")},
		}},
		{Name: "gamma", Summary: "the hidden tool.", Hidden: true, Run: toolsdoctest.StubHelp("Usage: gamma\n")},
	}
}

func TestSelect(t *testing.T) {
	type testCase struct {
		Names    []string
		Expected []string
	}

	testCases := map[string]testCase{
		"no names selects every non-hidden entry (lower boundary value)": {
			Names:    nil,
			Expected: []string{"alpha", "beta"},
		},
		"one name (lower boundary value)": {
			Names:    []string{"beta"},
			Expected: []string{"beta"},
		},
		"names are returned in catalog order (representative value)": {
			Names:    []string{"beta", "alpha"},
			Expected: []string{"alpha", "beta"},
		},
		"a named hidden entry is selected (representative value)": {
			Names:    []string{"gamma"},
			Expected: []string{"gamma"},
		},
		"an unknown name selects nothing (representative value)": {
			Names:    []string{"delta"},
			Expected: []string{},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			entries := stubEntries()

			// Act
			selected := Select(entries, testCase.Names)

			// Assert
			if !reflect.DeepEqual(testCase.Expected, Names(selected)) {
				t.Error(cmp.Diff(testCase.Expected, Names(selected)))
			}
		})
	}
}

func TestLookup(t *testing.T) {
	type testCase struct {
		Name        string
		ExpectedOK  bool
		ExpectedSum string
	}

	testCases := map[string]testCase{
		"a known entry (representative value)": {
			Name:        "alpha",
			ExpectedOK:  true,
			ExpectedSum: "the first tool.",
		},
		"a hidden entry is still found (representative value)": {
			Name:        "gamma",
			ExpectedOK:  true,
			ExpectedSum: "the hidden tool.",
		},
		"an unknown entry (representative value)": {
			Name:       "delta",
			ExpectedOK: false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			entries := stubEntries()

			// Act
			entry, ok := Lookup(entries, testCase.Name)

			// Assert
			if ok != testCase.ExpectedOK {
				t.Errorf("want %t, got %t", testCase.ExpectedOK, ok)
			}
			if entry.Summary != testCase.ExpectedSum {
				t.Errorf("want %q, got %q", testCase.ExpectedSum, entry.Summary)
			}
		})
	}
}

func TestWriteSummaries(t *testing.T) {
	// Arrange
	entries := Select(stubEntries(), nil)
	var got strings.Builder

	// Act
	err := WriteSummaries(&got, entries)

	// Assert
	if err != nil {
		t.Errorf("want nil, got %#v", err)
	}
	want := "alpha: the first tool.\nbeta: the second tool.\n"
	if want != got.String() {
		t.Error(cmp.Diff(want, got.String()))
	}
}

func TestWriteMarkdown(t *testing.T) {
	// Arrange
	entries := Select(stubEntries(), nil)
	var got strings.Builder

	// Act
	err := WriteMarkdown(&got, cli.NewEnvFunc(nil), entries)

	// Assert
	if err != nil {
		t.Errorf("want nil, got %#v", err)
	}
	want := "# alpha\n\n```\nUsage: alpha\n```\n\n" +
		"# beta\n\n```\nUsage: beta\n```\n\n" +
		"## beta one\n\n```\nUsage: beta one\n```\n\n"
	if want != got.String() {
		t.Error(cmp.Diff(want, got.String()))
	}
}

func TestWriteMarkdownFencesBackticks(t *testing.T) {
	// Arrange
	entries := []Entry{{Name: "alpha", Run: toolsdoctest.StubHelp("see ``` here\n")}}
	var got strings.Builder

	// Act
	err := WriteMarkdown(&got, cli.NewEnvFunc(nil), entries)

	// Assert
	if err != nil {
		t.Errorf("want nil, got %#v", err)
	}
	want := "# alpha\n\n````\nsee ``` here\n````\n\n"
	if want != got.String() {
		t.Error(cmp.Diff(want, got.String()))
	}
}

func TestWriteMarkdownTerminatesUnterminatedHelp(t *testing.T) {
	// Arrange
	entries := []Entry{{Name: "alpha", Run: toolsdoctest.StubHelp("Usage: alpha")}}
	var got strings.Builder

	// Act
	err := WriteMarkdown(&got, cli.NewEnvFunc(nil), entries)

	// Assert
	if err != nil {
		t.Errorf("want nil, got %#v", err)
	}
	want := "# alpha\n\n```\nUsage: alpha\n```\n\n"
	if want != got.String() {
		t.Error(cmp.Diff(want, got.String()))
	}
}

func TestWriteMarkdownFailingHelp(t *testing.T) {
	// Arrange
	entries := []Entry{{Name: "alpha", Run: toolsdoctest.StubFailingHelp()}}
	var got strings.Builder

	// Act
	err := WriteMarkdown(&got, cli.NewEnvFunc(nil), entries)

	// Assert
	if err == nil {
		t.Error("want not nil, got nil")
	}
	if got.String() != "" {
		t.Errorf("want no partial output, got %q", got.String())
	}
}

func TestNames(t *testing.T) {
	// Arrange
	entries := stubEntries()

	// Act
	names := Names(entries)

	// Assert
	want := []string{"alpha", "beta", "gamma"}
	if !reflect.DeepEqual(want, names) {
		t.Error(cmp.Diff(want, names))
	}
}

func TestFence(t *testing.T) {
	type testCase struct {
		Content  string
		Expected string
	}

	testCases := map[string]testCase{
		"empty content (lower boundary value)": {
			Content:  "",
			Expected: "```",
		},
		"no backtick (representative value)": {
			Content:  "Usage: alpha\n",
			Expected: "```",
		},
		"two backticks still fit in three (upper boundary value)": {
			Content:  "``\n",
			Expected: "```",
		},
		"three backticks need four (lower boundary value)": {
			Content:  "```\n",
			Expected: "````",
		},
		"four backticks need five (representative value)": {
			Content:  "````\n",
			Expected: "`````",
		},
		"the longest run wins (representative value)": {
			Content:  "` a ```` b ``\n",
			Expected: "`````",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange & Act
			fence := Fence(testCase.Content)

			// Assert
			if testCase.Expected != fence {
				t.Error(cmp.Diff(testCase.Expected, fence))
			}
		})
	}
}
