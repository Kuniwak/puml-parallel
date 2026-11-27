package main

import (
	"github.com/Kuniwak/puml-parallel/v9/tools"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfinclude/csdfincludecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfincludecmd.NewParseOptionsFunc(),
		csdfincludecmd.NewMainFunc(),
	).Run()
}
