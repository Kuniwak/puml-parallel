package main

import (
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/tools/csdfrepl/csdfreplcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfreplcmd.NewParseOptionsFunc(),
		csdfreplcmd.NewMainFunc(),
	).Run()
}
