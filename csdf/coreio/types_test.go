package coreio

import (
	"strings"
	"testing"
)

func TestReadAuthoredRefusesWhatTheTypesDoNotDeclare(t *testing.T) {
	cases := []struct {
		name, source, want string
	}{
		{
			name:   "a variable with no type",
			source: "@startuml\nstate \"s\" as s\ns : n\n[*] --> s : @eq(n', 0)\n@enduml\n",
			want:   "the variable n of s has no type; declare it as s : n ; <type>, or s : n ; any for a value of any type",
		},
		{
			name:   "a variable of two types",
			source: "@startuml\nstate \"s\" as s\ns : n ; N\nstate \"t\" as t\nt : n ; M\n[*] --> s : @eq(n', 0)\n@enduml\n",
			want:   "the variable n is declared of the type N and of the type M; a variable has one type in a diagram",
		},
		{
			name:   "a channel with fields that no note declares",
			source: "@startuml\nstate \"s\" as s\n[*] --> s\ns --> s : put?k\n@enduml\n",
			want:   "the channel put carries fields, and no channel note declares their types; declare it as channel put : <type>",
		},
		{
			name:   "an event with fields other than its channel's",
			source: "@startuml\nstate \"s\" as s\n[*] --> s\ns --> s : put?k?v\nnote as channels\n  channel put : K\nend note\n@enduml\n",
			want:   "s --> s : put?k?v has 2 fields, but the channel put is declared with 1",
		},
		{
			name:   "a variable sent on a field of another type",
			source: "@startuml\nstate \"s\" as s\ns : n ; N\n[*] --> s : @eq(n', 0)\ns --> s : put!n ; ; @eq(n', n)\nnote as channels\n  channel put : K\nend note\n@enduml\n",
			want:   "s --> s : put!n sends n, of the type N, on a field of the type K",
		},
		{
			name:   "a declared tau",
			source: "@startuml\nstate \"s\" as s\n[*] --> s\nnote as channels\n  channel tau\nend note\n@enduml\n",
			want:   "tau is internal and has no fields; remove its declaration from the channel note",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			_, err := ReadAuthored(c.source)

			// Assert
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("ReadAuthored: got %v, want an error containing %q", err, c.want)
			}
		})
	}
}

func TestReadAuthoredAcceptsAnySentOnAnyField(t *testing.T) {
	// Setup
	source := "@startuml\nstate \"s\" as s\ns : n ; N\n[*] --> s : @eq(n', 0)\ns --> s : put!n ; ; @eq(n', n)\ns --> s : stop\nnote as channels\n  channel put : any\nend note\n@enduml\n"

	// Execute
	_, err := ReadAuthored(source)

	// Assert
	if err != nil {
		t.Errorf("ReadAuthored: %v", err)
	}
}
