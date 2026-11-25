// Command lower prints the plain diagram on standard input as core.DiagramDoc,
// the JSON csdfbmc reads: the syntax tree csdf.Parse reads, lowered by
// coreio.Lower. It is the pages build's until csdfpromote -format json writes
// a plain diagram too (#215).
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Kuniwak/puml-parallel/v4/csdf"
	"github.com/Kuniwak/puml-parallel/v4/csdf/core"
	"github.com/Kuniwak/puml-parallel/v4/csdf/coreio"
)

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(in io.Reader, out io.Writer) error {
	source, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	tree, err := csdf.Parse(string(source))
	if err != nil {
		return err
	}
	d, err := coreio.Lower(tree)
	if err != nil {
		return err
	}
	raw, err := core.MarshalDiagram(d)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(raw))
	return err
}
