package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfrefinement/csdfrefinementcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfrefinementcmd.NewParseOptionsFunc(),
		csdfrefinementcmd.NewMainFunc(),
	).Run()
}
