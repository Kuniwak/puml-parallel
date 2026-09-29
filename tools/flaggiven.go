package tools

import "flag"

// FlagGiven reports whether the flag named name was given on the command line
// flags parsed, not merely left at its default.
func FlagGiven(flags *flag.FlagSet, name string) bool {
	given := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == name {
			given = true
		}
	})
	return given
}
