package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunPrintsJSON(t *testing.T) {
	// Setup
	input := `@startuml
state "Initial" as s0
s0: ready ; bool
s0: count
state "Done" as s1
[*] --> s0 : initialize
s0 --> s1 : finish(result) ; ready ; done
s1 --> [*] : complete
@enduml
`
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	want := `{"states":{"s0":{"id":"s0","name":"Initial","vars":[{"name":"ready","type":"bool"},{"name":"count"}]},"s1":{"id":"s1","name":"Done","vars":[]}},"start_edge":{"dst":"s0","post":"initialize"},"edges":[{"src":"s0","dst":"s1","event":{"id":"finish","params":["result"]},"guard":"ready","post":"done"}],"end_edge":{"src":"s1","guard":"complete"}}` + "\n"

	// Execute
	exitCode := Run(strings.NewReader(input), stdout, stderr)

	// Assert
	if exitCode != 0 {
		t.Errorf("Run() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if stdout.String() != want {
		t.Errorf("Run() stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Errorf("Run() stderr = %q, want empty", stderr.String())
	}

	// Teardown: no resources to release.
}
