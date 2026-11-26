package main

import (
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/Kuniwak/puml-parallel/v5/tools/csdflivelockfree/csdflivelockfreecmd"
)

func main() {
	tools.NewCommandFunc(
		csdflivelockfreecmd.NewParseOptionsFunc(),
		csdflivelockfreecmd.NewMainFunc(),
	).Run()
}
