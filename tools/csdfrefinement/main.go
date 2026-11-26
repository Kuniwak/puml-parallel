package main

import (
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/Kuniwak/puml-parallel/v5/tools/csdfrefinement/csdfrefinementcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfrefinementcmd.NewParseOptionsFunc(),
		csdfrefinementcmd.NewMainFunc(),
	).Run()
}
