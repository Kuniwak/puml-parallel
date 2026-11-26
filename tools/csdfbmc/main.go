package main

import (
	"github.com/Kuniwak/puml-parallel/v7/tools"
	"github.com/Kuniwak/puml-parallel/v7/tools/csdfbmc/csdfbmccmd"
)

func main() {
	tools.NewCommandFunc(
		csdfbmccmd.NewParseOptionsFunc(),
		csdfbmccmd.NewMainFunc(),
	).Run()
}
