package tracechk

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Result is the verdict on one trace, with the name it is reported under.
type Result struct {
	Name      string
	Trace     []csdf.Event
	Paths     []Path
	Rejection *Rejection
}

// Accepted reports whether the trace is a trace when every guard is true.
func (r Result) Accepted() bool { return r.Rejection == nil }

// Run checks trace against d under m and names the result after the file it
// came from.
func Run(m Match, d *csdf.Diagram, name string, trace []csdf.Event) Result {
	paths, rejection := CheckWith(m, d, trace)
	return Result{Name: name, Trace: trace, Paths: paths, Rejection: rejection}
}

// WriteMarkdown writes the result as a Markdown section. A rejection says where
// the trace stopped being one. An acceptance lists the predicates along every
// path and states, as a prompt, what has to hold of them for the trace to be a
// trace of the diagram in fact.
func WriteMarkdown(w io.Writer, d *csdf.Diagram, r Result) error {
	var sb strings.Builder
	if r.Rejection != nil {
		writeRejection(&sb, r)
	} else {
		writeAcceptance(&sb, d, r)
	}
	_, err := io.WriteString(w, sb.String())
	if err != nil {
		return fmt.Errorf("tracechk.WriteMarkdown: %w", err)
	}
	return nil
}

func writeRejection(sb *strings.Builder, r Result) {
	rej := r.Rejection
	fmt.Fprintf(sb, "# %s: REJECTED\n\n", r.Name)
	fmt.Fprintf(sb, "Event %d of %d, `%s` (row %d of %s), cannot be performed even when every\nguard is true, so this is not a trace of the diagram.\n\n",
		rej.Index+1, len(r.Trace), rej.Event, rej.Index+2, r.Name)
	fmt.Fprintf(sb, "- trace so far: %s\n", joinEvents(r.Trace[:rej.Index]))
	fmt.Fprintf(sb, "- states the diagram may be in before it: %s\n", joinStates(rej.States))
	fmt.Fprintf(sb, "- visible events enabled there: %s\n\n", joinEvents(rej.Enabled))
}

func joinEvents(events []csdf.Event) string {
	if len(events) == 0 {
		return "(none)"
	}
	ss := make([]string, len(events))
	for i, e := range events {
		ss[i] = "`" + string(e) + "`"
	}
	return strings.Join(ss, ", ")
}

func joinStates(states []csdf.StateID) string {
	ss := make([]string, len(states))
	for i, s := range states {
		ss[i] = string(s)
	}
	return strings.Join(ss, ", ")
}

func writeAcceptance(sb *strings.Builder, d *csdf.Diagram, r Result) {
	fmt.Fprintf(sb, "# %s: ACCEPTED when every guard is true\n\n", r.Name)
	fmt.Fprintf(sb, "The diagram performs the trace along %s when every guard is taken as\ntrue. Whether it is a trace of the diagram in fact depends on the\nnatural-language predicates below, which this tool does not evaluate.\n\n",
		plural(len(r.Paths), "path"))
	fmt.Fprintf(sb, "- trace: %s\n\n", joinEvents(r.Trace))
	for i, path := range r.Paths {
		fmt.Fprintf(sb, "## Path %d\n\n", i+1)
		writePath(sb, d, path)
	}
	sb.WriteString(`## Prompt

Decide whether the obligation of at least one path above is satisfiable,
reading every predicate as the natural-language statement in its table.
Answer SATISFIABLE with a witness for every valuation of that path, or
UNSATISFIABLE naming, for every path, the first conjunct that cannot hold
together with the ones before it. A guard is not true because it is written
down: the trace is a trace of the diagram only if the predicates admit it.

`)
}

// step is one row of a path's table: the start edge is step 0, so that the
// valuation x0 is the one its post admits.
type step struct {
	Event      string
	Transition string
	Line       int
	Guard      csdf.Predicate
	Post       csdf.Predicate
	Dst        csdf.StateID
}

func steps(d *csdf.Diagram, path Path) []step {
	ss := []step{{
		Event:      "(start)",
		Transition: fmt.Sprintf("[*] --> %s", d.StartEdge.Dst),
		Line:       d.StartEdge.Line,
		Guard:      csdf.PredicateTrue,
		Post:       d.StartEdge.Post,
		Dst:        d.StartEdge.Dst,
	}}
	for _, e := range path {
		ss = append(ss, step{
			Event:      "`" + string(e.Event) + "`",
			Transition: fmt.Sprintf("%s --> %s", e.Src, e.Dst),
			Line:       e.Line,
			Guard:      e.Guard,
			Post:       e.Post,
			Dst:        e.Dst,
		})
	}
	return ss
}

func writePath(sb *strings.Builder, d *csdf.Diagram, path Path) {
	ss := steps(d, path)

	sb.WriteString("| step | event | transition | guard | post |\n")
	sb.WriteString("|-----:|-------|------------|-------|------|\n")
	for i, s := range ss {
		guard := ""
		if i > 0 {
			guard = cell(s.Guard)
		}
		fmt.Fprintf(sb, "| %d | %s | `%s` (L%d) | %s | %s |\n", i, s.Event, s.Transition, s.Line, guard, cell(s.Post))
	}

	sb.WriteString("\nValuations, one per step, over the variables of the state the step reaches:\n\n")
	for i, s := range ss {
		fmt.Fprintf(sb, "- x%d: %s (%s)\n", i, s.Dst, varsOf(d, s.Dst))
	}

	sb.WriteString("\nObligation:\n\n∃")
	for i := range ss {
		fmt.Fprintf(sb, " x%d", i)
	}
	sb.WriteString(". ")
	conjuncts := make([]string, 0, 2*len(ss))
	for i, s := range ss {
		if i > 0 && !csdf.IsTrue(s.Guard) {
			conjuncts = append(conjuncts, fmt.Sprintf("guard_%d(x%d)", i, i-1))
		}
		if csdf.IsTrue(s.Post) {
			continue
		}
		if i == 0 {
			conjuncts = append(conjuncts, "post_0(x0)")
		} else {
			conjuncts = append(conjuncts, fmt.Sprintf("post_%d(x%d, x%d)", i, i-1, i))
		}
	}
	if len(conjuncts) == 0 {
		conjuncts = append(conjuncts, string(csdf.PredicateTrue))
	}
	sb.WriteString(strings.Join(conjuncts, " ∧ "))
	sb.WriteString(`

where guard_i and post_i are the guard and post of step i. In guard_i the
variables are those of x(i-1); in post_i the unprimed variables are those of
x(i-1) and the primed ones those of xi. A predicate that is exactly ` + "`true`" + `
is left out.

`)
}

// cell renders a predicate in a Markdown table cell, where a bar would end it.
func cell(p csdf.Predicate) string {
	if csdf.IsTrue(p) {
		return string(csdf.PredicateTrue)
	}
	return strings.ReplaceAll(string(p), "|", "\\|")
}

func varsOf(d *csdf.Diagram, id csdf.StateID) string {
	vars := d.States[id].Vars
	if len(vars) == 0 {
		return "no variables"
	}
	names := make([]string, len(vars))
	for i, v := range vars {
		names[i] = string(v.Name)
	}
	return strings.Join(names, ", ")
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
