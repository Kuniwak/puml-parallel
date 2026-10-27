package main

import (
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfevents/csdfeventscmd"
)

func main() {
	tools.NewCommandFunc(
		csdfeventscmd.NewParseOptionsFunc(),
		csdfeventscmd.NewMainFunc(),
	).Run()
}
