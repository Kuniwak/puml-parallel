package main

import (
	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/csdfrepl/csdfreplcmd"
)

func main() {
	cli.NewCommandFunc(
		csdfreplcmd.NewParseOptionsFunc(),
		csdfreplcmd.NewMainFunc(),
	).Run()
}
