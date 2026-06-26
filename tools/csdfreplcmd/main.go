package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfreplcmd/csdfreplcmdcmd"
)

func main() {
	tools.NewSubcommandFunc(
		"csdfreplcmd",
		"Drives a csdfrepld interactive exploration session.",
		csdfreplcmdcmd.Subcommands(),
	).Run()
}
