package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdfpromote/csdfpromotecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfpromotecmd.NewParseOptionsFunc(),
		csdfpromotecmd.NewMainFunc(),
	).Run()
}
