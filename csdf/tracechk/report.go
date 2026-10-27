package tracechk

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/Kuniwak/puml-parallel/v3/csdf/logic"
	"github.com/Kuniwak/puml-parallel/v3/csdf/obligation"
)

// WriteMarkdown writes the result as a Markdown section. A rejection says
// where the trace stopped being one and along which paths, and what would have
// to fail for the refusal not to be real. An acceptance lists, for every event
// of the trace and every path performing the prefix before it, the predicates
// along the path and the obligation that the state reached is unstable or can
// perform the event; it ends with a prompt asking a reader to decide them.
func WriteMarkdown(w io.Writer, d *core.Diagram, r Result) error {
	var sb strings.Builder
	var err error
	if r.Rejection != nil {
		err = writeRejection(&sb, d, r)
	} else {
		err = writeAcceptance(&sb, d, r)
	}
	if err != nil {
		return fmt.Errorf("tracechk.WriteMarkdown: %w", err)
	}
	_, err = io.WriteString(w, sb.String())
	if err != nil {
		return fmt.Errorf("tracechk.WriteMarkdown: %w", err)
	}
	return nil
}

func writeRejection(sb *strings.Builder, d *core.Diagram, r Result) error {
	rej := r.Rejection
	fmt.Fprintf(sb, "# %s: REJECTED\n\n", r.Trace.Name)
	fmt.Fprintf(sb, "After %s (%d of %d events) the diagram may be in %s.\n",
		prefixText(r.Trace.Events[:rej.Index]), rej.Index, len(r.Trace.Events), joinStates(rej.States))
	if len(rej.Refusing) > 0 {
		fmt.Fprintf(sb, "%s and has no edge for `%s`, so the diagram may refuse `%s` there even\nwhen every guard is true: this is not a trace of the diagram.\n\n",
			stableText(rej.Refusing), rej.Event, rej.Event)
		fmt.Fprintf(sb, "- visible events enabled at the refusing states: %s\n\n", joinEvents(rej.Enabled))
		sb.WriteString("The refusal is real unless every path below is infeasible, that is, unless\nthe clauses along it cannot all hold.\n\n")
	} else {
		fmt.Fprintf(sb, "None of them is stable and none can perform `%s`, so the diagram diverges\ninstead of performing it even when every guard is true: this is not a trace\nof the diagram.\n\n", rej.Event)
		sb.WriteString("The divergence is real unless every path below is infeasible, that is, unless\nthe clauses along it cannot all hold.\n\n")
	}
	for i, p := range rej.Paths {
		fmt.Fprintf(sb, "## Path %d: reaches %s\n\n", i+1, p.Dst)
		if err := writePathTable(sb, d, p.Path); err != nil {
			return err
		}
		r := run{}
		along, err := r.pathClauses(d, p.Path)
		if err != nil {
			return err
		}
		feasible, err := r.some(along)
		if err != nil {
			return err
		}
		infeasible := logic.Not(feasible)
		fmt.Fprintf(sb, "Infeasible iff: %s\n\n", notation.Spell(infeasible))
	}
	return nil
}

func writeAcceptance(sb *strings.Builder, d *core.Diagram, r Result) error {
	fmt.Fprintf(sb, "# %s: ACCEPTED when every guard is true\n\n", r.Trace.Name)
	sb.WriteString(`After every prefix of the trace, every stable state the diagram may be in can
perform the next event when every guard is taken as true. Whether it can in
fact depends on the clauses below, whose atoms this tool does not evaluate. A
path never revisits a state within one run of ` + "`tau`" + ` edges, so paths going
round a ` + "`tau`" + ` cycle, and their obligations, are not listed.

`)
	fmt.Fprintf(sb, "- trace: %s\n", joinEvents(r.Trace.Events))
	at0 := run{}
	start, err := at0.startClause(d)
	if err != nil {
		return err
	}
	admits, err := at0.some(start)
	if err != nil {
		return err
	}
	fmt.Fprintf(sb, "- the start edge `[*] --> %s`%s must admit a valuation: %s\n\n", d.Init, line(d.InitLine), notation.Spell(admits))

	for _, step := range r.Steps {
		fmt.Fprintf(sb, "## Event %d: `%s` after %s\n\n", step.Index+1, step.Event, prefixText(r.Trace.Events[:step.Index]))
		for j, p := range step.Paths {
			fmt.Fprintf(sb, "### Path %d.%d: reaches %s\n\n", step.Index+1, j+1, p.Dst)
			if err := writePathTable(sb, d, p.Path); err != nil {
				return err
			}
			if err := writeObligation(sb, d, p); err != nil {
				return err
			}
		}
	}

	sb.WriteString(`## Prompt

Decide whether every obligation above holds, reading each atom as the
statement its text makes of its holes. Answer HOLDS, or FAILS naming the
first obligation that does not hold and valuations that satisfy its premise
but not its conclusion. A guard is not true because it is written down: the
trace is a trace of the diagram only if the clauses make every obligation
hold. FAILS settles that it is not; HOLDS settles it only for the listed
paths, since paths going round a ` + "`tau`" + ` cycle are not listed.

`)
	return nil
}

// writePathTable tabulates the start edge as step 0 and each edge of the
// path as the following steps, then names the values each step reaches: the
// variable v of the state after step i is v@i, and a field k step i
// receives is k#i.
func writePathTable(sb *strings.Builder, d *core.Diagram, path Path) error {
	sb.WriteString("| step | event | transition | guard | post |\n")
	sb.WriteString("|-----:|-------|------------|-------|------|\n")
	post, err := cell(d.InitPost)
	if err != nil {
		return err
	}
	fmt.Fprintf(sb, "| 0 | (start) | `[*] --> %s`%s |  | %s |\n", d.Init, line(d.InitLine), post)
	for i, e := range path {
		row, err := edgeCells(e)
		if err != nil {
			return err
		}
		fmt.Fprintf(sb, "| %d | `%s` | %s |\n", i+1, e.Event.Label(), row)
	}
	sb.WriteString("\n")

	fmt.Fprintf(sb, "- after step 0: %s (%s)\n", d.Init, valuesOf(d, d.Init, 0))
	for i, e := range path {
		fmt.Fprintf(sb, "- after step %d: %s (%s)\n", i+1, e.Dst, valuesOf(d, e.Dst, i+1))
	}
	sb.WriteString("\n")
	return nil
}

// edgeCells are the transition, guard and post cells of e.
func edgeCells(e core.Edge) (string, error) {
	guard, err := cell(e.Guard)
	if err != nil {
		return "", err
	}
	post, err := cell(e.Post)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("`%s --> %s`%s | %s | %s", e.Src, e.Dst, line(e.Line), guard, post), nil
}

// writeObligation states, for one prefix path, that the values it reaches
// are unstable or enable the next event.
func writeObligation(sb *strings.Builder, d *core.Diagram, p PrefixPath) error {
	n := len(p.Path)
	if len(p.Taus) > 0 || len(p.Next) > 0 {
		sb.WriteString("| edge out of the reached state | transition | guard | post |\n")
		sb.WriteString("|-------------------------------|------------|-------|------|\n")
		for _, group := range []struct {
			kind  string
			edges []core.Edge
		}{{"tau", p.Taus}, {"next", p.Next}} {
			for _, e := range group.edges {
				row, err := edgeCells(e)
				if err != nil {
					return err
				}
				fmt.Fprintf(sb, "| %s | %s |\n", group.kind, row)
			}
		}
		sb.WriteString("\n")
	}

	r := run{}
	var conclusion []logic.Formula
	for _, e := range append(append([]core.Edge{}, p.Taus...), p.Next...) {
		enabled, err := obligation.Enabled(*d, e)
		if err != nil {
			return err
		}
		before, err := r.before(d, e, n)
		if err != nil {
			return err
		}
		conclusion = append(conclusion, logic.Rename(enabled, before))
	}
	along, err := r.pathClauses(d, p.Path)
	if err != nil {
		return err
	}
	ob, err := r.every(logic.Simplify(logic.Implies(along, logic.Or(conclusion...))))
	if err != nil {
		return err
	}
	fmt.Fprintf(sb, "Obligation: %s\n\n", notation.Spell(ob))
	return nil
}

// notation spells the formulas of a report, a space after each quantifier.
var notation = func() logic.Notation {
	n, err := logic.NewNotation(logic.Connectives{
		And: " ∧ ", Or: " ∨ ", Not: "¬", Implies: " → ",
		Exists: "∃ ", Forall: "∀ ", True: "true", False: "false",
	})
	if err != nil {
		panic(fmt.Sprintf("tracechk: the notation of a report is ill made: %v", err))
	}
	return n
}()

// run holds the type of each value along a run a report names: those
// obligation.After and obligation.Received name, which every variable of an
// obligation is.
type run map[logic.Var]string

// at is the value of the state variable v of d after step i.
func (r run) at(d *core.Diagram, v string, i int) (logic.Var, error) {
	t, err := d.VarType(v)
	if err != nil {
		return "", err
	}
	return r.add(obligation.After(v, i, t)), nil
}

// received is the value the j-th field of e receives at step i.
func (r run) received(d *core.Diagram, e core.Event, j, i int) (logic.Var, error) {
	t, err := d.FieldType(e, j)
	if err != nil {
		return "", err
	}
	return r.add(obligation.Received(e.Fields[j].Name, i, t)), nil
}

func (r run) add(v obligation.Value) logic.Var {
	r[v.Var] = v.Type
	return v.Var
}

// some binds the free variables of f existentially, and every universally,
// each of its type.
func (r run) some(f logic.Formula) (logic.Formula, error) {
	return r.bind(f, logic.ExistsTyped)
}

func (r run) every(f logic.Formula) (logic.Formula, error) {
	return r.bind(f, logic.ForallTyped)
}

func (r run) bind(f logic.Formula, q func([]logic.Var, []string, logic.Formula) logic.Formula) (logic.Formula, error) {
	vars := logic.FreeVars(f)
	if len(vars) == 0 {
		return f, nil
	}
	types := make([]string, len(vars))
	for i, v := range vars {
		typ, ok := r[v]
		if !ok {
			return nil, fmt.Errorf("tracechk: %s is no value of the run", v)
		}
		types[i] = typ
	}
	return q(vars, types, f), nil
}

// before maps the variables of the source of e to their values after step n,
// those e is taken from.
func (r run) before(d *core.Diagram, e core.Edge, n int) (map[logic.Var]logic.Var, error) {
	m := map[logic.Var]logic.Var{}
	for _, v := range d.Vars[e.Src] {
		x, err := r.at(d, v, n)
		if err != nil {
			return nil, err
		}
		m[logic.Var(v)] = x
	}
	return m, nil
}

// startClause is the post of the start edge over the values after step 0.
func (r run) startClause(d *core.Diagram) (logic.Formula, error) {
	f, err := obligation.Formula(d.InitPost)
	if err != nil {
		return nil, fmt.Errorf("the start post: %w", err)
	}
	m := map[logic.Var]logic.Var{}
	for _, v := range d.Vars[d.Init] {
		x, err := r.at(d, v, 0)
		if err != nil {
			return nil, fmt.Errorf("the start post: %w", err)
		}
		m[obligation.Var(clause.Free{Name: v, Primed: true})] = x
	}
	return logic.Rename(f, m), nil
}

// pathClauses is the conjunction of the clauses along a path, from the
// start post: the guard and post of step i read the values after step i-1,
// the fields it receives and the values after it.
func (r run) pathClauses(d *core.Diagram, path Path) (logic.Formula, error) {
	start, err := r.startClause(d)
	if err != nil {
		return nil, err
	}
	conjuncts := []logic.Formula{start}
	for i, e := range path {
		step, err := obligation.Step(*d, e)
		if err != nil {
			return nil, err
		}
		m, err := r.before(d, e, i)
		if err != nil {
			return nil, err
		}
		for _, v := range d.Vars[e.Dst] {
			x, err := r.at(d, v, i+1)
			if err != nil {
				return nil, err
			}
			m[obligation.Var(clause.Free{Name: v, Primed: true})] = x
		}
		for j, f := range e.Event.Fields {
			if f.Send {
				continue
			}
			x, err := r.received(d, e.Event, j, i+1)
			if err != nil {
				return nil, err
			}
			m[logic.Var(f.Name)] = x
		}
		conjuncts = append(conjuncts, logic.Rename(step, m))
	}
	return logic.Simplify(logic.And(conjuncts...)), nil
}

func prefixText(events []string) string {
	if len(events) == 0 {
		return "the empty prefix"
	}
	return joinEvents(events)
}

func stableText(refusing []string) string {
	if len(refusing) == 1 {
		return fmt.Sprintf("%s is stable", refusing[0])
	}
	return fmt.Sprintf("%s are stable", joinStates(refusing))
}

// cell renders a clause in a Markdown table cell, where a bar would end it.
func cell(f clause.And) (string, error) {
	if len(f.Conjuncts) == 0 {
		return "true", nil
	}
	s, err := clause.Print(f)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(s, "|", "\\|"), nil
}

// valuesOf names the values of the variables of the state s after step i.
func valuesOf(d *core.Diagram, s string, i int) string {
	vars := d.Vars[s]
	if len(vars) == 0 {
		return "no variables"
	}
	names := make([]string, len(vars))
	for j, v := range vars {
		names[j] = string(obligation.AfterVar(v, i))
	}
	return strings.Join(names, ", ")
}

func joinEvents(events []string) string {
	if len(events) == 0 {
		return "(none)"
	}
	ss := make([]string, len(events))
	for i, e := range events {
		ss[i] = "`" + e + "`"
	}
	return strings.Join(ss, ", ")
}

func joinStates(states []string) string {
	return strings.Join(states, ", ")
}

// line is where an edge was read, " (L<n>)", or nothing for one no reader
// read.
func line(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf(" (L%d)", n)
}
