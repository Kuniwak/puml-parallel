package tools

import "strings"

// Untitled is the PlantUML text puml with the title of its @startuml line
// left out, such as the command line GeneratedBy writes there, so that what
// a tool printed can be compared with a recording whatever paths it was
// given.
func Untitled(puml string) string {
	first, rest, _ := strings.Cut(puml, "\n")
	if !strings.HasPrefix(first, "@startuml") {
		return puml
	}
	return "@startuml\n" + rest
}
