package main

import (
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/tools/csdftracechk/csdftracechkcmd"
)

func main() {
	tools.NewCommandFunc(
		csdftracechkcmd.NewParseOptionsFunc(),
		csdftracechkcmd.NewMainFunc(),
	).Run()
}
