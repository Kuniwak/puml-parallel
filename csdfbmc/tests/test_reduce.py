"""Partial-order and symmetry reduction: fewer states walked, the same
findings reported."""
import unittest

from csdfbmc.model import Diagram
from csdfbmc.twins import Twins
from csdfbmc.explore import Explorer
from csdfbmc.tests.test_promoted import EXPANSION, GLOBAL_TWINS

# The twins of GLOBAL_TWINS as a module would spell them, for the symmetry
# check, which reads the source.
GLOBAL_SOURCE = '''
DOMAINS = {"口座ID": ["a1", "a2"], "額": [0, 1, 2]}
CLAUSES = {"accounts は空": lambda: [{"accounts": FrozenMap()}]}
SYMMETRIC = ["口座ID"]
'''
LOCAL_SOURCE = '''
CLAUSES = {
    "残高' = 額": lambda 額: [{"残高": 額}],
    "0 < 額": lambda 額: 0 < 額,
    "残高' = 残高 + 額": lambda 残高, 額: [{"残高": 残高 + 額}],
    "残高 = 0": lambda 残高: 残高 == 0,
}
'''


def twins(*, invariants=None, global_source=GLOBAL_SOURCE, symmetric=("口座ID",)) -> Twins:
    local = GLOBAL_TWINS.locals["local/ACCOUNT.puml"]
    return Twins(
        domains=GLOBAL_TWINS.domains, clauses=GLOBAL_TWINS.clauses, invariants=invariants or {},
        locals={"local/ACCOUNT.puml": Twins(domains={}, clauses=local.clauses, sources=[("local.py", LOCAL_SOURCE)])},
        symmetric=tuple(symmetric), sources=[("global.py", global_source)],
    )


# Without CLOSE an instance never returns to where it was, so the cycle
# proviso does not force every state to be expanded in full.
WITHOUT_CLOSE = {**EXPANSION,
                 "diagram": {**EXPANSION["diagram"], "edges": [e for e in EXPANSION["diagram"]["edges"] if e["event"] != "CLOSE(口座ID)"]},
                 "edges": [e for e in EXPANSION["edges"] if e["edge"]["event"] != "CLOSE(口座ID)"]}


def walk(expansion=EXPANSION, **kw):
    return Explorer(Diagram.from_json(expansion), twins(**{k: kw.pop(k) for k in ("invariants", "global_source") if k in kw}), **kw).run()


class PartialOrderTest(unittest.TestCase):
    def test_walks_fewer_states_and_finds_the_same_deadlocks(self):
        full, reduced = walk(WITHOUT_CLOSE), walk(WITHOUT_CLOSE, por=True)
        self.assertLess(len(reduced.states), len(full.states))
        self.assertEqual(set(full.deadlocks), set(reduced.deadlocks))
        for s in reduced.deadlocks:
            self.assertEqual(full.trace_to(s) != [], reduced.trace_to(s) != [])

    def test_a_constrain_that_reads_the_whole_map_ties_the_instances(self):
        # len(accounts) < 2 on OPEN: opening a1 can disable opening a2, so
        # the instances are not independent.
        exp = {**WITHOUT_CLOSE, "edges": [dict(e) for e in WITHOUT_CLOSE["edges"]]}
        exp["edges"][0]["guard"] = exp["edges"][0]["guard"] + [{"kind": "constrain", "text": "口座は1つまで", "formal": "len(accounts) < 1", "line": 30}]
        self.assertEqual(len(walk(exp).states), len(walk(exp, por=True).states))

    def test_an_invariant_that_reads_the_map_keeps_its_moves_visible(self):
        inv = {"残高は上限以下": lambda accounts: all(r.残高 <= 3 for r in accounts.values() if "残高" in r)}
        self.assertEqual(len(walk(WITHOUT_CLOSE, invariants=inv).states), len(walk(WITHOUT_CLOSE, invariants=inv, por=True).states))

    def test_an_avoided_event_is_never_reduced_away(self):
        full, reduced = walk(avoid="DEPOSIT"), walk(avoid="DEPOSIT", por=True)
        self.assertTrue(full.avoiding_cycles)
        self.assertEqual(bool(full.avoiding_cycles), bool(reduced.avoiding_cycles))


class SymmetryTest(unittest.TestCase):
    def test_states_that_differ_by_a_renaming_of_the_ids_are_one(self):
        full, reduced = walk(), walk(symmetry=True)
        self.assertLess(len(reduced.states), len(full.states))
        self.assertEqual(len(full.deadlocks), len(reduced.deadlocks))

    def test_the_trace_is_a_run_of_the_diagram(self):
        reduced = walk(symmetry=True)
        full = walk()
        for s in reduced.states:
            self.assertIn(s, full.parents)
            if reduced.parents[s] is not None:
                prev, label, _ = reduced.parents[s]
                self.assertIn(s, [t for _, l, t, _ in Explorer(Diagram.from_json(EXPANSION), twins()).successors(prev) if l == label])

    def test_a_twin_that_compares_an_id_with_a_constant_is_refused(self):
        source = GLOBAL_SOURCE + 'CLAUSES["特別"] = lambda 口座ID: 口座ID == "a1"\n'
        with self.assertRaisesRegex(ValueError, "a1"):
            walk(global_source=source, symmetry=True)

    def test_a_twin_that_orders_ids_is_refused(self):
        source = GLOBAL_SOURCE + 'CLAUSES["先"] = lambda 口座ID, 相手ID: 口座ID < 相手ID\n'
        with self.assertRaisesRegex(ValueError, "口座ID"):
            walk(global_source=source, symmetry=True)

    def test_a_domain_naming_some_ids_only_is_refused(self):
        tw = twins()
        tw.domains = dict(tw.domains, 相手ID=["a1"])
        with self.assertRaisesRegex(ValueError, "相手ID"):
            Explorer(Diagram.from_json(EXPANSION), tw, symmetry=True)

    def test_symmetry_without_a_declaration_is_refused(self):
        with self.assertRaisesRegex(ValueError, "SYMMETRIC"):
            Explorer(Diagram.from_json(EXPANSION), twins(symmetric=()), symmetry=True)

    def test_both_reductions_together_find_the_same_deadlocks(self):
        full, reduced = walk(WITHOUT_CLOSE), walk(WITHOUT_CLOSE, por=True, symmetry=True)
        self.assertLess(len(reduced.states), len(full.states))
        self.assertEqual(len(full.deadlocks), len(reduced.deadlocks))


if __name__ == "__main__":
    unittest.main()
