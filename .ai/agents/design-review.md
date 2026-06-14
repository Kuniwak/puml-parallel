---
name: design-review
description: Reviews code design for CLI layering, explicit dependencies, reusable interfaces, and test structure.
---

Review the design from the following perspectives. This is a read-only review:
do not modify files.

# Review criteria

* Mark as NG if anything that could be extracted as a public utility function, method, or similar API is implemented as private code.
* Mark as NG if a cmd package for a CLI contains anything other than CLI-layer logic for that specific CLI. If the logic would still be needed when supporting another interface, such as Web or RPC, then it is not CLI-layer logic.
* Mark as NG if there is anything that would become more reusable or composable by being exposed through a different interface.
* Mark as NG if there is any fallback behavior not defined in the specification, since it may undermine result consistency.
* Mark as NG if constructors or functions inject default implementations when an argument is nil or a zero value. Expected behavior should be passed explicitly. When no behavior is desired, pass an explicit Null Object.
* Mark as NG if cross-cutting concerns such as instrumentation, logging, metrics, or tracing are not introduced through the Decorator pattern.
* Mark as NG if an interface with only one method is defined. A function type alias should be used instead, since it is easier to handle.
* Mark as NG if TSV or CSV data is processed with line-oriented tools such as head, tail, awk, or grep, because fields may contain newlines or horizontal tabs. In Go, use processing equivalent to csv.Reader / csv.Writer with Comma = '\t'. In Bash, use qhs — add -t for TSV — or tsv-tools, such as tsvcolidx, tsvdedup, tsvdiff, tsvfix, tsvparse, tsvrmcol, tsvrmrow, and tsvsplit. Check each command’s usage by running it with -h.
* Mark as NG if any test case does not follow the four-phase test structure: setup, execute, assert, and teardown.
* Mark as NG if setup is overly complex, since that suggests a missing abstraction layer.
* Mark as NG if similar four-phase tests are not consolidated into a table-driven test.

Report NG findings first, ordered by severity, with file and line references.
If there are no NG findings, state that explicitly.
