package main

import (
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/Kuniwak/puml-parallel/v5/tools/csdfrepl/csdfreplcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfreplcmd.NewParseOptionsFunc(),
		csdfreplcmd.NewMainFunc(),
	).Run()
}
