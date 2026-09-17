package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdftracechk/csdftracechkcmd"
)

func main() {
	tools.NewCommandFunc(
		csdftracechkcmd.NewParseOptionsFunc(),
		csdftracechkcmd.NewMainFunc(),
	).Run()
}
