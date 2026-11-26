package main

import (
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/Kuniwak/puml-parallel/v5/tools/csdfslice/csdfslicecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfslicecmd.NewParseOptionsFunc(),
		csdfslicecmd.NewMainFunc(),
	).Run()
}
