package main

import (
	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/parallel"
)

func main() {
	cli.Run(parallel.MainCommandByArgs)
}
