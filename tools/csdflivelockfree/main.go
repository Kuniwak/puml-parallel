package main

import (
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/tools/csdflivelockfree/csdflivelockfreecmd"
)

func main() {
	tools.NewCommandFunc(
		csdflivelockfreecmd.NewParseOptionsFunc(),
		csdflivelockfreecmd.NewMainFunc(),
	).Run()
}
