package main

import (
	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/puml-refinement/refinement"
)

func main() {
	cli.Run(refinement.MainCommandByArgs)
}
