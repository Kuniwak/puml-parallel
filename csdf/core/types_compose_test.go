package core

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/types"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
)

func typed(d *Diagram, types map[string]types.Type, channels map[string][]types.Type) *Diagram {
	d.Types, d.Channels = types, channels
	return d
}

func TestComposeKeepsTheTypesOfBothSides(t *testing.T) {
	// Setup
	l := typed(oneState(t, "p", []string{"x"}, "$x' = 0", loop(t, "p", ev("put", send("x")), "", "$x' = $x")),
		map[string]types.Type{"x": types.Named("N")}, map[string][]types.Type{"put": {types.Named("N")}})
	r := typed(oneState(t, "c", []string{"x"}, "$x' = 0", loop(t, "c", ev("put", recv("v")), "", "$x' = $v"),
		loop(t, "c", ev("stop"), "", "$x' = $x")),
		map[string]types.Type{"x": types.Named("M")}, map[string][]types.Type{"put": {types.Named("N")}, "stop": {}})

	// Execute
	got, err := Compose(l, r, []string{"put"})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string]types.Type{"x": types.Named("N"), "x2": types.Named("M")}, got.Types); diff != "" {
		t.Errorf("Types (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string][]types.Type{"put": {types.Named("N")}, "stop": {}}, got.Channels); diff != "" {
		t.Errorf("Channels (-want +got):\n%s", diff)
	}
}

func TestComposeRefusesAChannelTheSidesDeclareApart(t *testing.T) {
	// Setup
	l := typed(oneState(t, "p", nil, "", loop(t, "p", ev("put", send("x")), "", "")), nil, map[string][]types.Type{"put": {types.Named("N")}})
	r := typed(oneState(t, "c", nil, "", loop(t, "c", ev("put", recv("v")), "", "")), nil, map[string][]types.Type{"put": {types.Named("M")}})

	// Execute
	_, err := Compose(l, r, []string{"put"})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "the channel put is declared put : N on one side and put : M on the other") {
		t.Errorf("Compose: got %v, want a refusal of the two declarations of put", err)
	}
}

func TestHideBindsAFieldByTheTypeOfItsChannel(t *testing.T) {
	// Setup
	d := typed(oneState(t, "c", []string{"got"}, "$got' = 0", loop(t, "c", ev("put", recv("v")), "", "$got' = $v")),
		map[string]types.Type{"got": types.Named("N")}, map[string][]types.Type{"put": {types.Named("N")}, "keep": {}})

	// Execute
	got, err := Hide(d, []string{"put"})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if want := "@exists(v : N) ($got' = $v)"; printed(t, got.Edges[0].Post) != want {
		t.Errorf("the post is %s, want %s", printed(t, got.Edges[0].Post), want)
	}
	if diff := cmp.Diff(map[string][]types.Type{"keep": {}}, got.Channels); diff != "" {
		t.Errorf("Channels (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(d.Types, got.Types); diff != "" {
		t.Errorf("Types (-want +got):\n%s", diff)
	}
}

func printed(t *testing.T, f clause.And) string {
	t.Helper()
	s, err := clause.Print(f)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
