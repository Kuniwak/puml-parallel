package main

import (
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/Kuniwak/puml-parallel/v5/tools/csdfinclude/csdfincludecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfincludecmd.NewParseOptionsFunc(),
		csdfincludecmd.NewMainFunc(),
	).Run()
}
