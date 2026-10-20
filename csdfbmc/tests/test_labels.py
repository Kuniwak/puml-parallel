"""A diagram whose clauses are written in a formal language is walked
through a glue: the checker asks it what a clause reads and gives a value
to, and what it evaluates to, and needs no twin keyed by prose."""
import io
import json
import os
import shlex
import sys
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from csdfbmc.tests import fixture

from csdfbmc.cone import Cone
from csdfbmc.explore import Explorer
from csdfbmc.model import Diagram
from csdfbmc.meaning import GlueMeaning, standard
from csdfbmc.protocol import Client, EvaluatorError, checked, start_module
from csdfbmc.twins import Twins

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

NO_TWINS = Twins(domains={}, clauses={})


def python_glue() -> Client:
    return start_module("csdfbmc.python_glue", [])


def glue(client):
    return standard(GlueMeaning(checked(client)))


def counter(**edits) -> Diagram:
    edges = [dict(e, **edits.get(e["event"], {})) for e in COUNTER["edges"]]
    return fixture.diagram(dict(COUNTER, edges=edges))


class LabelsTest(unittest.TestCase):
    def test_the_clauses_are_evaluated_through_the_glue(self):
        result = Explorer(counter(), NO_TWINS, meaning=glue(python_glue()), cone=Cone.keep_all()).run()
        self.assertEqual({("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())}, set(result.states))
        self.assertEqual([("b", (), ())], result.deadlocks)

    def test_the_tool_word_opaque_follows_the_opaque_rule_without_asking_the_glue(self):
        counting = Counting(python_glue())
        result = Explorer(counter(dec={"guard": "@opaque"}), NO_TWINS, meaning=glue(counting), cone=Cone.keep_all(), opaque="true", max_states=10).run()
        self.assertEqual(["@opaque"], result.opaque_clauses)
        self.assertNotIn("@opaque", [r["clause"] for r in counting.requests])

    def test_an_opaque_clause_is_refused_by_default(self):
        with self.assertRaises(KeyError) as cm:
            Explorer(counter(dec={"guard": "@opaque"}), NO_TWINS, meaning=glue(python_glue()), cone=Cone.keep_all()).run()
        self.assertIn("@opaque", str(cm.exception))

    def test_an_opaque_clause_may_be_taken_as_true(self):
        result = Explorer(counter(dec={"guard": "@opaque"}), NO_TWINS, meaning=glue(python_glue()), cone=Cone.keep_all(), opaque="true", max_states=10).run()
        self.assertIn(("a", (-1,), ()), set(result.states))

    def test_a_clause_is_evaluated_once_per_value_of_what_it_reads(self):
        counting = Counting(python_glue())
        Explorer(counter(), NO_TWINS, meaning=glue(counting), cone=Cone.keep_all()).run()
        guards = [r for r in counting.requests if r["op"] == "guard" and r["clause"] == "n > 0"]
        self.assertEqual(3, sum(len(r["bindings"]) for r in guards))

    def test_a_guard_is_asked_once_per_state_for_every_parameter_value(self):
        diagram = fixture.diagram(dict(COUNTER, edges=[
            {"src": "a", "dst": "a", "event": "add?k", "channel": "add", "fields": [{"dir": "?", "name": "k"}], "guard": "n + k <= 3", "post": "[{'n': n + k}]", "line": 6}]))
        counting = Counting(python_glue())
        result = Explorer(diagram, Twins(domains={"k": [1, 2, 3]}, clauses={}), meaning=glue(counting), cone=Cone.keep_all()).run()
        guards = [r for r in counting.requests if r["op"] == "guard"]
        self.assertEqual(len(result.states), len(guards))
        self.assertEqual([3, 3], [len(r["bindings"]) for r in guards[:2]])

    def test_twins_keyed_by_prose_are_refused_with_a_glue(self):
        twins = Twins(domains={}, clauses={"n > 0": lambda n: n > 0})
        with self.assertRaises(ValueError) as cm:
            Explorer(counter(), twins, meaning=glue(python_glue()), cone=Cone.keep_all())
        self.assertIn("CLAUSES", str(cm.exception))

    def test_an_assignment_must_give_a_value_to_exactly_the_writes(self):
        lying = Lying(python_glue())
        with self.assertRaises(EvaluatorError) as cm:
            Explorer(counter(), NO_TWINS, meaning=glue(lying), cone=Cone.keep_all()).run()
        self.assertIn("[{'n': n - 1}]", str(cm.exception))


# The expansion of test_promoted with every clause written in Python, as a
# front end renders the formal labels in place of the prose.
FORMAL = {
    "accounts は空": "[{'accounts': FrozenMap()}]",
    "残高' = 額": "[{'残高': 額}]",
    "0 < 額": "0 < 額",
    "残高' = 残高 + 額": "[{'残高': 残高 + 額}]",
    "残高 = 0": "残高 == 0",
    "残高 + 額 ≤ 上限": "accounts[口座ID].残高 + 額 <= 3",
}


def relabel(value):
    if isinstance(value, dict):
        return {k: (FORMAL.get(v, v) if k in ("text", "post", "guard") and isinstance(v, str) else relabel(v)) for k, v in value.items()}
    if isinstance(value, list):
        return [relabel(v) for v in value]
    return value


class FormalExpansionTest(unittest.TestCase):
    def setUp(self):
        from csdfbmc.tests.test_promoted import EXPANSION, GLOBAL_TWINS
        self.prose = Explorer.of_twins(fixture.diagram(EXPANSION), GLOBAL_TWINS).run()
        self.diagram = fixture.diagram(relabel(EXPANSION))
        self.twins = Twins(domains=GLOBAL_TWINS.domains, clauses={})

    def test_a_formal_expansion_needs_no_twins_and_finds_the_same(self):
        result = Explorer(self.diagram, self.twins, meaning=glue(python_glue()), cone=Cone.keep_all()).run()
        self.assertEqual(set(self.prose.states), set(result.states))
        self.assertEqual(self.prose.deadlocks, result.deadlocks)

    def test_partial_order_reduction_reads_what_the_glue_says_a_clause_reads(self):
        from csdfbmc.tests.test_promoted import EXPANSION, GLOBAL_TWINS
        prose = Explorer.of_twins(fixture.diagram(EXPANSION), GLOBAL_TWINS, por=True).run()
        result = Explorer(self.diagram, self.twins, meaning=glue(python_glue()), cone=Cone.keep_all(), por=True).run()
        self.assertEqual(set(prose.states), set(result.states))
        self.assertEqual(prose.deadlocks, result.deadlocks)

    def test_symmetry_is_refused_since_the_checker_cannot_read_the_clauses(self):
        twins = Twins(domains=self.twins.domains, clauses={}, symmetric=("口座ID",), sources=[("twins.py", "DOMAINS = {}")])
        with self.assertRaises(ValueError) as cm:
            Explorer(self.diagram, twins, meaning=glue(python_glue()), cone=Cone.keep_all(), symmetry=True)
        self.assertIn("through a glue", str(cm.exception))


class Counting:
    """A Client that records what it is asked."""

    def __init__(self, inner: Client):
        self.inner, self.language, self.requests = inner, inner.language, []

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


class Lying(Counting):
    """Says a post writes n but answers an assignment to m as well."""

    def post(self, clause, bindings):
        results = super().post(clause, bindings)
        if clause == "[{'n': n - 1}]":
            return [[dict(a, m=0) for a in r] for r in results]
        return results


class LabelsCommandTest(unittest.TestCase):
    """csdfbmc and csdfbmc next take the glue as a command or as a module."""

    def setUp(self):
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        self.diagram, self.twins = os.path.join(d.name, "d.json"), os.path.join(d.name, "t.py")
        with open(self.diagram, "w", encoding="utf-8") as f:
            json.dump(fixture.parsed(COUNTER), f)
        with open(self.twins, "w", encoding="utf-8") as f:
            f.write("DOMAINS = {}\n")

    def run_main(self, main, *args):
        out, err = io.StringIO(), io.StringIO()
        with redirect_stdout(out), redirect_stderr(err):
            code = main([self.diagram, self.twins, *args])
        return code, out.getvalue(), err.getvalue()

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


if __name__ == "__main__":
    unittest.main()
