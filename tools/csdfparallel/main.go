package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfparallel/csdfparallelcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfparallelcmd.NewParseOptionsFunc(),
		csdfparallelcmd.NewMainFunc(),
	).Run()
}
