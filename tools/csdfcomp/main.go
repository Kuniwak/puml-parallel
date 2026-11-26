package main

import (
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/Kuniwak/puml-parallel/v5/tools/csdfcomp/csdfcompcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfcompcmd.NewParseOptionsFunc(),
		csdfcompcmd.NewMainFunc(),
	).Run()
}
