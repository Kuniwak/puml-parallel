package main

import (
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/tools/csdfreplweb/csdfreplwebcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfreplwebcmd.NewParseOptionsFunc(),
		csdfreplwebcmd.NewMainFunc(),
	).Run()
}
