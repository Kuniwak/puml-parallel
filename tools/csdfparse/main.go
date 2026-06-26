package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfparse/csdfparsecmd"
)

func main() {
	tools.NewCommandFunc(
		csdfparsecmd.NewParseOptionsFunc(),
		csdfparsecmd.NewMainFunc(),
	).Run()
}
