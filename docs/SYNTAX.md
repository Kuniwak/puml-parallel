Composable State Diagram Format
===============================
A string representation of composable state transition models. It is a subset of PlantUML's state diagram grammar rules.


Grammar Rules
-------------
```abnf
diagram = "@startuml" 1*LF 1*stateDecl startEdgeDecl *edgeDecl "@enduml" LF
stateDecl = "state" SP stateName SP "as" SP stateID 1*LF *(stateID *SP ":" *SP var LF)
startEdgeDecl = "[*]" SP "-->" SP stateID 0*1(*SP ":" SP post) *SP 1*LF
edgeDecl = stateID SP "-->" SP stateID *SP ":" *SP event 0*1(*SP ";" *SP guard 0*1(*SP ";" *SP post)) *SP 1*LF
stateName = DQUOTE 1*(unicode_char_except_dquote_and_backslash / escape_backslash / escape_dquote) DQUOTE
escape_backslash = "\\"
escape_dquote = "\" DQUOTE
stateID = id
var = id
eventID = id
event = eventID 0*1("(" var 0*1("," SP var) ")")
guard = *unicode_char_except_semicolon
post = *unicode_char_except_semicolon
id = 1*(ALPHA / DIGIT / "_" / "-")
unicode_char_except_dquote_and_backslash = %x21 / %x23-5B / %x5D-7F / %x80-10FFFF
unicode_char_except_semicolon = %x21-3A / %3C-7F / %x80-10FFFF
```

The following symbols are ABNF core rules:

* `ALPHA`: ASCII uppercase and lowercase letters
* `DIGIT`: Decimal digits
* `DQUOTE`: Double quote
* `SP`: Space
* `LF`: Line feed


Types
-----

```go
package example

type ID string
type StateID ID
type EventID ID
type Var ID

type Diagram struct {
	State     map[StateID]State
	StartEdge StartEdge
	Edges     []Edge
}

type State struct {
	ID   StateID
	Name string
	Vars []Var
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

type Event struct {
	ID     EventID
	Params []Var
}
```


Semantics
---------
| Syntax Element                             | Corresponding Type | Meaning                                                                                                                                                                  |
|:-------------------------------------------|:-------------------|:-------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `diagram`                                  | `Diagram`          | Represents a declaration of a state transition model.                                                                                                                    |
| `stateDecl`                                | `State`            | Represents a state declaration.                                                                                                                                          |
| `startEdgeDecl`                            | `Edge`             | Represents a declaration of transition to the initial state.                                                                                                             |
| `edgeDecl`                                 | `Edge`             | Represents a declaration of a directed edge.                                                                                                                             |
| `endEdgeDecl`                              | `EndEdge`          | Represents a declaration of transition to the end state.                                                                                                                 |
| `stateName`                                | `string`           | State name. Represents a string with leading and trailing double quotes removed and escapes resolved.                                                                    |
| `escape_backslash`                         | `rune`             | Represents `\`.                                                                                                                                                          |
| `escape_dquote`                            | `rune`             | Represents `"`.                                                                                                                                                          |
| `stateID`                                  | `StateID`          | Represents an ID string.                                                                                                                                                 |
| `var`                                      | `Var`              | Represents a variable name.                                                                                                                                              |
| `event`                                    | `Event`            | Represents an event. Variables are stored in Params in the order they appear. When the event ID is `tau`, it is an internal transition. Therefore, Params must be empty. |
| `guard`                                    | `string`           | Represents a natural language expression of guard conditions.                                                                                                            |
| `post`                                     | `string`           | Represents a natural language expression of post-conditions.                                                                                                             |
| `id`                                       | `string`           | Represents an ID string.                                                                                                                                                 |
| `unicode_char_except_dquote_and_backslash` | `rune`             | Represents Unicode characters except double quotes and backslashes.                                                                                                      |
| `unicode_char_except_semicolon`            | `rune`             | Represents Unicode characters except semicolons.                                                                                                                         |
