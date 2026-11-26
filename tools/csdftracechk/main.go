package main

import (
	"github.com/Kuniwak/puml-parallel/v7/tools"
	"github.com/Kuniwak/puml-parallel/v7/tools/csdftracechk/csdftracechkcmd"
)

func main() {
	tools.NewCommandFunc(
		csdftracechkcmd.NewParseOptionsFunc(),
		csdftracechkcmd.NewMainFunc(),
	).Run()
}
