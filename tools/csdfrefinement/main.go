package main

import (
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/tools/csdfrefinement/csdfrefinementcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfrefinementcmd.NewParseOptionsFunc(),
		csdfrefinementcmd.NewMainFunc(),
	).Run()
}
