"""A diagram whose clauses are written in a formal language is walked
through a glue: the checker asks it what a clause reads and gives a value
to, and what it evaluates to, and needs no twin keyed by prose."""
import unittest

from csdfbmc.explore import Explorer
from csdfbmc.model import Diagram
from csdfbmc.protocol import Evaluator, start
from csdfbmc.twins import Twins

COUNTER = {
    "states": {
        "a": {"name": "counting", "vars": [{"name": "n"}], "line": 2},
        "b": {"name": "stopped", "vars": [], "line": 4},
    },
    "start_edge": {"dst": "a", "post": "[{'n': 2}]", "line": 5},
    "edges": [
        {"src": "a", "dst": "a", "event": "dec", "guard": "n > 0", "post": "[{'n': n - 1}]", "line": 6},
        {"src": "a", "dst": "b", "event": "stop", "guard": "n == 0", "post": "true", "line": 7},
    ],
    "end_edge": None,
    "assumptions": [],
}

NO_TWINS = Twins(domains={}, clauses={})


def python_glue() -> Evaluator:
    return start(module=["csdfbmc.python_glue"])


def counter(**edits) -> Diagram:
    edges = [dict(e, **edits.get(e["event"], {})) for e in COUNTER["edges"]]
    return Diagram.from_json(dict(COUNTER, edges=edges))


class LabelsTest(unittest.TestCase):
    def test_the_clauses_are_evaluated_through_the_glue(self):
        result = Explorer(counter(), NO_TWINS, evaluator=python_glue()).run()
        self.assertEqual({("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())}, set(result.states))
        self.assertEqual([("b", (), ())], result.deadlocks)

    def test_an_opaque_clause_is_refused_by_default(self):
        with self.assertRaises(KeyError) as cm:
            Explorer(counter(dec={"guard": "OPAQUE"}), NO_TWINS, evaluator=python_glue()).run()
        self.assertIn("OPAQUE", str(cm.exception))

    def test_an_opaque_clause_may_be_taken_as_true(self):
        result = Explorer(counter(dec={"guard": "OPAQUE"}), NO_TWINS, evaluator=python_glue(), opaque="true", max_states=10).run()
        self.assertIn(("a", (-1,), ()), set(result.states))
        self.assertEqual(["OPAQUE"], result.opaque_clauses)

    def test_a_clause_is_evaluated_once_per_value_of_what_it_reads(self):
        counting = Counting(python_glue())
        Explorer(counter(), NO_TWINS, evaluator=counting).run()
        guards = [r for r in counting.requests if r["op"] == "guard" and r["clause"] == "n > 0"]
        self.assertEqual(3, sum(len(r["bindings"]) for r in guards))

    def test_an_assignment_must_give_a_value_to_exactly_the_writes(self):
        lying = Lying(python_glue())
        with self.assertRaises(ValueError) as cm:
            Explorer(counter(), NO_TWINS, evaluator=lying).run()
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
        self.prose = Explorer(Diagram.from_json(EXPANSION), GLOBAL_TWINS).run()
        self.diagram = Diagram.from_json(relabel(EXPANSION))
        self.twins = Twins(domains=GLOBAL_TWINS.domains, clauses={})

    def test_a_formal_expansion_needs_no_twins_and_finds_the_same(self):
        result = Explorer(self.diagram, self.twins, evaluator=python_glue()).run()
        self.assertEqual(set(self.prose.states), set(result.states))
        self.assertEqual(self.prose.deadlocks, result.deadlocks)

    def test_partial_order_reduction_reads_what_the_glue_says_a_clause_reads(self):
        from csdfbmc.tests.test_promoted import EXPANSION, GLOBAL_TWINS
        prose = Explorer(Diagram.from_json(EXPANSION), GLOBAL_TWINS, por=True).run()
        result = Explorer(self.diagram, self.twins, evaluator=python_glue(), por=True).run()
        self.assertEqual(set(prose.states), set(result.states))
        self.assertEqual(prose.deadlocks, result.deadlocks)

    def test_symmetry_is_refused_since_the_checker_cannot_read_the_clauses(self):
        twins = Twins(domains=self.twins.domains, clauses={}, symmetric=("口座ID",), sources=[("twins.py", "DOMAINS = {}")])
        with self.assertRaises(ValueError) as cm:
            Explorer(self.diagram, twins, evaluator=python_glue(), symmetry=True)
        self.assertIn("through a glue", str(cm.exception))


class Counting(Evaluator):
    def __init__(self, inner: Evaluator):
        self.inner, self.language, self.requests = inner, inner.language, []

    def _send(self, request: dict) -> dict:
        self.requests.append(request)
        return self.inner._send(request)


class Lying(Counting):
    """Says a post writes n but answers an assignment to m as well."""

    def _send(self, request: dict) -> dict:
        answer = super()._send(request)
        if request["op"] == "post" and request["clause"] == "[{'n': n - 1}]":
            return {"results": [[dict(a, m=0) for a in r] for r in answer["results"]]}
        return answer


if __name__ == "__main__":
    unittest.main()
