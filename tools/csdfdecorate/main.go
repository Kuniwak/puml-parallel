package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdfdecorate/csdfdecoratecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfdecoratecmd.NewParseOptionsFunc(),
		csdfdecoratecmd.NewMainFunc(),
	).Run()
}
