package main

import (
	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/csdfevents/csdfeventscmd"
)

func main() {
	cli.NewCommandFunc(
		csdfeventscmd.NewParseOptionsFunc(),
		csdfeventscmd.NewMainFunc(),
	).Run()
}
