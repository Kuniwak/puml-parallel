package main

import (
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfdecorate/csdfdecoratecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfdecoratecmd.NewParseOptionsFunc(),
		csdfdecoratecmd.NewMainFunc(),
	).Run()
}
