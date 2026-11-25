package main

import (
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/tools/csdfdecorate/csdfdecoratecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfdecoratecmd.NewParseOptionsFunc(),
		csdfdecoratecmd.NewMainFunc(),
	).Run()
}
