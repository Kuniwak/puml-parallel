package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdfrepld/csdfrepldcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfrepldcmd.NewParseOptionsFunc(),
		csdfrepldcmd.NewMainFunc(),
	).Run()
}
