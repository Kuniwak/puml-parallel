package main

import (
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/Kuniwak/puml-parallel/v5/tools/csdftranstable/csdftranstablecmd"
)

func main() {
	tools.NewCommandFunc(
		csdftranstablecmd.NewParseOptionsFunc(),
		csdftranstablecmd.NewMainFunc(),
	).Run()
}
