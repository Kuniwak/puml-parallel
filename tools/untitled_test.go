package tools

import "testing"

func TestUntitled(t *testing.T) {
	cases := []struct{ name, puml, want string }{
		{"a title", "@startuml auto-generated-by: x a.puml\nstate \"a\" as a\n@enduml\n", "@startuml\nstate \"a\" as a\n@enduml\n"},
		{"no title", "@startuml\n@enduml\n", "@startuml\n@enduml\n"},
		{"no @startuml line", "text\n", "text\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			got := Untitled(c.puml)

			// Assert
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
