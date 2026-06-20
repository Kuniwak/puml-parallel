package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/Kuniwak/puml-parallel/tools/csdfrepl/csdfreplcmd"
)

func main() {
	flag.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: %s <file.puml|file.png>\n", os.Args[0])
		_, _ = fmt.Fprintln(os.Stderr, "Interactively explores a Composable State Diagram.")
	}
	flag.Parse()

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	if exitCode := csdfreplcmd.Run(flag.Args(), os.Stdin, os.Stdout, os.Stderr, interrupts); exitCode != 0 {
		os.Exit(exitCode)
	}
}
