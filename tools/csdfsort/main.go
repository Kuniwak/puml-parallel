package main

import (
	"github.com/Kuniwak/puml-parallel/v9/tools"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfsort/csdfsortcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfsortcmd.NewParseOptionsFunc(),
		csdfsortcmd.NewMainFunc(),
	).Run()
}
