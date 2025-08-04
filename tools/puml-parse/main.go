package main

import (
	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/puml-parse/parse"
)

func main() {
	cli.Run(parse.MainCommandByArgs)
}
