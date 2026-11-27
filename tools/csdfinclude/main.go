package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdfinclude/csdfincludecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfincludecmd.NewParseOptionsFunc(),
		csdfincludecmd.NewMainFunc(),
	).Run()
}
