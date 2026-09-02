package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfsort/csdfsortcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfsortcmd.NewParseOptionsFunc(),
		csdfsortcmd.NewMainFunc(),
	).Run()
}
