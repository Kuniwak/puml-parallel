package main

import (
	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/puml-events/events"
)

func main() {
	cli.Run(events.MainCommandByArgs)
}
