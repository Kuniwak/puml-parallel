package main

import (
	"os"

	"github.com/Kuniwak/puml-parallel/tools/csdfevents/csdfeventscmd"
)

func main() {
	os.Exit(csdfeventscmd.Run(os.Args[1:], os.Stdout, os.Stderr))
}
