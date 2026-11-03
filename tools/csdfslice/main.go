package main

import (
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfslice/csdfslicecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfslicecmd.NewParseOptionsFunc(),
		csdfslicecmd.NewMainFunc(),
	).Run()
}
