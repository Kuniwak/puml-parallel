package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdftranstable/csdftranstablecmd"
)

func main() {
	tools.NewCommandFunc(
		csdftranstablecmd.NewParseOptionsFunc(),
		csdftranstablecmd.NewMainFunc(),
	).Run()
}
