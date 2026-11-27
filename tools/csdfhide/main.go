package main

import (
	"github.com/Kuniwak/puml-parallel/v9/tools"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfhide/csdfhidecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfhidecmd.NewParseOptionsFunc(),
		csdfhidecmd.NewMainFunc(),
	).Run()
}
