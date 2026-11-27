package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdflivelockfree/csdflivelockfreecmd"
)

func main() {
	tools.NewCommandFunc(
		csdflivelockfreecmd.NewParseOptionsFunc(),
		csdflivelockfreecmd.NewMainFunc(),
	).Run()
}
