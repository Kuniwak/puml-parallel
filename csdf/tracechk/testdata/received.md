# trace.tsv: ACCEPTED when every guard is true

After every prefix of the trace, every stable state the diagram may be in can
perform the next event when every guard is taken as true. Whether it can in
fact depends on the clauses below, whose atoms this tool does not evaluate. A
path never revisits a state within one run of `tau` edges, so paths going
round a `tau` cycle, and their obligations, are not listed.

- trace: `x?k`, `y`
- the start edge `[*] --> a` (L7) must admit a valuation: ∃ n@0. "$_0 = 0"(n@0)

## Event 1: `x?k` after the empty prefix

### Path 1.1: reaches a

| step | event | transition | guard | post |
|-----:|-------|------------|-------|------|
| 0 | (start) | `[*] --> a` (L7) |  | $n' = 0 |

- after step 0: a (n@0)

| edge out of the reached state | transition | guard | post |
|-------------------------------|------------|-------|------|
| next | `a --> b` (L8) | $n < $k | $n' = $n + $k ∧ $m' = 0 |

Obligation: ∀ n@0. "$_0 = 0"(n@0) → ∃ k n' m'. "$_0 < $_1"(n@0, k) ∧ "$_0 = $_1 + $_2"(n', n@0, k) ∧ "$_0 = 0"(m')

## Event 2: `y` after `x?k`

### Path 2.1: reaches b

| step | event | transition | guard | post |
|-----:|-------|------------|-------|------|
| 0 | (start) | `[*] --> a` (L7) |  | $n' = 0 |
| 1 | `x?k` | `a --> b` (L8) | $n < $k | $n' = $n + $k ∧ $m' = 0 |

- after step 0: a (n@0)
- after step 1: b (n@1, m@1)

| edge out of the reached state | transition | guard | post |
|-------------------------------|------------|-------|------|
| tau | `b --> a` (L9) | $m = 0 | @eq(n', n) |
| next | `b --> b` (L10) | $m > 0 | @eq(n', n) ∧ @eq(m', m) |

Obligation: ∀ n@0 k#1 n@1 m@1. "$_0 = 0"(n@0) ∧ "$_0 < $_1"(n@0, k#1) ∧ "$_0 = $_1 + $_2"(n@1, n@0, k#1) ∧ "$_0 = 0"(m@1) → (∃ n'. "$_0 = 0"(m@1) ∧ n' = n@1) ∨ ∃ n' m'. "$_0 > 0"(m@1) ∧ n' = n@1 ∧ m' = m@1

### Path 2.2: reaches a

| step | event | transition | guard | post |
|-----:|-------|------------|-------|------|
| 0 | (start) | `[*] --> a` (L7) |  | $n' = 0 |
| 1 | `x?k` | `a --> b` (L8) | $n < $k | $n' = $n + $k ∧ $m' = 0 |
| 2 | `tau` | `b --> a` (L9) | $m = 0 | @eq(n', n) |

- after step 0: a (n@0)
- after step 1: b (n@1, m@1)
- after step 2: a (n@2)

| edge out of the reached state | transition | guard | post |
|-------------------------------|------------|-------|------|
| next | `a --> a` (L11) | $n > 0 | @eq(n', n) |

Obligation: ∀ n@0 k#1 n@1 m@1 n@2. "$_0 = 0"(n@0) ∧ "$_0 < $_1"(n@0, k#1) ∧ "$_0 = $_1 + $_2"(n@1, n@0, k#1) ∧ "$_0 = 0"(m@1) ∧ "$_0 = 0"(m@1) ∧ n@2 = n@1 → ∃ n'. "$_0 > 0"(n@2) ∧ n' = n@2

## Prompt

Decide whether every obligation above holds, reading each atom as the
statement its text makes of its holes. Answer HOLDS, or FAILS naming the
first obligation that does not hold and valuations that satisfy its premise
but not its conclusion. A guard is not true because it is written down: the
trace is a trace of the diagram only if the clauses make every obligation
hold. FAILS settles that it is not; HOLDS settles it only for the listed
paths, since paths going round a `tau` cycle are not listed.

