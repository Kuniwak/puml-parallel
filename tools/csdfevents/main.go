package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfevents/csdfeventscmd"
)

func main() {
	tools.NewCommandFunc(
		csdfeventscmd.NewParseOptionsFunc(),
		csdfeventscmd.NewMainFunc(),
	).Run()
}
