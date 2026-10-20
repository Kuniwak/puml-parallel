package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdfbmc/csdfbmccmd"
)

func main() {
	tools.NewCommandFunc(
		csdfbmccmd.NewParseOptionsFunc(),
		csdfbmccmd.NewMainFunc(),
	).Run()
}
