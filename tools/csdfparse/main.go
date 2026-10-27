package main

import (
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfparse/csdfparsecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfparsecmd.NewParseOptionsFunc(),
		csdfparsecmd.NewMainFunc(),
	).Run()
}
