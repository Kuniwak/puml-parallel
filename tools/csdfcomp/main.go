package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfcomp/csdfcompcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfcompcmd.NewParseOptionsFunc(),
		csdfcompcmd.NewMainFunc(),
	).Run()
}
