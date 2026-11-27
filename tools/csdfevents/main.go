package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdfevents/csdfeventscmd"
)

func main() {
	tools.NewCommandFunc(
		csdfeventscmd.NewParseOptionsFunc(),
		csdfeventscmd.NewMainFunc(),
	).Run()
}
