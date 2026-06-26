package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdflivelockfree/csdflivelockfreecmd"
)

func main() {
	tools.NewCommandFunc(
		csdflivelockfreecmd.NewParseOptionsFunc(),
		csdflivelockfreecmd.NewMainFunc(),
	).Run()
}
