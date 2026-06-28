package main

import (
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/obligationirc/obligationirccmd"
)

func main() {
	tools.NewCommandFunc(
		obligationirccmd.NewParseOptionsFunc(),
		obligationirccmd.NewMainFunc(),
	).Run()
}
