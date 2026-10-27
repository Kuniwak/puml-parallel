Composable State Diagram Format
===============================
A string representation of composable state transition models. It is a subset of PlantUML's state diagram grammar rules.


Grammar Rules
-------------
```abnf
diagram = "@startuml" inlineTrivia 0*1(diagramName) LF trivia *((stateDecl / assumeDecl) trivia) startEdgeDecl trivia *((edgeDecl / assumeDecl) trivia) 0*1(endEdgeDecl trivia) *(assumeDecl trivia) "@enduml" LF
diagramName = 1*(HTAB / unicode_char)
stateDecl = "state" inlineSeparator stateName inlineSeparator "as" inlineSeparator stateID inlineTrivia LF trivia *(stateVarDecl trivia)
stateVarDecl = stateID inlineTrivia ":" inlineTrivia var inlineTrivia 0*1(";" inlineTrivia varType) LF
startEdgeDecl = "[*]" inlineSeparator "-->" inlineSeparator stateID 0*1(inlineTrivia ":" inlineSeparator post) inlineTrivia LF
edgeDecl = stateID inlineSeparator "-->" inlineSeparator stateID inlineTrivia ":" inlineTrivia event 0*1(inlineTrivia ";" inlineTrivia guard 0*1(inlineTrivia ";" inlineTrivia post)) inlineTrivia LF
endEdgeDecl = stateID inlineSeparator "-->" inlineSeparator "[*]" 0*1(inlineTrivia ":" inlineSeparator guard) inlineTrivia LF
assumeDecl = "note" inlineSeparator "as" inlineSeparator noteID inlineTrivia LF *(inlineTrivia LF) inlineTrivia "assume" inlineSeparator assumption inlineTrivia LF *(inlineTrivia LF) inlineTrivia "end" inlineSeparator "note" inlineTrivia LF
noteID = name
assumption = 1*textElement
stateName = DQUOTE 1*(unicode_char_except_dquote_and_backslash / escape_backslash / escape_dquote) DQUOTE
escape_backslash = "\\"
escape_dquote = "\" DQUOTE
stateID = name
var = name
varType = *textElement
event = channel *(inlineTrivia ("?" / "!") inlineTrivia field)
channel = name
field = name
guard = *textElement
post = *textElement
textElement = unicode_char_except_semicolon / block_comment
name = id ; not starting with a digit, not true, false or null
id = 1*(unicode_letter / unicode_digit / "_")
trivia = *(LF / HTAB / SP / block_comment / line_comment / ignore_region)
inlineTrivia = *(HTAB / SP / block_comment)
inlineSeparator = 1*(HTAB / SP / block_comment)
line_comment = "'" *unicode_char LF
ignore_region = ignore_begin *ignore_line ignore_end
ignore_begin = *(HTAB / SP) "'" *(HTAB / SP) "CSDF-IGNORE-BEGIN" *(HTAB / SP) LF
ignore_end = *(HTAB / SP) "'" *(HTAB / SP) "CSDF-IGNORE-END" *(HTAB / SP) LF
ignore_line = *unicode_char LF
block_comment = "/'" *(LF / unicode_char_except_squote / (%x27 unicode_char_except_slash)) "'/"
unicode_letter = <any character in Unicode general category L>
unicode_digit = <any character in Unicode general category N>
unicode_char = %x20-7F / %x80-10FFFF
unicode_char_except_dquote_and_backslash = %x20-21 / %x23-5B / %x5D-7F / %x80-10FFFF
unicode_char_except_squote = %x20-26 / %x28-7F / %x80-10FFFF
unicode_char_except_slash = %x20-2E / %x30-7F / %x80-10FFFF
unicode_char_except_semicolon = %x20-3A / %x3C-7F / %x80-10FFFF
```

Line comments are accepted between declarations and state-variable lines.
Block comments are accepted wherever horizontal whitespace is accepted, including
inside `varType`, `event`, `guard`, and `post`. Comments are discarded while parsing.
A line comment whose trimmed text is exactly `CSDF-IGNORE-BEGIN` opens an ignore region
(taking precedence over a plain `line_comment`) that extends through the next line comment
whose trimmed text is exactly `CSDF-IGNORE-END`. `ignore_line` matches any single line that
does not match `ignore_end`, so the region closes at the *first* `CSDF-IGNORE-END` marker
rather than the last (the rule is not greedy). Every line in between, along with both marker
lines, is discarded. Because the markers are themselves PlantUML line comments, the
wrapped content (for example Graphviz directives such as `left to right direction`) is still
rendered by PlantUML while CSDF ignores it. An unterminated ignore region is a parse error.
Comment delimiters inside a state name are ordinary text. A clause language's quotes are not the tools': a comment delimiter, `;` or ` ∧ ` inside one is read as anywhere else.
An event is a channel followed by its fields, in the CSP style: `c?x` receives `x`
(the environment chooses its value, from those the guard accepts) and `c!y` sends
the value of `y`, which the step already binds: a state variable before the step, or
a field the same event receives (`c?x!x`). A sent field ranges over nothing, and a
constant is sent through a state variable that holds it. `tau` is the internal event, and carries no fields. An event names its fields and carries
no value and no expression; anything about a value is said by the guard and the
post, which read a field by its name. Whitespace and comments around `?` and `!`
are removed, so `c ? x` is `c?x`. Composition synchronises on channels, and
`csdfpromote` matches `sync` and `constrain` by the channel, and hiding hides
channels: a hidden field is a value the `tau` chooses, which the clauses still
read by its name.

Identifiers are not restricted to ASCII: any Unicode letter or digit is accepted, with `_`, so a
state ID or a state variable may be written in Japanese (`承認記録' の承認者 = 操作者`).
PlantUML accepts such identifiers too, which keeps CSDF a subset of PlantUML.
`-` is not part of an identifier, so that a name followed by `-` in a clause
reads as the name and then the text (`$x-1`).

Everything a diagram names is a `name`: a state ID, a state variable, a
channel, a field, a note ID, the names in a clause, and the names in a
global diagram's directives (docs/PROMOTION.md) and a composition
expression's channels. A name is an identifier that does not start with a
digit and is none of the reserved words `true`, `false` and `null`, which
a clause reads as literals; so a name means the same wherever it is
written, and a field or a variable is never mistaken for a value. A diagram
using such a name is refused, the error naming the reserved word or the
leading digit. A digit may still follow the first character (`s0`, `item2`).

An `assumeDecl` is PlantUML's floating note holding one `assume` line. It states
what the diagram assumes of its runs as a whole — a fairness assumption such as
"a transaction whose fee keeps being raised is eventually confirmed" — which no
guard can state, because a guard is about one step. The assumption is opaque text
like a predicate; `csdflivelockfree` and `csdfrefinement` carry it as a hypothesis
of the obligation, and `csdfparallel` gives a composition the assumptions of both
sides. A note is the one PlantUML-only element the grammar admits, so that the
picture shows the assumption next to the diagram; a note for the picture alone,
and an anchored note (`note left of s`), still go in a `CSDF-IGNORE` region.

A `guard` or a `post` is a *clause*: a conjunction of conjuncts joined by
` ∧ ` (U+2227 with one space on each side), each a builtin such as `@eq(a, b)`
or an atom, text with holes, written in whatever language its reader takes
(natural language, or a formal language a `csdfbmc` glue reads). Its syntax
is in [Clauses](#clauses) below. A `guard` or `post` that is omitted has no
conjunct, and holds. The structure of a diagram reads a clause up to the `;`
or the end of the line that ends it; `\;` writes a `;` inside one.

The clause syntax of earlier releases is refused: `@opaque` and the renaming
`@[from := to] clause` are unknown tool words. A clause not formalised yet is
an atom whose holes are what it reads (README, bounded model checking).

The following symbols are ABNF core rules:

* `DQUOTE`: Double quote
* `SP`: Space
* `LF`: Line feed


Types
-----

```go
package example

type ID string
type StateID ID
type Event string
type Var ID

type StateVar struct {
	Name Var
	Type string
}

type Diagram struct {
	Name      string
	States    map[StateID]State
	StartEdge StartEdge
	Edges     []Edge
	EndEdge   *EndEdge
}

type State struct {
	ID   StateID
	Name string
	Vars []StateVar
}

type StartEdge struct {
	Dst  StateID
	Post string
}

type Edge struct {
	Src   StateID
	Dst   StateID
	Event Event
	Guard string
	Post  string
}

type EndEdge struct {
	Src   StateID
	Guard string
}

type Assumption struct {
	ID   string
	Text string
}
```


Semantics
---------
| Syntax Element                             | Corresponding Type | Meaning                                                                                                                                                                  |
|:-------------------------------------------|:-------------------|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `diagram`                                  | `Diagram`          | Represents a declaration of a state transition model.                                                                                                                    |
| `diagramName`                              | `string`           | Optional PlantUML diagram name written on the `@startuml` line. Any text up to the end of the line is accepted, quoted or not, and comment delimiters within it are plain text. Leading and trailing whitespace is removed. It carries no meaning, but it is retained so that printing a parsed diagram reproduces it. |
| `stateDecl`                                | `State`            | Represents a state declaration.                                                                                                                                          |
| `stateVarDecl`                             | `StateVar`         | Represents a state variable name and its optional type.                                                                                                                   |
| `startEdgeDecl`                            | `StartEdge`        | Represents a declaration of transition to the initial state.                                                                                                             |
| `edgeDecl`                                 | `Edge`             | Represents a declaration of a directed edge.                                                                                                                             |
| `endEdgeDecl`                              | `EndEdge`          | Represents a declaration of transition to the end state.                                                                                                                 |
| `assumeDecl`                               | `Assumption`       | Represents an assumption about the runs of the diagram as a whole, written as a floating note holding one `assume` line. `noteID` names it; `assumption` is its natural-language text. |
| `stateName`                                | `string`           | State name. Represents a string with leading and trailing double quotes removed and escapes resolved.                                                                    |
| `escape_backslash`                         | `rune`             | Represents `\`.                                                                                                                                                          |
| `escape_dquote`                            | `rune`             | Represents `"`.                                                                                                                                                          |
| `stateID`                                  | `StateID`          | Represents an ID string.                                                                                                                                                 |
| `var`                                      | `Var`              | Represents a variable name.                                                                                                                                              |
| `varType`                                  | `string`           | Represents an optional state-variable type. Leading and trailing whitespace is removed.                                                                                   |
| `event`                                    | `Event`            | An event: a channel and its `?` (received) and `!` (sent) fields, written without whitespace. `Event.Channel` and `Event.Fields` take it apart, and csdfparse writes them as `channel` and `fields` next to `event`. When it is exactly `tau`, it is an internal transition. |
| `guard`                                    | `string`           | Represents a natural language expression of guard conditions.                                                                                                            |
| `post`                                     | `string`           | Represents a natural language expression of post-conditions.                                                                                                             |
| `id`                                       | `string`           | Represents an ID string.                                                                                                                                                 |
| `trivia`                                   | N/A                | Whitespace and comments accepted between declarations.                                                                                                                   |
| `inlineTrivia`                             | N/A                | Horizontal whitespace and block comments accepted inside declarations.                                                                                                  |
| `line_comment`                             | N/A                | PlantUML line comment beginning with `'`. It is not retained in the AST.                                                                                                 |
| `ignore_region`                            | N/A                | Non-interpreted region delimited by `CSDF-IGNORE-BEGIN`/`CSDF-IGNORE-END` line comments, holding PlantUML-only directives. It is discarded while parsing.                  |
| `block_comment`                            | N/A                | PlantUML block comment delimited by `/'` and `'/`. It is not retained in the AST.                                                                                         |
| `unicode_char_except_dquote_and_backslash` | `rune`             | Represents Unicode characters except double quotes and backslashes.                                                                                                      |
| `unicode_char_except_semicolon`            | `rune`             | Represents Unicode characters except semicolons.                                                                                                                         |


Clauses
-------
The surface syntax of the clause representation in `csdf/clause`, which
follows `docs/CLAUSE_DESIGN.md`: every tool reads guards and posts by it.

```abnf
clause   = [conjunct *(" ∧ " conjunct)]
conjunct = builtin / exists / atom
builtin  = "@eq(" term ", " term ")"        ; equal
         / "@ne(" term ", " term ")"        ; differ
         / "@in(" term ", " term ")"        ; key in the domain of a map
         / "@notin(" term ", " term ")"     ; key not in the domain of a map
         / "@update(" term ", " term ", " term ", " term ")"  ; after, before, key, value
         / "@frame(" term ", " term *(", " term) ")"          ; after, before, keys
         / "@at(" term ", " id ")"          ; instance at a state of its local diagram
         / "@empty(" term ")"               ; a map without entries
exists   = "@exists(" id " : " id ") (" clause ")" ; written by tools only
atom     = 1*(text / "$" holeName ["'"] / "${" term "}")
term     = literal
         / name ["'"] *("[" term "]" / "." name) ["'"]
         / name "(" [term *(", " term)] ")" ; an environment function
name     = <name, as in the diagram grammar above>
literal  = <a JSON value (RFC 8259) other than null>
holeName = (unicode_letter / "_") *(unicode_letter / unicode_digit / "_")
```

- A literal is a JSON value: an integer (`0`, `-7`, of any size), a float
  (a number with a point or an exponent: `1.0`, `1e2`), a string
  (`"在庫"`, with JSON's escapes), `true`, `false`, an array (`[1, "a"]`,
  ordered) or an object (`{"a": 1}`, a record from string keys). `null` is
  no value. A float is an IEEE 754 binary64: it is rounded to the nearest
  one, and one beyond the largest (`1e400`) and NaN or the infinities,
  which JSON cannot write, are refused. Negative zero (`-0.0`, `-0e5`, or a
  negative number rounded to zero) is read as `0.0`, and `-0` as `0`.
- Literals are equal when they are the same value: `1` and `1.0` are two
  values, an integer and a float, and `"1"` and `true` are neither; two
  floats are equal when they are the same number. An object's key order and
  the spaces in an array or object do not matter. The tools print a literal
  in its canonical form: no spaces, an object's keys sorted, a float
  shortest with a point or an exponent, and a string with its characters as
  they are but `"`, `\`, control characters, `;` (written `\u003b`, since it
  ends a clause on an edge) and `/` (written `\/`, since `/'` begins a comment).
- A literal has no value after the step and is no path: `0'`, `{"a": 1}.a`
  and `[1][0]` are refused. A literal may be a key (`m["k"]`), an argument
  (`f(1)`) or a hole (`${[1, 2]}`).
- `true`, `false` and `null` are reserved, and a name does not start with a
  digit, so `@eq(n', 0)` compares `n` with the integer 0. A clause using one
  as a variable, a field, a function or a bound name is refused.
- A free name must be bound: before the step a state variable of the source
  or a field of the event, after it (primed) a state variable of the target.
  A name nothing binds is refused, never read as a parameter.

- A `$name` hole ends where its name ends (`id`): whatever follows, `[`,
  `.`, `-` or anything else, is the atom's language. One space after any
  hole, `$name` or `${term}`, belongs to the hole when the next character
  could continue a name or is another space, so `$在庫 が` and `${在庫} が`
  read the hole `在庫` and the text `が`, and `$x  が` the text ` が`.
- `${term}` holds any term, for indexing, fields, environment functions
  and literals (`${[1, 2]}`). A trailing `'` primes the
  variable the path starts from: `${m[k].f'}` is the entry at `k` of `m`
  after the step. An environment function has no value after the step.
- Only the variable a term starts from may be primed. A key, `m[k']`, or an
  argument of an environment function, `f(x')`, read after the step is
  refused when the atom is given to a glue: an argument is a value before
  the step or after it, never a mix.
- A glue receives the space that ended a hole back in the text after it,
  when that text goes on with a character that could continue a name, so
  that its own reading of the hole does not run into the text. `${x}` and
  `$x` are the same hole.
- `$` not followed by a letter, `_` or `{` is text, so `$100` is text.
- A builtin is a whole conjunct. Text that starts with `@` is written `\@`.
- `@exists(v : T) (…)` binds `v`, ranging over the type `T`, in its body; the innermost binder of a name
  wins, and a bound variable has no value after the step. Authors do not
  write it (`docs/CLAUSE_DESIGN.md`); tools do. Inside a body, parentheses
  in text nest unless escaped.
- A clause that reads an entry a map does not have, `m[x]` with `x` not in
  the domain of `m`, is false.
- In text, `\` before one of `$ ∧ ; @ \ ( )` is that character, and before
  anything else is itself.
- A variable bound by `@exists` is a de Bruijn index inside the tools. The
  tools print it with the name the tool that made it gave (its hint), unless
  that name is free in the clause or bound by an enclosing `@exists`; then
  with the least `name2`, `name3`, … that is neither (`name_2` after a name
  ending in a digit). A hint that is no identifier is an error, never
  replaced.
- Printing and parsing back gives the same clause: the tools write
  `${name}` where `$name` would read differently (before a space and a
  character that could continue a name, or before a `'` that is text), one
  space between `$name` and a character that could continue a name (a
  letter, a digit or `_`), and a backslash before what would otherwise be
  syntax.


Diagrams of formulas (draft)
----------------------------
**Draft for v3.0.0 (#76); read by `csdf/coreio`, which no tool calls yet.**

A diagram whose clauses are in the syntax above has the structure of the
grammar at the top of this document: states, their variables, a start edge,
edges and one end edge. Each guard and post is a clause. A clause written by
an author holds no `@exists` (`coreio.ReadAuthored`); one written by a tool
may (`coreio.Read`). A variable's type, written after `;` on its line, is
not read yet: a variable has the type named like it (`docs/CLAUSE_DESIGN.md`).
An assume note is refused for now.

A global diagram that promotes families is written as:

```
@startuml
state "稼働中" as running {
  state "locations : l ⇸ Location" as runningLocations <<promote>> {
    !include local/LOCATION.puml
  }
  state "shipments : s ⇸ Shipment" as runningShipments <<promote>> {
    !include local/SHIPMENT.puml
  }
}
[*] --> running

note as move
  sync MOVE : shipments(s).SHIP, locations(from).DEPART, locations(to).ARRIVE ; ${locations[from].在庫} > 1
end note
@enduml
```

- A composite state holds a `<<promote>>` block per family, titled
  `map : ID ⇸ Type` (or `->>`) as in docs/PROMOTION.md: the map that holds
  the instances, the name of an instance's ID, and the type of one instance,
  which is not read yet. The block includes the family's local diagram, read
  relative to the global file. The composite state may declare the maps it
  holds, as `running : buys ; 約定ID ⇸ Buy`. Its states are the modes
  below, and its edges are the start edge and the hand-written edges between
  modes.
- A local diagram keeps the contract of docs/PROMOTION.md: its start state
  means that the instance does not exist, an edge out of it creates an
  instance and an edge into it deletes one.
- A note, floating or anchored to a block, whose first line is a directive
  holds that one directive; the lines after it are prose. A note whose first
  line is no directive is for the picture and is skipped. The directive is:
  - `sync EVENT : target, … [; clause]` — the instances that take EVENT
    together. A target is `map(id)`, which takes the local event named
    EVENT, or `map(id).event`, which takes `event`; two targets of one map
    bind two instances, whose IDs the expansion says differ. The clause after
    `;` is the sync's guard: it reads the maps and the targets' IDs, and
    what it reads of a bound instance is carried by the event.
- `CSDF-IGNORE` regions, `'` comments and preprocessor lines such as
  `!define` are for the picture and are skipped.
- A second `sync` of one event, a second block of one map, and
  `!include_many` or `!includesub` are refused.

- `constrain EVENT ; guard` is read as docs/PROMOTION.md says: its guard
  is conjoined onto every generated edge matching the event in channel and
  field directions, and may read any map.

- Several global states are modes. A mode holds the maps it declares
  (`mode : map ; …`) or has a `<<promote>>` block for, and every mode holds
  every family's map. Exactly one block of a map includes its local diagram;
  the others are bodyless. A mode holding a map without a block for it has
  the family frozen there.
- A mode may declare variables that are no family's map (`mode : v`, inside
  a composite mode or under a plain one): the controller's own variables.
- A hand-written edge between modes, `src --> dst : EVENT?f… ; guard ; post`,
  reads the maps as they are, its source's controller variables and its own
  received fields; its post writes its target's controller variables, and
  primes a map only as its frame `m' = m`, which promotion writes if left
  out. It has no `!` field and no primed read in its guard, and it is not on
  a channel that a generated edge of the global diagram carries. There is
  no end edge.
- The post of the global start edge says `@empty(m')` of maps, which start
  empty anyway, and gives the start mode's controller variables their
  values; any other post is refused.
- A floating `assume` note is carried into the expansion; an anchored one is
  refused.

`csdfpromote` reads a global diagram with this reader and prints the
expansion core.Promote gives, restricted by `-only` and glossed by
`-template` ([PROMOTION.md](./PROMOTION.md)).


Global Diagram Format
---------------------
The grammar above is what every tool reads. `csdfpromote` reads an upper-compatible
one on top of it, in which a diagram may also declare a promotion. The directives
are spelled in PlantUML's own syntax, so that a hand-written global diagram still
renders; `csdfpromote` lifts them out and prints a diagram of the grammar above.
Refer to PROMOTION.md for what they mean.

```abnf
globalDiagram   = "@startuml" inlineTrivia 0*1(diagramName) LF trivia
                  *((stateDecl / compositeState / preprocessorLine / noteBlock) trivia)
                  startEdgeDecl trivia
                  *((edgeDecl / noteBlock / preprocessorLine) trivia)
                  0*1(endEdgeDecl trivia)
                  "@enduml" LF

compositeState  = "state" inlineSeparator stateName inlineSeparator "as" inlineSeparator stateID
                  inlineTrivia "{" LF trivia
                  *((stateVarDecl / promoteBlock / preprocessorLine) trivia)
                  "}" LF

promoteBlock    = "state" inlineSeparator promoteTitle inlineSeparator "as" inlineSeparator stateID
                  inlineSeparator "<<promote>>"
                  0*1(inlineTrivia "{" LF trivia includeLine trivia "}") inlineTrivia LF
promoteTitle    = DQUOTE var inlineTrivia ":" inlineTrivia param inlineTrivia ("⇸" / "->>") inlineTrivia typeName DQUOTE
includeLine     = "!include" inlineSeparator path inlineTrivia LF

noteBlock       = "note" inlineSeparator ("as" inlineSeparator noteID / noteAnchor) inlineTrivia LF
                  *(noteLine LF)
                  inlineTrivia "end" inlineSeparator "note" inlineTrivia LF
noteAnchor      = ("left" / "right" / "top" / "bottom") inlineSeparator "of" inlineSeparator stateID
noteLine        = *unicode_char_except_LF
noteID          = name

syncBody        = "sync" inlineSeparator eventName inlineTrivia ":" inlineTrivia mapRef *(inlineTrivia "," inlineTrivia mapRef)
constrainBody   = "constrain" inlineSeparator event inlineTrivia ";" inlineTrivia guard
mapRef          = var inlineTrivia "(" inlineTrivia param inlineTrivia ")"

preprocessorLine = "!" *unicode_char_except_LF LF
path            = 1*(unicode_char_except_space) / DQUOTE 1*(unicode_char_except_dquote) DQUOTE
typeName        = 1*(unicode_char_except_space)
param           = name
eventName       = channel
```

| Syntax Element     | Corresponding Type | Meaning                                                                                                                                                                                 |
|:-------------------|:-------------------|:------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `globalDiagram`    | `GlobalDiagram`    | A diagram together with the directives lifted out of it. What is left is a `Diagram` of the grammar above, with the composite states flattened into ordinary ones.                        |
| `compositeState`   | `State`            | An ordinary state whose braces hold its state variables and the `<<promote>>` blocks of the families that move in it. Nesting anything else is an error.                                   |
| `promoteBlock`     | `Promote`          | One family of instances. The title names the map, the parameter the instance ID is written as, and the type of one instance; the alias is a PlantUML identifier and carries no meaning. A block with no body only says that the family moves in that state too. |
| `includeLine`      | `string`           | The local diagram of the family. Exactly one block per map carries it.                                                                                                                    |
| `noteBlock`        | `Sync`/`Constrain` | A directive when its first non-empty line starts with `sync` or `constrain`; a note for the reader otherwise, and then discarded.                                                          |
| `noteAnchor`       | `Anchor`           | The block the note points at, which is drawn as a line. A floating `note as <id>` points at nothing: PlantUML's state diagrams have no `<id> .. <state>` link.                              |
| `preprocessorLine` | N/A                | A PlantUML preprocessor line such as `!define` or `!ifndef`. It is discarded, and it is how a local diagram hides its PlantUML-only lines when it is included.                              |
