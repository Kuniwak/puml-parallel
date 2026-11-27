package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdftranstable/csdftranstablecmd"
)

func main() {
	tools.NewCommandFunc(
		csdftranstablecmd.NewParseOptionsFunc(),
		csdftranstablecmd.NewMainFunc(),
	).Run()
}
