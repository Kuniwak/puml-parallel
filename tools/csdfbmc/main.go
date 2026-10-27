package main

import (
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdfbmc/csdfbmccmd"
)

func main() {
	tools.NewCommandFunc(
		csdfbmccmd.NewParseOptionsFunc(),
		csdfbmccmd.NewMainFunc(),
	).Run()
}
