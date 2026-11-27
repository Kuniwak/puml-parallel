package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdflint/csdflintcmd"
)

func main() {
	tools.NewCommandFunc(
		csdflintcmd.NewParseOptionsFunc(),
		csdflintcmd.NewMainFunc(),
	).Run()
}
