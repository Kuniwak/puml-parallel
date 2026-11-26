package main

import (
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/tools/csdfparse/csdfparsecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfparsecmd.NewParseOptionsFunc(),
		csdfparsecmd.NewMainFunc(),
	).Run()
}
