package tracechk

import (
	"fmt"
	"io"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// WriteMarkdown writes the result as a Markdown section. A rejection says
// where the trace stopped being one and along which paths, and what would have
// to fail for the refusal not to be real. An acceptance lists, for every event
// of the trace and every path performing the prefix before it, the predicates
// along the path and the obligation that the state reached is unstable or can
// perform the event; it ends with a prompt asking a reader to decide them.
func WriteMarkdown(w io.Writer, d *csdf.Diagram, r Result) error {
	var sb strings.Builder
	if r.Rejection != nil {
		writeRejection(&sb, d, r)
	} else {
		writeAcceptance(&sb, d, r)
	}
	_, err := io.WriteString(w, sb.String())
	if err != nil {
		return fmt.Errorf("tracechk.WriteMarkdown: %w", err)
	}
	return nil
}

func writeRejection(sb *strings.Builder, d *csdf.Diagram, r Result) {
	rej := r.Rejection
	fmt.Fprintf(sb, "# %s: REJECTED\n\n", r.Trace.Name)
	fmt.Fprintf(sb, "After %s (%d of %d events) the diagram may be in %s.\n",
		prefixText(r.Trace.Events[:rej.Index]), rej.Index, len(r.Trace.Events), joinStates(rej.States))
	if len(rej.Refusing) > 0 {
		fmt.Fprintf(sb, "%s and has no edge for `%s`, so the diagram may refuse `%s` there even\nwhen every guard is true: this is not a trace of the diagram.\n\n",
			stableText(rej.Refusing), rej.Event, rej.Event)
		fmt.Fprintf(sb, "- visible events enabled at the refusing states: %s\n\n", joinEvents(rej.Enabled))
		sb.WriteString("The refusal is real unless every path below is infeasible, that is, unless\nthe predicates along it cannot all hold.\n\n")
	} else {
		fmt.Fprintf(sb, "None of them is stable and none can perform `%s`, so the diagram diverges\ninstead of performing it even when every guard is true: this is not a trace\nof the diagram.\n\n", rej.Event)
		sb.WriteString("The divergence is real unless every path below is infeasible, that is, unless\nthe predicates along it cannot all hold.\n\n")
	}
	for i, p := range rej.Paths {
		fmt.Fprintf(sb, "## Path %d: reaches %s\n\n", i+1, p.Dst)
		writePathTable(sb, d, p.Path)
		fmt.Fprintf(sb, "Infeasible iff: ¬∃%s. %s\n\n", valuations(len(p.Path)), pathPredicates(d, p.Path))
	}
}

func writeAcceptance(sb *strings.Builder, d *csdf.Diagram, r Result) {
	fmt.Fprintf(sb, "# %s: ACCEPTED when every guard is true\n\n", r.Trace.Name)
	sb.WriteString(`After every prefix of the trace, every stable state the diagram may be in can
perform the next event when every guard is taken as true. Whether it can in
fact depends on the natural-language predicates below, which this tool does not
evaluate. A path never revisits a state within one run of ` + "`tau`" + ` edges, so
paths going round a ` + "`tau`" + ` cycle, and their obligations, are not listed.

`)
	fmt.Fprintf(sb, "- trace: %s\n", joinEvents(r.Trace.Events))
	fmt.Fprintf(sb, "- the start edge `[*] --> %s` (L%d) must admit a valuation: ∃ x0. post_0(x0), where post_0 is %s\n\n",
		d.StartEdge.Dst, d.StartEdge.Line, predicateText(d.StartEdge.Post))

	for _, step := range r.Steps {
		fmt.Fprintf(sb, "## Event %d: `%s` after %s\n\n", step.Index+1, step.Event, prefixText(r.Trace.Events[:step.Index]))
		for j, p := range step.Paths {
			fmt.Fprintf(sb, "### Path %d.%d: reaches %s\n\n", step.Index+1, j+1, p.Dst)
			writePathTable(sb, d, p.Path)
			writeObligation(sb, d, p)
		}
	}

	sb.WriteString(`## Prompt

Decide whether every obligation above holds, reading each predicate as the
natural-language statement in its table. Answer HOLDS, or FAILS naming the
first obligation that does not hold and valuations that satisfy its premise
but not its conclusion. A guard is not true because it is written down: the
trace is a trace of the diagram only if the predicates make every obligation
hold. FAILS settles that it is not; HOLDS settles it only for the listed
paths, since paths going round a ` + "`tau`" + ` cycle are not listed.

`)
}

// writePathTable tabulates the start edge as step 0 and each edge of the path
// as the following steps, then names the valuation each step reaches.
func writePathTable(sb *strings.Builder, d *csdf.Diagram, path Path) {
	sb.WriteString("| step | event | transition | guard | post |\n")
	sb.WriteString("|-----:|-------|------------|-------|------|\n")
	fmt.Fprintf(sb, "| 0 | (start) | `[*] --> %s` (L%d) |  | %s |\n", d.StartEdge.Dst, d.StartEdge.Line, cell(d.StartEdge.Post))
	for i, e := range path {
		fmt.Fprintf(sb, "| %d | `%s` | `%s --> %s` (L%d) | %s | %s |\n", i+1, e.Event, e.Src, e.Dst, e.Line, cell(e.Guard), cell(e.Post))
	}
	sb.WriteString("\n")

	fmt.Fprintf(sb, "- x0: %s (%s)\n", d.StartEdge.Dst, varsOf(d, d.StartEdge.Dst))
	for i, e := range path {
		fmt.Fprintf(sb, "- x%d: %s (%s)\n", i+1, e.Dst, varsOf(d, e.Dst))
	}
	sb.WriteString("\n")
}

// writeObligation states, for one prefix path, that the valuation it reaches
// either is unstable or enables the next event. Edges out of the reached state
// are named by source line, since the path table has no row for them.
func writeObligation(sb *strings.Builder, d *csdf.Diagram, p PrefixPath) {
	n := len(p.Path)
	if len(p.Taus) > 0 || len(p.Next) > 0 {
		sb.WriteString("| edge out of the reached state | transition | guard | post |\n")
		sb.WriteString("|-------------------------------|------------|-------|------|\n")
		for _, e := range p.Taus {
			fmt.Fprintf(sb, "| tau | `%s --> %s` (L%d) | %s | %s |\n", e.Src, e.Dst, e.Line, cell(e.Guard), cell(e.Post))
		}
		for _, e := range p.Next {
			fmt.Fprintf(sb, "| next | `%s --> %s` (L%d) | %s | %s |\n", e.Src, e.Dst, e.Line, cell(e.Guard), cell(e.Post))
		}
		sb.WriteString("\n")
	}

	var conclusion []string
	for _, e := range p.Taus {
		conclusion = append(conclusion, enabled(e, n))
	}
	for _, e := range p.Next {
		conclusion = append(conclusion, enabled(e, n))
	}
	if len(conclusion) == 0 {
		conclusion = append(conclusion, "false")
	}
	fmt.Fprintf(sb, "Obligation: ∀%s. %s → %s\n\n", valuations(n), pathPredicates(d, p.Path), strings.Join(conclusion, " ∨ "))
}

// enabled is the enabledness of an edge at valuation xn: its guard holds and
// its post admits some successor. A predicate that is exactly true is left out.
func enabled(e csdf.Edge, n int) string {
	var parts []string
	if !csdf.IsTrue(e.Guard) {
		parts = append(parts, fmt.Sprintf("guard_L%d(x%d)", e.Line, n))
	}
	if !csdf.IsTrue(e.Post) {
		parts = append(parts, fmt.Sprintf("∃ x'. post_L%d(x%d, x')", e.Line, n))
	}
	if len(parts) == 0 {
		return "true"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, " ∧ ") + ")"
}

// pathPredicates is the conjunction of the predicates along a path, over the
// valuations x0 (admitted by the start edge) to xn. Predicates that are exactly
// true are left out.
func pathPredicates(d *csdf.Diagram, path Path) string {
	var conjuncts []string
	if !csdf.IsTrue(d.StartEdge.Post) {
		conjuncts = append(conjuncts, "post_0(x0)")
	}
	for i, e := range path {
		if !csdf.IsTrue(e.Guard) {
			conjuncts = append(conjuncts, fmt.Sprintf("guard_%d(x%d)", i+1, i))
		}
		if !csdf.IsTrue(e.Post) {
			conjuncts = append(conjuncts, fmt.Sprintf("post_%d(x%d, x%d)", i+1, i, i+1))
		}
	}
	if len(conjuncts) == 0 {
		return "true"
	}
	return strings.Join(conjuncts, " ∧ ")
}

func valuations(n int) string {
	var sb strings.Builder
	for i := 0; i <= n; i++ {
		fmt.Fprintf(&sb, " x%d", i)
	}
	return sb.String()
}

func prefixText(events []csdf.Event) string {
	if len(events) == 0 {
		return "the empty prefix"
	}
	return joinEvents(events)
}

func stableText(refusing []csdf.StateID) string {
	if len(refusing) == 1 {
		return fmt.Sprintf("%s is stable", refusing[0])
	}
	return fmt.Sprintf("%s are stable", joinStates(refusing))
}

func predicateText(p csdf.Predicate) string {
	if csdf.IsTrue(p) {
		return "`true`"
	}
	return "`" + string(p) + "`"
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
