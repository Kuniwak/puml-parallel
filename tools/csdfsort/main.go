package main

import (
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/tools/csdfsort/csdfsortcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfsortcmd.NewParseOptionsFunc(),
		csdfsortcmd.NewMainFunc(),
	).Run()
}
