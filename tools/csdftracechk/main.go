package main

import (
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/Kuniwak/puml-parallel/v5/tools/csdftracechk/csdftracechkcmd"
)

func main() {
	tools.NewCommandFunc(
		csdftracechkcmd.NewParseOptionsFunc(),
		csdftracechkcmd.NewMainFunc(),
	).Run()
}
