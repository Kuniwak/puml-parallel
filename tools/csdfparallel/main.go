package main

import (
	"os"

	"github.com/Kuniwak/puml-parallel/tools/csdfparallel/csdfparallelcmd"
)

func main() {
	os.Exit(csdfparallelcmd.Run(os.Args[1:], os.Stdout, os.Stderr))
}
