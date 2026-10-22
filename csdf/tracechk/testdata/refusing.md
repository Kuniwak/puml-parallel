# trace.tsv: REJECTED

After `1` (1 of 2 events) the diagram may be in B, C.
B is stable and has no edge for `2`, so the diagram may refuse `2` there even
when every guard is true: this is not a trace of the diagram.

- visible events enabled at the refusing states: (none)

The refusal is real unless every path below is infeasible, that is, unless
the clauses along it cannot all hold.

## Path 1: reaches B

| step | event | transition | guard | post |
|-----:|-------|------------|-------|------|
| 0 | (start) | `[*] --> A` (L7) |  | $n' = 0 |
| 1 | `1` | `A --> B` (L8) | $n = 0 | true |

- after step 0: A (n@0)
- after step 1: B (no variables)

Infeasible iff: ¬∃ n@0. "$_0 = 0"(n@0) ∧ "$_0 = 0"(n@0)

