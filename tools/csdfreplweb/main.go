package main

import (
	"github.com/Kuniwak/puml-parallel/v9/tools"
	"github.com/Kuniwak/puml-parallel/v9/tools/csdfreplweb/csdfreplwebcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfreplwebcmd.NewParseOptionsFunc(),
		csdfreplwebcmd.NewMainFunc(),
	).Run()
}
