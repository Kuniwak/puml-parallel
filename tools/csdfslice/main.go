package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdfslice/csdfslicecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfslicecmd.NewParseOptionsFunc(),
		csdfslicecmd.NewMainFunc(),
	).Run()
}
