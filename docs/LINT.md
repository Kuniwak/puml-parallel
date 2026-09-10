# Lint

`csdflint` checks Composable State Diagrams and writes what it finds as TSV.

It has two readers at once, and the output is shaped for both:

- a person or a CI job, which wants a verdict: the exit status is 1 when
  anything is an ERROR and 0 otherwise;
- an AI asked to review a diagram it wrote itself, which wants to be told what
  to look at: a message says what is suspicious *and what would settle it*, so
  that a finding can be pasted into a review prompt as it stands.

## Output

One header row, then one row per finding, written with `encoding/csv` at
`Comma = '\t'`:

| Column | Meaning |
|---|---|
| `file` | The file the finding is in; `-` for standard input. |
| `start_line` | 1-based first line of the span; 0 when the finding is about the file as a whole. |
| `end_line` | 1-based last line of the span. |
| `rule` | The rule that produced it, e.g. `self-transition-divergence`. |
| `severity` | `ERROR`, `WARN`, `HINT` or `STYLE_PROBLEM`. |
| `message` | What is wrong, or what to check. Never holds a tab, a newline or a double quote, so a finding is always one line. |

Findings are ordered by file (the order of the arguments), then by line, then by
rule.

## Severities

| Severity | Meaning | Fails the run |
|---|---|---|
| `ERROR` | Definitely wrong. | yes |
| `WARN` | Probably not what the author meant. | no |
| `HINT` | Suspicious, but may well be intended. | no |
| `STYLE_PROBLEM` | About how the source is written, not about what it means. | no |

A `HINT` is a question, not a verdict: the predicates of a CSDF are natural
language and so opaque, and no rule can decide what they mean. The rule points
at the shape where the question matters and says how to answer it.

## Rules

`csdflint -list-rules` prints this list from the code.

### `syntax-error` (ERROR)

The file is not a Composable State Diagram the CSDF grammar (docs/SYNTAX.md) can
read. Nothing else can be checked about such a file, so it is the only finding
it gets.

### `self-transition-divergence` (HINT)

A transition from a state back to itself. Such an edge can be taken any number
of times in a row, so the diagram admits the infinite trace
`prefix -> A -> A -> ...`, in which the event repeats without bound and the
suffix after `A` never begins.

A diagram written by an AI has far more of these than one written by hand,
because a state that should have been split is easier to write as a loop. The
loop is nonetheless often what the author meant — polling, retrying, adding one
more item — so the finding asks for the trace to be read rather than claiming
the edge is wrong. A `tau` self-transition is the sharper case: nothing
observable ever happens again, which is divergence rather than a loop of work,
and `csdflivelockfree` is the tool that settles it.

One shape trips this rule by design: the expansion of a promotion
(docs/PROMOTION.md) *is* one state with many self-loops, because the local
control state has been absorbed into a state variable. Lint the local diagrams
and the hand-written global diagram, not the output of `csdfpromote`.

### `self-transition-post-breaks-guard` (HINT)

A self-transition that has both a guard and a post. A self-transition says the
system is in the same situation after the event as before it. If the post makes
the guard false, that is not so — the edge is enabled once and then not again —
and the two situations are two states that were written as one.

Answering it is mechanical even though it cannot be automated: assume the guard,
apply the post, and evaluate the guard again in the resulting state.

## Adding a rule

The rules are a slice, so adding a check is writing a `lint.Rule` in
`csdf/lint` and putting it in `lint.DefaultRules`. Nothing else in the package
knows what the rules are.

```go
type Rule interface {
	ID() string
	Description() string
	Check(in *Input) []Finding
}
```

`Input` carries the file name, the source text, and the diagram — parsed once
for every rule. `Input.Diagram` is nil exactly when `Input.ParseErr` is not, so
a rule that reads transitions returns nothing for a file that does not parse;
`syntax-error` is the rule that speaks for that file.

Two things a new rule owes its readers:

- **A message that says what would settle the question.** Anything below ERROR
  is read as a review prompt, and "this looks odd" is not one.
- **A message with no tab, newline or double quote in it.** `csv.Writer` quotes
  such a field, and a quoted field may span lines; the table would stop being
  readable with `cut(1)` or at a glance. `TestMessagesNeedNoQuoting` guards this.
