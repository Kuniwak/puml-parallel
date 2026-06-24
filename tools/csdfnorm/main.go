package main

import (
	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/csdfnorm/csdfnormcmd"
)

func main() {
	cli.NewCommandFunc(
		csdfnormcmd.NewParseOptionsFunc(),
		csdfnormcmd.NewMainFunc(),
	).Run()
}
