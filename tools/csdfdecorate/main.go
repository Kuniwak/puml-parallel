package main

import (
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/tools/csdfdecorate/csdfdecoratecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfdecoratecmd.NewParseOptionsFunc(),
		csdfdecoratecmd.NewMainFunc(),
	).Run()
}
