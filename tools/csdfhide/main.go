package main

import (
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/tools/csdfhide/csdfhidecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfhidecmd.NewParseOptionsFunc(),
		csdfhidecmd.NewMainFunc(),
	).Run()
}
