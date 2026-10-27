package main

import (
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfrefinement/csdfrefinementcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfrefinementcmd.NewParseOptionsFunc(),
		csdfrefinementcmd.NewMainFunc(),
	).Run()
}
