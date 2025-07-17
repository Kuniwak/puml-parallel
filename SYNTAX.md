Composable State Diagram Format
==========================================
PlantUML の状態遷移図の文法規則のサブセットです。


文法規則
--------
```abnf
diagram = "@startuml" 1*LF 1*stateDecl *edgeDecl "@enduml" LF
stateDecl = "state" SP stateName SP "as" SP stateID 1*LF *(stateID SP ":" SP var LF)
edgeDecl = stateID SP "-->" SP stateID SP ":" SP event SP ";" SP guard SP ";" SP post 1*LF
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

次の記号は ABNF の中核規則である：

* `ALPHA`: ASCII の大文字と小文字
* `DIGIT`: 十進数字
* `DQUOTE`: 二重引用符
* `SP`: 空白
* `LF`: 改行コード

型
--

```go
type ID string
type StateID ID
type EventID ID
type Var ID

type Diagram struct {
    State map[StateID]State 
    Edges []Edge
}

type State struct {
    ID StateID
    Name string
    Vars []Var
}

type Edge struct {
    Src   StateID
    Dst   StateID
    Event Event
    Guard string
    Post  string
}

type Event struct {
    ID EventID
    Params []Var
}
```


意味
----
| 構文要素 | 対応する型 | 意味 |
|:---------|:-----------|:-----|
| `diagram` | `Diagram` | 状態遷移モデルの宣言を表す。 |
| `stateDecl` | `State` | 状態の宣言を表す。 |
| `edgeDecl` | `Edge` | 有向辺の宣言を表す。 |
| `stateName` | `string` | 状態名。先頭と末尾の二重引用符は除去し、エスケープを解除した文字列を表す。 |
| `escape_backslash` | `rune` | `\` を表す。 |
| `escape_dquote`    | `rune` | `"` を表す。 |
| `stateID` | `StateID`  | ID文字列を表す。 |
| `var`     | `Var`      | 変数名を表す。   |
| `event`   | `Event`    | イベントを表す。出現する変数の順番で Params に格納する。 |
| `guard`   | `string`   | ガード条件の自然言語表現を表す。 |
| `post`    | `string`   | 事後条件の自然言語表現を表す。   |
| `id`     | `string`   | ID 文字列を表す。 |
| `unicode_char_except_dquote_and_backslash` | `rune` | 二重引用符とバックスラッシュを除くUnicode文字を表す。 |
| `unicode_char_except_semicolon` | `rune` | セミコロンを除くUnicode文字を表す。 |
