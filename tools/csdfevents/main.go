package main

import (
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/tools/csdfevents/csdfeventscmd"
)

func main() {
	tools.NewCommandFunc(
		csdfeventscmd.NewParseOptionsFunc(),
		csdfeventscmd.NewMainFunc(),
	).Run()
}
