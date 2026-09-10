package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdflint/csdflintcmd"
)

func main() {
	tools.NewCommandFunc(
		csdflintcmd.NewParseOptionsFunc(),
		csdflintcmd.NewMainFunc(),
	).Run()
}
