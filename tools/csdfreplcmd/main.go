package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdfreplcmd/csdfreplcmdcmd"
)

func main() {
	tools.NewSubcommandFunc(
		"csdfreplcmd",
		csdfreplcmdcmd.Description,
		csdfreplcmdcmd.Subcommands(),
	).Run()
}
