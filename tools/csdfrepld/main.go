package main

import (
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/tools/csdfrepld/csdfrepldcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfrepldcmd.NewParseOptionsFunc(),
		csdfrepldcmd.NewMainFunc(),
	).Run()
}
