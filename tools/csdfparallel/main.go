package main

import (
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/tools/csdfparallel/csdfparallelcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfparallelcmd.NewParseOptionsFunc(),
		csdfparallelcmd.NewMainFunc(),
	).Run()
}
