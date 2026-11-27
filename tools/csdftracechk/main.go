package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/csdftracechk/csdftracechkcmd"
)

func main() {
	tools.NewCommandFunc(
		csdftracechkcmd.NewParseOptionsFunc(),
		csdftracechkcmd.NewMainFunc(),
	).Run()
}
