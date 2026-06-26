package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfnorm/csdfnormcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfnormcmd.NewParseOptionsFunc(),
		csdfnormcmd.NewMainFunc(),
	).Run()
}
