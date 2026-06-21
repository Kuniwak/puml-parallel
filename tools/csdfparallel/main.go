package main

import (
	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/csdfparallel/csdfparallelcmd"
)

func main() {
	cli.NewCommandFunc(
		csdfparallelcmd.NewParseOptionsFunc(),
		csdfparallelcmd.NewMainFunc(),
	).Run()
}
