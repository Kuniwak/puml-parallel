package main

import (
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfsort/csdfsortcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfsortcmd.NewParseOptionsFunc(),
		csdfsortcmd.NewMainFunc(),
	).Run()
}
