# Clause design principles

This document records what a clause (a guard or a postcondition of a
transition) has to be, and why. It is the reference for future decisions on
the clause language: a change to clauses should say which operators and
properties below it serves, and must not break any of them.

The operators are taken from the current tools and from what they are
expected to grow into; the properties are derived from the operators; the
syntax is derived from the properties and deliberately not from the current
syntax.

## Terms

- A **glue** is a program that gives the atoms of one clause language their
  meaning, through the evaluator protocol; `csdfbmc/python_glue.py` is the
  reference glue for Python.
- The **environment** is the set of functions that atoms without a glue
  denote (see P15, P16). An environment function, `envfn` below, is pure.

## Semantic ground

A diagram whose transitions carry guards and postconditions over state
variables denotes a labelled transition system: expanding the state
variables away gives a graph without them. The meaning ⟦D⟧ of a diagram D
is that graph in the stable failures model of CSP: two diagrams mean the
same when their graphs are stable-failures equivalent (=SF), and one refines
another by stable-failures refinement (⊑SF).

The stable failures model is used throughout, for every operator below:

- it is a congruence for the CSP operators, so the algebraic laws of CSP
  (for example a → (P ⊓ Q) = (a → P) ⊓ (a → Q)) hold, which strong
  bisimulation would not grant;
- equality and refinement must be taken in one model, or an abstraction
  proven under one would not compose with a construction proven under
  another.

Divergence is therefore not part of the meaning. Checking livelock would
need the failures-divergences model, and moving to it would be a change for
every operator at once, not for one.

Any representation of a clause, text or JSON, is acceptable as long as it
determines that meaning by itself.

### What each kind of operator must satisfy

Every operator below is given a denotation, and the obligation depends on
its kind:

| Kind | Operators | Obligation |
|---|---|---|
| Change of representation | splitting a conjunction, parsing and printing, α-renaming | ⟦op(D)⟧ =SF ⟦D⟧ |
| Constructor | composition, hiding, channel renaming, promotion, replicated composition | ⟦op(D₁, …)⟧ =SF op_sem(⟦D₁⟧, …), where op_sem is the CSP operator |
| Abstraction | restriction, data abstraction | ⟦op(D)⟧ ⊑SF ⟦D⟧: the original refines its abstraction (P ⊑ Q reads "Q refines P") |
| Analysis | evaluation, translation to logic, refinement checking | sound with respect to ⟦·⟧, and complete where stated |
| Optimisation | partial-order reduction, symmetry, cone of influence | the answer to the property checked is unchanged; the graph may differ |

For composition, op_sem is the CSP parallel operator on the graphs:
⟦D₁ ‖A D₂⟧ =SF ⟦D₁⟧ ‖A ⟦D₂⟧; for hiding, ⟦D \ C⟧ =SF ⟦D⟧ \ C.

For promotion, op_sem is a CSP term built from the local diagram P:

- each instance i starts absent; creation leads it into P with every
  channel c renamed to c.i, and deletion leads it back to absent;
- all instances are interleaved over the set of IDs, ||| i ∈ ID;
- a sync renames the local events it takes to the global event and
  synchronises on it (for `MOVE`, `DEPART.from` and `ARRIVE.to` become
  `MOVE`).

The map of a promoted diagram is then the tuple of the instances' local
states, domain membership is "not absent", and an instance's control state
is its position in P; the obligation is that the expanded graph of the
promoted diagram is stable-failures equivalent to that term.

## Operators on clauses

Present:

| # | Operator | Where it occurs today |
|---|---|---|
| O1 | Conjunction and its splitting | composition and promotion join guards and posts; `Predicate.Clauses` splits them |
| O2 | Substitution of a name by a name | renaming a field when two events synchronise |
| O3 | Substitution of a name by a term | promotion lifts a local state variable to an entry of a map, `x` to `m[id].x` |
| O4 | Existential quantification | hiding closes a field of a hidden event; `csdfpromote -only` drops maps |
| O5 | Relational composition | two updates of one map in one step; tau closure in obligations |
| O6 | Generation of atoms | equality, distinctness, domain membership, map update, frame and opaque clauses, stated by a tool |
| O7 | Evaluation | a valuation gives a truth value or a set of next valuations (csdfbmc, `csdf.SolveJSON`) |
| O8 | Translation to logic | proof obligations (csdftracechk, csdftranstable, csdfrefinement) |
| O9 | Free-name extraction | partial-order reduction, cone of influence, symmetry, restriction |
| O10 | Permutation of IDs | symmetry reduction |
| O11 | Refinement checking | relating a specification diagram to an implementation (csdfrefinement) |
| O12 | Parsing and printing | text to the internal form and back (`csdf.ParseClause`, JSON, the PUML printer) |
| O13 | Well-formedness checking | reserved words and escapes (`csdf.CheckClauses`, csdflint) |

Foreseen; the design must leave room for them:

| # | Operator | Meaning |
|---|---|---|
| O14 | Nested promotion | promoting a diagram that is itself promoted |
| O15 | Channel renaming | CSP `P[[a <- b]]` |
| O16 | Replicated composition | `|||` over a set of IDs, generalising families |
| O17 | Data abstraction | shrinking the state space through an abstraction map |

O11 exists today as proof obligations; checking it by bounded exploration is
foreseen.

## Properties

Each property names the operators that need it.

| # | Property | Needed by |
|---|---|---|
| P1 | Closed under ∧, with `true` as its unit | O1 |
| P2 | The free variables of a clause are visible to the tools | O2–O5, O8–O10, O17 |
| P3 | Substitution avoids capture; tools can make fresh names | O2–O5, O14–O16 |
| P4 | There is a syntax of terms, at least access to an entry of a map | O3, O6, O14 |
| P5 | A name, including a map-valued one, can be bound (∃) | O4, O5 |
| P6 | Values before and after a step are told apart, and intermediate states can be named | O5, O8 |
| P7 | Tools state facts without using any clause language | O6 |
| P8 | The meaning of an atom is given by a glue | O7 |
| P9 | An atom has an ordered list of arguments, its free variables | O8, O9 |
| P10 | IDs are told apart from constants | O10, O16 |
| P11 | Every operator has a denotation and meets the obligation of its kind (see "What each kind of operator must satisfy") | O1–O17 |
| P12 | A representation is self-contained: no meaning lives outside it | O12, and through it O7, O8 |
| P13 | Reserved words are told apart from the author's words, with escapes | O12, O13 |
| P14 | Identifiers may use Unicode | O12 |
| P15 | Every operator is defined on a diagram whose atoms are not all formalised; such an atom is an uninterpreted function of its arguments | O7, O8, O11 |
| P16 | The environment is a set of pure functions given from outside; a counterexample is reported with its environment so that it can be reproduced | O7 |

Optimisations need information but not meaning; they must never change a
result:

| # | Information | Used by |
|---|---|---|
| Q1 | The variables a clause reads and writes | partial-order reduction, cone of influence (O9) |
| Q2 | Whether a clause is invariant under permutation of IDs | symmetry reduction (O10) |
| Q3 | The maps a clause depends on | restriction (O9), O17 |
| Q4 | Narrowed domains for values | approximating the environment and bounded checking (O7) |

Q1 and Q3 follow from P2 without asking a glue.

Natural language and formal languages fit the same frame (P15): an atom
without a glue is an uninterpreted predicate of its arguments, and the
opaque clause becomes such a predicate too. An environment function is pure;
its values may be given from the command line or generated from a seed, so
running with different seeds widens what bounded checking covers, and the
seed reproduces a counterexample (P16).

## Sketch of the syntax

A clause has two layers: a skeleton that the tools operate on, and atoms
that a language interprets.

```
clause  ::= atom | builtin | clause ∧ clause | ∃ var . clause
builtin ::= term = term | term ≠ term | term ∈ dom term
          | map update | frame
term    ::= var | var' | term[term].field | envfn(term, …)
```

A variable bound by ∃ may stand for any value, a map included; that is how
an intermediate state is named (P5, P6).

An atom is text with marked holes. In the examples below the identifiers are
Japanese, as P14 allows, and the marker is not yet chosen:

```
‹在庫'› = ‹在庫› - ‹数量›
‹在庫› が ‹数量› 以上ある
```

- The tools know only where the holes are. Substitution (O2, O3) fills a hole
  with a term, so it never parses the text and never captures (P2–P4).
- The holes are the atom's arguments (P9), so obligations, reduction and
  restriction need no glue (Q1, Q3).
- A glue receives the text without the holes and the values for them (P8).
- The builtins are what tools state (P7). Two updates of one map in one step
  bind the intermediate map with ∃ (P5, P6).

Negation and disjunction are left undefined until an operator needs them;
see the open questions.

### Binding

Names are a surface notation only. Inside the tools, every binder (∃, the
fields of an event, the parameters of a step, the IDs of instances) is
represented without names, by de Bruijn indices or a locally nameless
representation (bound variables as indices, free variables as names).
Substitution, renaming and α-equivalence are then defined once, for every
operator, instead of being handled case by case (P3). Fresh names are
needed only when a clause is printed back (O12), and printing chooses them
so that they capture nothing.

## Open questions

- The hole marker, and how it is escaped.
- Whether authors may write ∃, or only tools.
- How an instance's control state (the state of its local diagram) is a
  term, and whether a clause may read another instance's control state:
  encoding control as data preserves the expanded graph as long as only
  transitions change it.
- Translation to logic (O8) and refinement (O11) may need negation; when
  they do, decide whether it belongs to the skeleton.
