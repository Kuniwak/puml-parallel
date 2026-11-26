package main

import (
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/tools/csdfpromote/csdfpromotecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfpromotecmd.NewParseOptionsFunc(),
		csdfpromotecmd.NewMainFunc(),
	).Run()
}
