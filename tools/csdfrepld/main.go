package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfrepld/csdfrepldcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfrepldcmd.NewParseOptionsFunc(),
		csdfrepldcmd.NewMainFunc(),
	).Run()
}
