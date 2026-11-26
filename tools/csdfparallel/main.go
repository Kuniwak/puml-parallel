package main

import (
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/Kuniwak/puml-parallel/v5/tools/csdfparallel/csdfparallelcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfparallelcmd.NewParseOptionsFunc(),
		csdfparallelcmd.NewMainFunc(),
	).Run()
}
