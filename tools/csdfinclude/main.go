package main

import (
	"github.com/Kuniwak/puml-parallel/v7/tools"
	"github.com/Kuniwak/puml-parallel/v7/tools/csdfinclude/csdfincludecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfincludecmd.NewParseOptionsFunc(),
		csdfincludecmd.NewMainFunc(),
	).Run()
}
