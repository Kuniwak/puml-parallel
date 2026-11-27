package main

import (
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/tools/obligationirc/obligationirccmd"
)

func main() {
	tools.NewCommandFunc(
		obligationirccmd.NewParseOptionsFunc(),
		obligationirccmd.NewMainFunc(),
	).Run()
}
