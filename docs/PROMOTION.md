Promotion
=========

A system built from many instances of one thing — an account per customer, a
trade per order, a report per business day — has no readable single Composable
State Diagram. Written as one, the control state of every instance is absorbed
into a state variable holding a partial map from instance IDs to local states,
and the diagram collapses to one global state with a bundle of self-loops. That
form is what `csdflivelockfree`, `csdfrefinement` and `csdfrepl` want. It is not
what a reader wants.

The readable form is the diagram of *one* instance. The two cannot be the same
artefact, so the author writes the local diagrams plus a few directives, and
`csdfpromote` generates the global one.

The generation has to run that way round. A local diagram is already a
structured Composable State Diagram, so expanding it into the global form needs
no structure that is not already there. Projecting a global diagram back onto
one instance would instead need conventions about map names and state names
*inside* the predicates — and a predicate in this format is opaque natural
language, which is exactly what such a convention would have to break.

Coupling has to be writable too. A guard that reaches across maps, and an event
that two kinds of instance take together, are the substance of a specification;
without somewhere to write them, promotion produces the direct product of
independent maps, which says almost nothing.


`csdfpromote` reads this syntax with clauses that are formulas
(docs/CLAUSE_DESIGN.md), through `csdf/coreio`, and prints the expansion
`csdf/core` gives. Its `-json` and `-format json` outputs still come from the
reader before it, which reads clauses as text and which this document
describes where the two differ; `csdfbmc` reads that JSON until it moves onto
the clause syntax (#76). The diagram JSON of an expansion (version 2) writes
the family of each map as well (`families`): its local states, its start
state, which is absence, and the variables each local state holds, so that
csdfbmc gives an instance at a local state exactly that state's variables.
The formula promotion keeps the local-diagram
contract below; it reads `constrain`, several global states with frozen
families and hand-written global edges, variables of the global state that
are no family's map, a post on the global start edge, global `assume`
notes, `-only` and templates (docs/CLAUSE_DESIGN.md gives their meaning). Its
differences from `csdfpromote`: a hand-written global edge's post primes a
map only as its frame, which promotion writes if the post leaves it out; a
hand-written edge is on a channel no generated edge carries; the global
start post may say only `@empty(m')` of maps, beside the start mode's own
variables; and every mode holds every family's map. `csdfpromote` skips a global `assume`
note as a note for the picture alone; `csdf/coreio` carries it into the
expansion.


Vocabulary
----------

The vocabulary is Z's.

| Directive   | Z                                                                | CSP                                                                               |
|:------------|:-----------------------------------------------------------------|:-----------------------------------------------------------------------------------|
| `promote`   | free promotion: `θLocal = m(id)`, `m' = m ⊕ {id ↦ θLocal'}`      | the symbolic form of `\|\|\| id : dom m @ Local(m(id))[[e ← e(id)]]`               |
| `sync`      | the conjunction of two framing schemas                            | alphabetised parallel, `[\| {e} \|]`                                               |
| `constrain` | constrained promotion                                             | a monitor process in parallel (docs/CLAUSE_DESIGN.md, the denotation of promotion) |


How the directives are written
------------------------------

They are written in PlantUML's own syntax, so that the diagram an author writes
is also the picture a reader looks at. A hand-written global diagram renders.

```plantuml
@startuml TRADES
!define PROMOTED

state "稼働中" as running {
  running : buys ; 約定ID ⇸ Buy
  running : cycles ; 基準日 ⇸ Cycle

  state "buys : 約定ID ⇸ Buy" as runningBuys <<promote>> {
    !include local/BUY.puml
  }
  state "cycles : 基準日 ⇸ Cycle" as runningCycles <<promote>> {
    !include local/CYCLE.puml
  }
}

[*] --> running : buys と cycles は空

note as sync1
  sync BOOK : buys(約定ID), cycles(基準日)
end note

note bottom of runningBuys
  constrain BUY?約定ID?数量 ; 数量 は最小取引単位の倍数である
end note
@enduml
```

**`promote`** is a composite state with the `<<promote>>` stereotype, written
inside the state its family moves in. Its title is `<map> : <ID> ⇸ <Type>`: the
map the instances live in, the parameter the instance ID is written as, and the
type of one instance. Its body is one `!include` naming the local diagram. `⇸`
is U+21F8; `->>` is accepted in its place.

The map is named by the **title**, not by the PlantUML alias, because an alias
has to be unique across the diagram and one map may have a block in several
states.

**`sync`** and **`constrain`** are notes whose first non-empty line starts with
the directive's name. A note that starts with neither is there for the picture
alone and is ignored. A directive that points at one block is written as
`note <left|right|top|bottom> of <alias>`, which draws the line; one that spans
several maps is written as a floating `note as <id>`. PlantUML's state diagrams
have no `<id> .. <state>` link, so there is no way to draw a line from a
floating note.

**`assume`** is the core grammar's own note (docs/SYNTAX.md): a floating note
holding one `assume <text>` line states what the diagram assumes of its runs
as a whole, such as "a transaction whose fee keeps being raised is eventually
confirmed". It is not a directive — it narrows no edge, because no guard can
say it — so `csdfpromote` leaves it in place and the expansion carries it, where
`csdflivelockfree` and `csdfrefinement` take it as a hypothesis. It cannot be
anchored to a block: an assumption is about the whole system.
`examples/promote/ATTEMPTS.puml` is the worked example: a stuck transaction
attempt is replaced by one paying a higher fee, the `constrain` says what a
replacement is, and the `assume` note says that the replacements stop, which
is the hypothesis `csdflivelockfree` states its theorem under once the
attempts are hidden.

Everything else in the file is ordinary Composable State Diagram syntax:
hand-written states, the start edge, and hand-written edges such as the ones
that switch operating modes.


Where a family moves
--------------------

The state a `<<promote>>` block is written in is where that family moves. There
is no shorthand: even a one-state diagram writes the state as a composite state
around the block.

To drive one map in several states, write a block in each. Only one of them
carries the `!include`; the others are bodyless:

```plantuml
state "縮退中" as degraded {
  degraded : accounts ; 口座ID ⇸ Account

  state "accounts : 口座ID ⇸ Account" as degradedAccounts <<promote>>
}
```

A second `!include` of the same file would collide with the first on every state
ID, and PlantUML would merge the two boxes into one.

A state that holds the map but has no block for it has that family **frozen**
there, which is reported as `info`. This is deliberate: expanding into every
state that holds the map would quietly write a specification in which trades go
on being booked during maintenance. Every state holds every family's map: the
instances exist whatever the mode, so a state that declares some maps and not
others is refused. The edges that switch modes are hand-written. Every map is
unchanged across them, since an edge of the global diagram cannot write an
instance: write the frame that says so as their post, or leave the post out and
promotion writes it; any other post is refused.


The local-diagram contract
--------------------------

The start state `S₀` means *this instance does not exist*. So:

- `S₀` holds no state variables (error).
- An edge out of `S₀` creates an instance; an edge into it deletes one.
- `S₀` has no self-loop (error): it would be an event of an instance that is not
  there.
- The diagram has no end edge (error). Write completion as a state with no
  transition out of it.
- The post of the local start edge is ignored, and the post of a deletion edge is
  discarded, both with a warning. Neither can say anything: there is no instance
  for them to be about.
- The post of a creation edge alone decides the initial values of the state it
  leads to. Two creation edges with different destinations each decide their own;
  write a shared initial value on both.

Because `!include` drops every local diagram into one namespace, state IDs have
to be unique across the local diagrams of one global diagram (error). Prefix them
per family (`acc`, `buy`, …).

PlantUML-only lines — `title`, `skinparam`, standalone notes — belong inside
`!ifndef PROMOTED … !endif`, with `!define PROMOTED` at the top of the global
diagram, so they show when the local diagram is rendered on its own and not when
it is included. Wrap them in a `CSDF-IGNORE-BEGIN`/`CSDF-IGNORE-END` region as
well, so the core parser skips them too.


Expansion
---------

For a global state `G`, a map `m`, an instance ID written `id`, and a local edge
`S --> T : e a… ; g ; p`, where `a…` are the event's fields:

```
G --> G : e?id a… ; id ∈ dom m ∧ m(id) ∈ 〈S〉 ∧ g ; m' = m ⊕ {id ↦ 〈T〉} ∧ p ∧ FRAME
```

with `id ∉ dom m` and `∪` for a creation, and `⩤` for a deletion. `FRAME` says
that the *other* state variables of `G` are unchanged. The map itself is not
framed: `⊕`, `∪` and `⩤` already say what happens to every key but `id`.

`sync e : m₁(p₁), m₂(p₂)` merges the edges each map contributes to `e` — the
cartesian product, since a guard on one side can pick out any state on the other
— into one edge per combination, over the states where all of them move. The
merged event's fields are the instance IDs, received, in the order the directive names
them, then each side's own fields with duplicate names merged into one. Two
arguments that happen to share a name and mean different things would be merged
wrongly, so name them apart. In the states where the sync applies, the maps do
not expand the event on their own.

`constrain e?q… ; c` conjoins `c` onto the guard of every generated edge that
matches `e` in channel and the directions of its fields, creations and merged edges included.
Matching is by position: the constrain's i-th field is the edge's i-th field,
whatever either calls it. This is
where a condition on another map goes; the local diagram cannot see one.
Hand-written edges are left alone: they already say everything they were meant
to.

A constrain's guard is written like any other clause: in natural language,
or in the language of a `csdfbmc` glue when the diagram is to be checked. A
`formal:` line under a constrain, which an earlier `csdfpromote` carried, is
refused with that advice.

A constrain reaches **every** matching generated edge, in every state. The block
its note points at is a drawing: it tells a reader which family the condition is
about, and it narrows nothing. A parameter of the constrain that no matching
edge takes is a warning — a mistyped name would otherwise be a free variable
sitting in a guard, and nothing else would notice.

`tau` is promoted without an instance ID — an internal event with an argument is
an observable one — and the instance is implicitly existentially quantified.
Syncing `tau` is an error. A promoted `tau` is a self-loop, so
`csdflivelockfree` will not call the result structurally livelock-free and will
emit a predicate obligation instead. That is the right answer, not a defect: the
obligation to discharge is that the number of instances in `〈S〉` goes down.

Predicates are copied **verbatim**. A local variable `v` denotes `m(id).v` after
promotion, but it is not rewritten: a predicate is opaque text, so a textual
substitution could only guess, and a wrong guess in a predicate is invisible.
The bare `v` that survives is readable in context thanks to the origin comment
printed above the edge.


Wording
-------

The generated clauses are builtins, which belong to no natural language.
With the formula promotion, `-template` glosses them in a line comment above
each edge, by Go `text/template` templates named after the builtin words
(`in`, `notin`, `at`, `frame`, `update`, `empty`, `eq`, `ne`), each given the
builtin's terms as `.Args` and, for `at`, its state as `.State`; the
Japanese ones are in `examples/promote/templates/ja.tmpl`. The reader before
it replaced the clauses themselves, with these templates:

| Template   | Fields              | Default                    |
|:-----------|:--------------------|:---------------------------|
| `inDom`    | `Map`, `ID`         | `id ∈ dom m`               |
| `notInDom` | `Map`, `ID`         | `id ∉ dom m`               |
| `atState`  | `Map`, `ID`, `Src`  | `m(id) ∈ 〈S〉`              |
| `update`   | `Map`, `ID`, `Dst`  | `m' = m ⊕ {id ↦ 〈T〉}`      |
| `create`   | `Map`, `ID`, `Dst`  | `m' = m ∪ {id ↦ 〈T〉}`      |
| `delete`   | `Map`, `ID`         | `m' = {id} ⩤ m`            |
| `frame`    | `Var`               | `v' = v`                   |
| `empty`    | `Map`               | `m = ∅`                    |

In the formula promotion the generated clauses are builtins (`@in`,
`@notin`, `@at`, `@frame`, `@empty`, `@eq`, `@ne`), and a template does
not replace one: it renders a gloss of it, printed as a line comment above
the edge, so that the diagram still reads back to the same formulas. Its
templates are named after the builtins, with the builtin's terms as
`.Args` and, for `@at`, its state as `.State`; the names above belong to
`csdfpromote`.

A clause the file does not define keeps its default. Every clause is rendered
once when the file is read, so a template that cannot render is refused rather
than turning into an opaque predicate. A Japanese set ships as
`examples/promote/templates/ja.tmpl`.

What joins the clauses is not wording: a predicate of the expansion is its
clauses joined by the connective ` ∧ ` (docs/SYNTAX.md), the generated ones
first and then the author's, so that a tool reading the expansion gets the
clauses back by splitting on it. A template file that defines `and`, the name
an earlier version gave the connective, is refused.

Every map starts empty, which the `<<promote>>` blocks already say. The
formula promotion accepts a post on the global start edge only if it says the
same, a conjunction of `@empty(m')` over maps, and refuses any other. The
expansion says it on the start edge: after the author's clauses, one `empty`
clause per map the start state holds, in the order the maps were promoted.
The author need not write it. The clauses are carried in the expansion's
structure alone (`Expansion.Start`, `start` in `-format json`, always a
list), with kind `empty` for the generated ones and `local` for the
author's; the start edge's text stays the author's, so a tool that reads the
expansion as a plain diagram reads what it read before. A start-edge clause
that restates the empty start, as older diagrams do, is kept and must agree.
`-only` keeps the `empty` clauses of the kept maps alone, and keeps the
author's start clauses as written, with a warning when the start state held
a map it leaves out, as it does for a hand-written edge. A reader of an
expansion written before `start` existed reads the start edge's text.

The clauses are also kept apart in the expansion's own structure: every edge
that `Expand` generates carries them one by one (`ExpandedEdge.Guard` and
`.Post`), each with its kind — `in_dom`, `not_in_dom`, `at_state`, `update`,
`create`, `delete`, `frame` for the generated clauses, whose meaning a tool
can act on without reading the wording, and `local` or `constrain` for the
author's clauses, which stay opaque text — together with the map, instance ID
and local state the generated clause is about, or the file and line the
author's clause was written on. `csdfpromote -format json` prints them, with
the `<<promote>>` blocks (`promotes`) and the local diagrams they include
(`locals`, by path), since a generated clause names a local state and a local
clause reads local variables that the expansion's text never spells out.


Keeping some families
---------------------

A global diagram with many families expands into a state whose maps hold
every family's instances, and a bounded walk of all of it blows up even when
the question is about a few families that touch the rest lightly.
`-only MAP[,MAP…]` keeps the named families — each written as its map (the
reader before the formula promotion also took its type) — and leaves out the
others:

- The maps of the left-out families go from every state, and so do the edges
  that only they move.
- On an edge a kept family takes together with a left-out one (a `sync`), the
  kept side stays and the event keeps every argument. The left-out side is
  now the environment, which offers the event.
- A guard clause that read a left-out map is not removed in silence. It is
  taken as true and listed: as a warning, and in `-format json` as a clause
  of kind `dropped`, outside the predicate, with the `maps` it read. A
  generated clause about a left-out map and a local clause of a left-out
  family read it by construction. A `constrain` clause is written in a
  language no tool here reads, so whether it reads a left-out map cannot be
  told: it is kept, with a warning saying so. The post clauses and frames of
  a left-out map go with the map.
- Hand-written edges are kept as they were written: a guard and the frame
  that leaves every map unchanged. Their guards are opaque text, so one on a
  state that held a left-out map is reported as a warning: it may still name
  the map.

With every listed clause taken as true, each run of the full expansion
projects onto a run of the restricted one. So a broken invariant and an
event-avoiding cycle over the kept families survive the restriction. A
deadlock does not: one that a left-out family causes by refusing an event, or
by never meeting a listed clause, has no counterpart in the restriction.
docs/CLAUSE_DESIGN.md states this in the stable failures model: a safety
specification the restriction refines, the full expansion with the events
only left-out families take hidden refines too; a refusal result, a
deadlock among them, is no conclusion about the full expansion, and a
counterexample may be spurious, which a tool checking a restriction says. In the formula promotion every clause is a
formula, so whether a `constrain` clause or a hand-written edge's guard
reads a left-out map is told by its free names: such a conjunct is dropped
and reported, like any other, rather than kept with a warning. There a family
is named by its map alone, the type of an instance not being read yet. The
listed clauses are exactly what was assumed; restate the ones that matter as
`assume` notes, or let a checker take them as true and say so (`csdfbmc`
reports them as `dropped_clauses`).


Output
------

Plain Composable State Diagram syntax: the directives, the `!include`s, the
notes but the `assume` ones, and the preprocessor lines are gone, and the
composite states are flattened into ordinary ones. Hand-written states and edges are kept as they
were, and every edge is printed in the canonical order. A line comment above
each generated edge names the local edge it came from; `-no-comments` leaves
them out.

The expanded form is **not** drawn. It is one state with a bundle of self-loops,
which is unreadable and slow to lay out. A repository whose CI expects a sibling
PNG for every `*.puml` should narrow that to the hand-written ones.


Diagnostics
-----------

An `error` leaves the diagram unprinted, so an unsound expansion never reaches
the tools downstream. A `warning` and an `info` go to standard error and let the
expansion through; `-Werror` makes a warning fatal. `-lint-only` checks without
printing. They are reported in source order.

Errors:

| What                                                                    | Why                                                                            |
|:------------------------------------------------------------------------|:--------------------------------------------------------------------------------|
| A `<<promote>>` title that is not `<map> : <ID> ⇸ <Type>`                | There is no map to promote into.                                                |
| A `<<promote>>` body that is not one `!include`                          | A block names one local diagram or none.                                        |
| A `<<promote>>` block outside any state                                  | The state it is written in is where the family moves.                           |
| A map the state holds no variable for                                    | There is nowhere for the instances to live.                                     |
| The same map promoted twice in one state                                 | The second block says nothing the first does not.                               |
| No `!include`, or two, across a map's blocks                             | One local diagram per map; two copies collide on every state ID.                 |
| Blocks of one map that disagree on the ID or the type                    | They are one family, so they are one shape.                                     |
| A composite state nested in another                                      | Only a `<<promote>>` block may be nested.                                       |
| A local `S₀` that holds state variables                                  | `S₀` means that the instance does not exist.                                    |
| A local end edge                                                         | Completion is a state with no transition out of it.                             |
| A local `S₀` self-loop                                                   | It is an event of an instance that is not there.                                |
| A local creation edge on `tau`                                           | Its instance is only existentially quantified, so instances would appear freely. |
| One state ID declared by two local diagrams                              | `!include` drops them into one namespace.                                       |
| A `sync` of `tau`                                                        | An internal event two families take together is an observable one.              |
| A `sync` naming one map twice                                            | One edge moves one key of one map.                                              |
| A `sync` of a map that is not promoted, or of an event a side lacks      | There is nothing to merge.                                                      |
| A `sync` whose maps never move in the same state                         | The event they take together can never happen.                                  |
| A `constrain` no generated edge matches in channel and field directions | It constrains nothing.                                                          |
| A note holding a second directive                                        | One note, one directive.                                                        |
| A directive body that does not parse                                     | —                                                                               |

Warnings:

| What                                                                    | Why                                                                              |
|:------------------------------------------------------------------------|:----------------------------------------------------------------------------------|
| A post on the local start edge, or on a deletion edge                    | It is dropped: there is no instance for it to be about.                           |
| An event two local diagrams have that no `sync` names                    | Two families that name an event alike usually mean to take it together.           |
| A `sync` side whose edges do not all take the same arguments             | The merged event's arity would depend on the combination.                         |
| A `constrain` argument the matching edges do not take                    | A mistyped name would sit in the guard as a free variable.                        |
| A `constrain` guard naming none of its own arguments                     | It says nothing about the instance.                                               |
| A note pointing at a block of a map its `sync` does not name             | The line drawn is misleading.                                                     |
| A `<<promote>>` type the state variable does not mention                 | One of the two is out of date.                                                    |
| A note whose first word is a directive's name in another case            | `Sync` is not `sync`, and it would be drawn as prose.                             |

Info:

| What                                                                    | Why                                                                              |
|:------------------------------------------------------------------------|:----------------------------------------------------------------------------------|
| A state holding a map with no `<<promote>>` block for it                 | The family is frozen there, which may well be meant.                              |
| An `!include` outside a `<<promote>>` block                              | It names no local diagram and is dropped.                                         |

Every other tool refuses a diagram that still holds directives: the core grammar
has no rule for them, and the parse error says to run `csdfpromote` first.
