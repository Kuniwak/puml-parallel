package coreio

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/v9/csdf/core"
	"github.com/Kuniwak/puml-parallel/v9/csdf/types"
)

// gated is a diagram with an edge whose clauses are generated builtins, its
// guard placing the door at shut, the states given.
func gated(t *testing.T, shut string) *core.Diagram {
	return &core.Diagram{
		States:   []string{"G"},
		Vars:     map[string][]string{"G": {"doors"}},
		Types:    map[string]types.Type{"doors": types.MapOf(types.Named("Loc"), types.Named("Door"))},
		Channels: map[string][]types.Type{"OPENS": {types.Named("Loc")}},
		Init:     "G",
		InitPost: mustParse(t, "@empty(doors')"),
		Edges: []core.Edge{{Src: "G", Dst: "G", Event: core.Event{Channel: "OPENS", Fields: []core.Field{{Name: "loc"}}},
			Guard: mustParse(t, "@in(loc, doors) ∧ @at(doors[loc], "+shut+") ∧ $loc は正面"),
			Post:  mustParse(t, "@frame(doors', doors, loc) ∧ @at(doors'[loc], wide)")}},
	}
}

func TestWriteGlossedGlossesTheBuiltinsAndReadsBackTheSame(t *testing.T) {
	cases := []struct {
		name, shut, templates string
		// want is the line above the edge, or "" for none.
		want string
	}{
		{"a gloss of the builtins with a template", "shut",
			`{{define "in"}}{{index .Args 0}} がある{{end}}{{define "at"}}{{index .Args 0}} は {{.State}}{{end}}`,
			"' loc がある ∧ doors[loc] は shut ∧ doors'[loc] は wide\n"},
		{"a gloss of each state of @at", "shut, ajar",
			`{{define "at"}}{{index .Args 0}} は {{range $i, $s := .States}}{{if $i}} か {{end}}{{$s}}{{end}}{{end}}`,
			"' doors[loc] は shut か ajar ∧ doors'[loc] は wide\n"},
		{"a gloss holding a quote", "shut",
			`{{define "empty"}}x{{end}}{{define "frame"}}'{{index .Args 0}}' は保たれる{{end}}`,
			"' 'doors'' は保たれる\n"},
		{"no template that applies", "shut", `{{define "ne"}}{{index .Args 0}} ≠ {{index .Args 1}}{{end}}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			templates, err := LoadTemplates(c.templates)
			if err != nil {
				t.Fatal(err)
			}
			d := gated(t, c.shut)

			// Execute
			got, err := WriteGlossed(d, templates)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got, c.want+"G --> G : OPENS?loc") {
				t.Errorf("WriteGlossed wrote not %q above the edge:\n%s", c.want, got)
			}
			if c.want == "" && strings.Contains(got, "\n' ") {
				t.Errorf("WriteGlossed wrote a gloss where no template applies:\n%s", got)
			}
			back, err := Read(got)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(d, back, sameClauses); diff != "" {
				t.Errorf("Read(WriteGlossed(d)) (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWriteGlossedRefusesTheOneStateOfAnAtOverSeveral(t *testing.T) {
	// Setup
	templates, err := LoadTemplates(`{{define "at"}}{{index .Args 0}} は {{.State}}{{end}}`)
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	_, err = WriteGlossed(gated(t, "shut, ajar"), templates)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "@at over 2 states has no one state; read .States") {
		t.Errorf("WriteGlossed error = %v, want one saying @at over 2 states has no one state", err)
	}
}

func TestLoadTemplatesRefuses(t *testing.T) {
	cases := []struct {
		name, text, want string
	}{
		{"a template named after csdfpromote's clauses", `{{define "inDom"}}x{{end}}`, "coreio.LoadTemplates: the template inDom is not named after a builtin word"},
		{"a template that cannot render", `{{define "at"}}{{.Nope}}{{end}}`, "coreio.LoadTemplates: the template at cannot render: "},
		{"a template writing a line break", "{{define \"at\"}}a\nb{{end}}", "coreio.LoadTemplates: the template at writes a line break, which would end its line comment"},
		{"a file that does not parse", `{{define "at"}}{{end`, "coreio.LoadTemplates: the templates do not parse: "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			_, err := LoadTemplates(c.text)

			// Assert
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("LoadTemplates: got %v, want an error containing %q", err, c.want)
			}
		})
	}
}
