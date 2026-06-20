package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Kuniwak/puml-parallel/tools/csdfparse/csdfparsecmd"
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s < <file.puml>\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "Parses a PlantUML state diagram from stdin and prints the parsed structure as JSON.\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nExamples:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  $ %s < path/to/file.puml\n", os.Args[0])
		_, _ = fmt.Fprintf(os.Stderr, "  $ cat path/to/file.puml | %s\n", os.Args[0])
	}
	flag.Parse()

	if exitCode := csdfparsecmd.Run(os.Stdin, os.Stdout, os.Stderr); exitCode != 0 {
		os.Exit(exitCode)
	}
}
