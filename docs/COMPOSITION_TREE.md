Composition Tree Format
=======================
A JSON representation of the reference, hiding, and interface parallel expressions
needed to reduce every component's Composable State Diagram (see [SYNTAX.md](./SYNTAX.md))
to the single diagram that is checked for stable-failures refinement against a
top-level specification.

`csdfcomp` reads a composition tree and prints the composed diagram as PlantUML.

```json
{
  "op": "HIDE",
  "proc": {
    "op": "INTERFACE_PARALLEL",
    "sync": ["EVT-A"],
    "procs": [
      {"op": "REFER", "path": "path/to/COMPONENT-A.puml"},
      {"op": "REFER", "path": "path/to/COMPONENT-B.puml"}
    ]
  },
  "events": ["EVT-A", "EVT-B"]
}
```


Elements
--------
### Process expression
A reference expression, a hiding expression, or an interface parallel expression.

```
proc = refer-expr | hide-expr | para-expr
```


### Reference expression
A file path to an existing state diagram in the [CSDF](./SYNTAX.md) format. Both
`.puml` text files and `.png` images written by PlantUML are accepted, as
everywhere else in this repository. A relative path is resolved against the
directory of the tree file (against the current directory when the tree is read
from standard input), which `csdfcomp -base` overrides. The path must not be empty.

```json
{
  "op": "REFER",
  "path": string
}
```


### Hiding expression
Hides the given events of the given process (CSP hiding, `P \ A`): every edge
carrying one of the events becomes an internal `tau` transition. Events that do
not occur in the process are ignored.

```json
{
  "op": "HIDE",
  "events": string[],
  "proc": proc
}
```


### Interface parallel expression
Composes the given processes in parallel synchronising on the given events (CSP
interface parallel). A single process composes to itself, and more than two
processes are folded with the same synchronisation set. At least one process is
required. End edges (`state --> [*]`) are not supported by parallel composition.

```json
{
  "op": "INTERFACE_PARALLEL",
  "sync": string[],
  "procs": proc[]
}
```
