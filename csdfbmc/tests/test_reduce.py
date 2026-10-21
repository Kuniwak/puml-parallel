"""Partial-order and symmetry reduction: fewer states walked, the same
findings reported."""
import unittest

from csdfbmc.environment import NONE
from csdfbmc.tests import fixture

from csdfbmc.model import Diagram
from csdfbmc.checks import Checks
from csdfbmc.explore import Explorer
from csdfbmc.tests.test_promoted import GLOBAL_CHECKS, GLUE_EXPANSION

# The checks module as a module would spell it, for the symmetry check,
# which reads the source of the checks (ghosts and invariants) itself.
CHECKS_SOURCE = '''
TYPES = {"口座ID": ["a1", "a2"], "額": [0, 1, 2]}
SYMMETRIC = ["口座ID"]
'''


def without_close(expansion):
    """expansion without CLOSE: an instance never returns to where it was, so
    the cycle proviso does not force every state to be expanded in full."""
    return {**expansion,
            "diagram": {**expansion["diagram"], "edges": [e for e in expansion["diagram"]["edges"] if e["event"] != "CLOSE?口座ID"]},
            "edges": [e for e in expansion["edges"] if e["edge"]["event"] != "CLOSE?口座ID"]}


WITHOUT_CLOSE = without_close(GLUE_EXPANSION)


def walk(expansion=GLUE_EXPANSION, *, invariants=None, symmetric=(), types=None, **kw):
    """The expansion walked through the reference Python glue."""
    checks = Checks(types=types or GLOBAL_CHECKS.types, invariants=invariants or {},
                   symmetric=tuple(symmetric), sources=[("checks.py", CHECKS_SOURCE)])
    return fixture.explorer(expansion, checks, **kw)


def with_constrain(text: str) -> dict:
    """GLUE_EXPANSION with a constrain of text on OPEN."""
    exp = {**GLUE_EXPANSION, "edges": [dict(e) for e in GLUE_EXPANSION["edges"]]}
    exp["edges"][0]["guard"] = exp["edges"][0]["guard"] + [{"kind": "constrain", "form": "text", "text": text, "line": 30}]
    return exp


class PartialOrderTest(unittest.TestCase):
    def test_walks_fewer_states_and_finds_the_same_deadlocks(self):
        # The constrain reads accounts only as accounts[口座ID], which the
        # glue says when asked beyond, so the instances move independently.
        full, reduced = walk(WITHOUT_CLOSE).run(), walk(WITHOUT_CLOSE, por=True).run()
        self.assertLess(len(reduced.states), len(full.states))
        self.assertEqual(set(full.deadlocks), set(reduced.deadlocks))
        for s in reduced.deadlocks:
            self.assertEqual(full.trace_to(s) != [], reduced.trace_to(s) != [])

    def test_a_glue_that_does_not_answer_beyond_ties_the_instances(self):
        # Setup
        from csdfbmc.cone import Cone
        from csdfbmc.tests.test_labels import NotAnswering, glue, python_glue
        d = fixture.diagram(WITHOUT_CLOSE)
        checks = Checks(types=GLOBAL_CHECKS.types)

        # Execute
        full = walk(WITHOUT_CLOSE).run()
        unasked = Explorer(d, checks, meaning=glue(NotAnswering(python_glue())), cone=Cone.keep_all(), environment=NONE, por=True).run()

        # Assert
        self.assertEqual(len(full.states), len(unasked.states))

    def test_a_constrain_that_reads_the_whole_map_ties_the_instances(self):
        # len(accounts) < 1 on OPEN: opening a1 can disable opening a2, so
        # the instances are not independent.
        exp = without_close(with_constrain("len(accounts) < 1"))
        self.assertEqual(len(walk(exp).run().states), len(walk(exp, por=True).run().states))

    def test_an_invariant_that_reads_the_map_keeps_its_moves_visible(self):
        inv = {"残高は上限以下": lambda accounts: all(r.残高 <= 3 for r in accounts.values() if "残高" in r)}
        self.assertEqual(len(walk(WITHOUT_CLOSE, invariants=inv).run().states), len(walk(WITHOUT_CLOSE, invariants=inv, por=True).run().states))

    def test_an_avoided_event_is_never_reduced_away(self):
        full, reduced = walk(avoid="DEPOSIT").run(), walk(avoid="DEPOSIT", por=True).run()
        self.assertTrue(full.avoiding_cycles)
        self.assertEqual(bool(full.avoiding_cycles), bool(reduced.avoiding_cycles))


class SymmetryTest(unittest.TestCase):
    """Symmetry asks the glue whether a clause tells the IDs apart, and reads
    the checks module's source for the ghosts and invariants."""

    def test_states_that_differ_by_a_renaming_of_the_ids_are_one(self):
        full, reduced = walk().run(), walk(symmetric=("口座ID",), symmetry=True).run()
        self.assertLess(len(reduced.states), len(full.states))
        self.assertEqual(len(full.deadlocks), len(reduced.deadlocks))

    def test_the_trace_is_a_run_of_the_diagram(self):
        reduced = walk(symmetric=("口座ID",), symmetry=True).run()
        explorer = walk()
        full = explorer.run()
        for s in reduced.states:
            self.assertIn(s, full.parents)
            if reduced.parents[s] is not None:
                prev, label, _ = reduced.parents[s]
                self.assertIn(s, [t for _, l, t, _ in explorer.successors(prev) if l == label])

    def test_a_clause_that_compares_an_id_with_a_constant_is_refused(self):
        with self.assertRaisesRegex(ValueError, "a1"):
            walk(with_constrain("口座ID == 'a1'"), symmetric=("口座ID",), symmetry=True)

    def test_a_clause_that_orders_ids_is_refused(self):
        with self.assertRaisesRegex(ValueError, "口座ID"):
            walk(with_constrain("all(口座ID < j for j in accounts)"), symmetric=("口座ID",), symmetry=True)

    def test_a_domain_naming_some_ids_only_is_refused(self):
        with self.assertRaisesRegex(ValueError, "相手ID"):
            walk(symmetric=("口座ID",), types=dict(GLOBAL_CHECKS.types, 相手ID=["a1"]), symmetry=True)

    def test_symmetry_without_a_declaration_is_refused(self):
        with self.assertRaisesRegex(ValueError, "SYMMETRIC"):
            walk(symmetry=True)

    def test_both_reductions_together_find_the_same_deadlocks(self):
        full, reduced = walk(WITHOUT_CLOSE).run(), walk(WITHOUT_CLOSE, symmetric=("口座ID",), por=True, symmetry=True).run()
        self.assertLess(len(reduced.states), len(full.states))
        self.assertEqual(len(full.deadlocks), len(reduced.deadlocks))


if __name__ == "__main__":
    unittest.main()
