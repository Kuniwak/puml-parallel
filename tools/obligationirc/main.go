package main

import (
	"github.com/Kuniwak/puml-parallel/v5/tools"
	"github.com/Kuniwak/puml-parallel/v5/tools/obligationirc/obligationirccmd"
)

func main() {
	tools.NewCommandFunc(
		obligationirccmd.NewParseOptionsFunc(),
		obligationirccmd.NewMainFunc(),
	).Run()
}
