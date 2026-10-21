"""A diagram whose clauses are written in a formal language is walked
through a glue: the checker asks it what a clause reads and gives a value
to, and what it evaluates to."""
import io
from contextlib import redirect_stderr
from dataclasses import replace
import json
import os
import shlex
import sys
import unittest

from csdfbmc.environment import NONE
from csdfbmc.tests import fixture

from csdfbmc.cone import Cone
from csdfbmc.explore import Explorer
from csdfbmc.model import Clause, Diagram
from csdfbmc.meaning import GlueMeaning, standard
from csdfbmc.protocol import Client, EvaluatorError, checked, start_module
from csdfbmc.checks import Checks

COUNTER = {
    "states": {
        "a": {"name": "counting", "vars": [{"name": "n"}], "line": 2},
        "b": {"name": "stopped", "vars": [], "line": 4},
    },
    "start_edge": {"dst": "a", "post": "[{'n': 2}]", "line": 5},
    "edges": [
        {"src": "a", "dst": "a", "event": "dec", "channel": "dec", "fields": [], "guard": "n > 0", "post": "[{'n': n - 1}]", "line": 6},
        {"src": "a", "dst": "b", "event": "stop", "channel": "stop", "fields": [], "guard": "n == 0", "post": "", "line": 7},
    ],
    "end_edge": None,
    "assumptions": [],
}

NO_CHECKS = Checks(types={})


def python_glue() -> Client:
    return start_module("csdfbmc.python_glue", [])


def glue(client):
    return standard(GlueMeaning(checked(client)))


def counter(**edits) -> Diagram:
    edges = [dict(e, **edits.get(e["event"], {})) for e in COUNTER["edges"]]
    return fixture.diagram(dict(COUNTER, edges=edges))


class LabelsTest(unittest.TestCase):
    def test_the_clauses_are_evaluated_through_the_glue(self):
        result = Explorer(counter(), NO_CHECKS, meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE).run()
        self.assertEqual({("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())}, set(result.states))
        self.assertEqual([("b", (), ())], result.deadlocks)

    def test_the_tool_word_opaque_follows_the_opaque_rule_without_asking_the_glue(self):
        counting = Counting(python_glue())
        result = Explorer(counter(dec={"guard": "@opaque"}), NO_CHECKS, meaning=glue(counting), cone=Cone.keep_all(), environment=NONE, opaque="true", max_states=10).run()
        self.assertEqual(["@opaque"], result.opaque_clauses)
        self.assertNotIn("@opaque", [r["clause"] for r in counting.requests])

    def test_an_opaque_clause_is_refused_by_default(self):
        with self.assertRaises(KeyError) as cm:
            Explorer(counter(dec={"guard": "@opaque"}), NO_CHECKS, meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE).run()
        self.assertIn("@opaque", str(cm.exception))

    def test_an_opaque_clause_may_be_taken_as_true(self):
        result = Explorer(counter(dec={"guard": "@opaque"}), NO_CHECKS, meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE, opaque="true", max_states=10).run()
        self.assertIn(("a", (-1,), ()), set(result.states))

    def test_a_clause_is_evaluated_once_per_value_of_what_it_reads(self):
        counting = Counting(python_glue())
        Explorer(counter(), NO_CHECKS, meaning=glue(counting), cone=Cone.keep_all(), environment=NONE).run()
        guards = [r for r in counting.requests if r["op"] == "guard" and r["clause"] == "n > 0"]
        self.assertEqual(3, sum(len(r["bindings"]) for r in guards))

    def test_a_guard_is_asked_once_per_state_for_every_parameter_value(self):
        diagram = fixture.diagram(dict(COUNTER, edges=[
            {"src": "a", "dst": "a", "event": "add?k", "channel": "add", "fields": [{"dir": "?", "name": "k"}], "guard": "n + k <= 3", "post": "[{'n': n + k}]", "line": 6}]))
        counting = Counting(python_glue())
        result = Explorer(diagram, Checks(types={"k": [1, 2, 3]}), meaning=glue(counting), cone=Cone.keep_all(), environment=NONE).run()
        guards = [r for r in counting.requests if r["op"] == "guard"]
        self.assertEqual(len(result.states), len(guards))
        self.assertEqual([3, 3], [len(r["bindings"]) for r in guards[:2]])

    def test_an_assignment_must_give_a_value_to_exactly_the_writes(self):
        lying = Lying(python_glue())
        with self.assertRaises(EvaluatorError) as cm:
            Explorer(counter(), NO_CHECKS, meaning=glue(lying), cone=Cone.keep_all(), environment=NONE).run()
        self.assertIn("[{'n': n - 1}]", str(cm.exception))


from csdfbmc.tests.test_promoted import relabel  # noqa: E402


class SymmetryThroughAGlueTest(unittest.TestCase):
    """Symmetry asks the glue of each clause whether it tells the IDs apart."""

    def setUp(self):
        from csdfbmc.tests.test_promoted import GLOBAL_CHECKS
        self.checks = Checks(types=GLOBAL_CHECKS.types, symmetric=("口座ID",), sources=[("checks.py", "TYPES = {}")])

    def test_symmetry_is_refused_with_a_glue_that_does_not_answer_it(self):
        # Setup
        from csdfbmc.tests.test_promoted import GLUE_EXPANSION
        silent = NotAnswering(python_glue())

        # Execute, Assert
        with self.assertRaisesRegex(ValueError, "does not answer"):
            Explorer(fixture.diagram(GLUE_EXPANSION), self.checks, meaning=glue(silent), cone=Cone.keep_all(), environment=NONE, symmetry=True)

    def test_symmetry_refuses_a_clause_the_glue_says_tells_the_ids_apart(self):
        # Setup
        from csdfbmc.tests.test_reduce import with_constrain
        d = fixture.diagram(with_constrain("口座ID != 'a1'"))

        # Execute, Assert
        with self.assertRaisesRegex(ValueError, "tells the IDs apart"):
            Explorer(d, self.checks, meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE, symmetry=True)


class Counting:
    """A Client that records what it is asked."""

    def __init__(self, inner: Client):
        self.inner, self.language, self.supports, self.requests = inner, inner.language, inner.supports, []

    def analyse(self, clause, role):
        return self.inner.analyse(clause, role)

    def guard(self, clause, bindings):
        self.requests.append({"op": "guard", "clause": clause, "bindings": bindings})
        return self.inner.guard(clause, bindings)

    def post(self, clause, bindings):
        self.requests.append({"op": "post", "clause": clause, "bindings": bindings})
        return self.inner.post(clause, bindings)

    def close(self):
        self.inner.close()


class NotAnswering(Counting):
    """A Client of a glue that answers none of the optional requests."""

    def __init__(self, inner: Client):
        super().__init__(inner)
        self.supports = frozenset()


class Lying(Counting):
    """Says a post writes n but answers an assignment to m as well."""

    def post(self, clause, bindings):
        results = super().post(clause, bindings)
        if clause == "[{'n': n - 1}]":
            return [[dict(a, m=0) for a in r] for r in results]
        return results


class LabelsCommandTest(unittest.TestCase):
    """csdfbmc and csdfbmc next take the glue as a command or as a module."""

    def run_main(self, main, *args):
        return fixture.run_main(self, main, COUNTER, *args)

    def test_the_checker_reads_the_clauses_through_either_form_of_the_glue(self):
        from csdfbmc.__main__ import main
        # The command finds the package through PYTHONPATH (so no -E), wherever the test runs.
        command = shlex.join(["env", f"PYTHONPATH={ROOT}", sys.executable, "-s", "-B", "-m", "csdfbmc.python_glue"])
        for flag, value in (("--evaluator", command), ("--evaluator-module", "csdfbmc.python_glue")):
            with self.subTest(flag=flag):
                code, out, err = self.run_main(main, flag, value)
                self.assertEqual(1, code, err)
                self.assertIn("deadlock at b [] after ['dec', 'dec', 'stop']", out)

    def test_next_reads_the_clauses_through_the_glue(self):
        from csdfbmc.next import main
        code, out, err = self.run_main(main, "--evaluator-module", "csdfbmc.python_glue", "--initial")
        self.assertEqual(0, code, err)
        self.assertEqual({"candidates": [{"params": {}, "values": [2]}]}, json.loads(out))

    def test_a_walk_without_a_glue_is_refused(self):
        from csdfbmc.__main__ import main

        # Execute
        code, _, err = self.run_main(main)

        # Assert
        self.assertEqual(2, code)
        self.assertIn("csdfbmc reads every clause through one", err)

    def test_the_command_line_refuses_two_glues_and_local(self):
        from csdfbmc.__main__ import main
        for name, args in [
            ("both forms", ("--evaluator", "glue", "--evaluator-module", "csdfbmc.python_glue")),
            ("--local", ("--evaluator-module", "csdfbmc.python_glue", "--local", "local/A.puml=a.py")),
        ]:
            with self.subTest(name):
                # Execute, Assert
                with redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as raised:
                    self.run_main(main, *args)
                self.assertEqual(2, raised.exception.code)

    def test_a_glue_that_cannot_start_is_a_loading_error(self):
        from csdfbmc.__main__ import main
        code, _, err = self.run_main(main, "--evaluator-module", "no.such.glue")
        self.assertEqual(2, code)
        self.assertIn("no.such.glue", err)

    def test_an_empty_glue_module_is_a_loading_error(self):
        from csdfbmc.__main__ import main
        code, _, err = self.run_main(main, "--evaluator-module", "")
        self.assertEqual(2, code)
        self.assertIn("MODULE", err)

    def test_the_two_forms_of_the_glue_exclude_each_other(self):
        from csdfbmc.__main__ import main
        with self.assertRaises(SystemExit):
            self.run_main(main, "--evaluator", "x", "--evaluator-module", "y")


ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


# What hiding put?x leaves: a tau whose clauses still read x.
HIDDEN = {
    "states": {"a": {"name": "a", "vars": [{"name": "n"}], "line": 2}},
    "start_edge": {"dst": "a", "post": "[{'n': 0}]", "line": 3},
    "edges": [{"src": "a", "dst": "a", "event": "tau", "guard": "x > 0 and n + x <= 3", "post": "[{'n': n + x}]", "line": 4}],
    "end_edge": None, "assumptions": [],
}


class HiddenFieldTest(unittest.TestCase):
    """A name a tau reads that is neither a state variable nor a field is a
    value the tau chooses: hiding c?x leaves ⊓ x. τ, over the domain of x."""

    def test_a_tau_chooses_a_value_it_reads_from_its_domain(self):
        # Setup
        explorer = Explorer(fixture.diagram(HIDDEN), Checks(types={"x": [0, 1, 2]}), meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE)

        # Execute
        result = explorer.run()

        # Assert
        self.assertEqual({("a", (n,), ()) for n in range(4)}, set(result.states))
        self.assertIn("tau(x=1)", [label for _, label, _, _ in explorer.successors(("a", (0,), ()))])

    def test_a_name_with_no_domain_is_not_chosen_but_refused(self):
        # Setup
        explorer = Explorer(fixture.diagram(HIDDEN), NO_CHECKS, meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE)

        # Execute, Assert
        with self.assertRaisesRegex(KeyError, "'x'"):
            explorer.run()



# What composing put?x with put?y leaves: the right clause reads y for x.
SYNCED = {
    "states": {"a": {"name": "a", "vars": [{"name": "n"}], "line": 2}},
    "start_edge": {"dst": "a", "post": "[{'n': 0}]", "line": 3},
    "edges": [{"src": "a", "dst": "a", "event": "put?x", "guard": "", "post": "[{'n': x}]", "line": 4}],
    "end_edge": None, "assumptions": [],
}


class SynchronisedDomainTest(unittest.TestCase):
    """Two sides synchronise only on a value both can take: a field renamed
    onto another ranges over the values both their domains hold."""

    def test_a_field_ranges_over_the_domains_of_the_names_renamed_onto_it(self):
        # Setup
        d = fixture.diagram(SYNCED)
        d.edges[0] = replace(d.edges[0], guard=(Clause("local", "y >= 0", rename=(("y", "x"),)),))
        explorer = Explorer(d, Checks(types={"x": [0, 1, 2], "y": [0, 1]}), meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE)

        # Execute
        result = explorer.run()

        # Assert
        self.assertEqual({("a", (0,), ()), ("a", (1,), ())}, set(result.states))



# A counter that reports its value: report!n sends n, which the state holds.
REPORTING = {
    "states": {"a": {"name": "a", "vars": [{"name": "n"}], "line": 2}},
    "start_edge": {"dst": "a", "post": "[{'n': 0}]", "line": 3},
    "edges": [
        {"src": "a", "dst": "a", "event": "inc", "guard": "n < 2", "post": "[{'n': n + 1}]", "line": 4},
        {"src": "a", "dst": "a", "event": "report!n", "guard": "", "post": "[{'n': n}]", "line": 5},
    ],
    "end_edge": None, "assumptions": [],
}


class SentFieldTest(unittest.TestCase):
    """A sent field is the value its name has at the step, not a value
    chosen from a domain: c!y sends y."""

    def test_a_sent_field_sends_the_value_the_step_binds_without_a_domain(self):
        # Setup
        explorer = Explorer(fixture.diagram(REPORTING), NO_CHECKS, meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE)

        # Execute
        labels = [label for _, label, _, _ in explorer.successors(("a", (1,), ()))]

        # Assert
        self.assertEqual(["inc", "report(n=1)"], sorted(labels))

    def test_the_label_follows_the_order_of_the_fields(self):
        # Setup
        diagram = dict(REPORTING, edges=[dict(REPORTING["edges"][1], event="report!n?k")])
        explorer = Explorer(fixture.diagram(diagram), Checks(types={"k": [7]}), meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE)

        # Execute
        labels = [label for _, label, _, _ in explorer.successors(("a", (1,), ()))]

        # Assert
        self.assertEqual(["report(n=1, k=7)"], labels)

    def test_a_ghost_is_not_a_name_an_event_sends(self):
        # Setup
        diagram = dict(REPORTING, edges=[dict(REPORTING["edges"][1], event="report!g")])
        checks = Checks(types={}, ghosts={"g": (0, {})})

        # Execute, Assert
        with self.assertRaisesRegex(KeyError, "sends 'g'"):
            Explorer(fixture.diagram(diagram), checks, meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE)

    def test_a_sent_field_whose_name_the_step_does_not_bind_is_refused(self):
        # Setup
        diagram = dict(REPORTING, edges=[dict(REPORTING["edges"][1], event="report!m")])
        # Execute, Assert: refused before a walk, whether or not it is enabled.
        with self.assertRaisesRegex(KeyError, "sends 'm'"):
            Explorer(fixture.diagram(diagram), NO_CHECKS, meaning=glue(python_glue()), cone=Cone.keep_all(), environment=NONE)


if __name__ == "__main__":
    unittest.main()
