package main

import (
	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/csdfparse/csdfparsecmd"
)

func main() {
	cli.NewCommandFunc(
		csdfparsecmd.NewParseOptionsFunc(),
		csdfparsecmd.NewMainFunc(),
	).Run()
}
