package main

import (
	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/csdfrepld/csdfrepldcmd"
)

func main() {
	cli.NewCommandFunc(
		csdfrepldcmd.NewParseOptionsFunc(),
		csdfrepldcmd.NewMainFunc(),
	).Run()
}
