package main

import (
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/tools/csdfreplweb/csdfreplwebcmd"
)

func main() {
	tools.NewCommandFunc(
		csdfreplwebcmd.NewParseOptionsFunc(),
		csdfreplwebcmd.NewMainFunc(),
	).Run()
}
