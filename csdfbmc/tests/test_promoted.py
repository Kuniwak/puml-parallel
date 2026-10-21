"""An expansion of a promoted diagram: the state variables are maps from
instance IDs to local records, the generated clauses have built-in meaning,
and the local clauses are read by the reference Python glue in the names of
the family's own variables."""
import unittest
from csdfbmc.tests import fixture

from csdfbmc.model import Diagram
from csdfbmc.checks import Checks
from csdfbmc.explore import Explorer
from csdfbmc.values import FrozenMap, record

# What `csdfpromote -format json` prints for a one-family global diagram whose
# local diagram is: none --OPEN?額--> open(残高) --CLOSE--> none, with a
# self-loop DEPOSIT?額 on open, and a constrain on DEPOSIT.
LOCAL = {
    "states": {
        "accNone": {"name": "未開設", "vars": [], "line": 2},
        "accOpen": {"name": "開設済み", "vars": [{"name": "残高"}], "line": 3},
    },
    "start_edge": {"dst": "accNone", "post": "", "line": 5},
    "edges": [
        {"src": "accNone", "dst": "accOpen", "event": "OPEN?額", "channel": "OPEN", "fields": [{"dir": "?", "name": "額"}], "guard": "", "post": "残高' = 額", "line": 6},
        {"src": "accOpen", "dst": "accOpen", "event": "DEPOSIT?額", "channel": "DEPOSIT", "fields": [{"dir": "?", "name": "額"}], "guard": "0 < 額", "post": "残高' = 残高 + 額", "line": 7},
        {"src": "accOpen", "dst": "accNone", "event": "CLOSE", "channel": "CLOSE", "fields": [], "guard": "残高 = 0", "post": "", "line": 8},
    ],
    "end_edge": None, "assumptions": [],
}


def gen(kind, text, **more):
    return dict(kind=kind, text=text, map="accounts", id="口座ID", **more)


EXPANSION = {
    "diagram": {
        "states": {"running": {"name": "稼働中", "vars": [{"name": "accounts", "type": "口座ID ⇸ Account"}], "line": 2}},
        "start_edge": {"dst": "running", "post": "accounts は空", "line": 5},
        "edges": [
            {"src": "running", "dst": "running", "event": "OPEN?口座ID?額", "channel": "OPEN", "fields": [{"dir": "?", "name": "口座ID"}, {"dir": "?", "name": "額"}], "guard": "g", "post": "p", "line": 0},
            {"src": "running", "dst": "running", "event": "DEPOSIT?口座ID?額", "channel": "DEPOSIT", "fields": [{"dir": "?", "name": "口座ID"}, {"dir": "?", "name": "額"}], "guard": "g", "post": "p", "line": 0},
            {"src": "running", "dst": "running", "event": "CLOSE?口座ID", "channel": "CLOSE", "fields": [{"dir": "?", "name": "口座ID"}], "guard": "g", "post": "p", "line": 0},
        ],
        "end_edge": None, "assumptions": [],
    },
    "edges": [
        {"edge": {"src": "running", "dst": "running", "event": "OPEN?口座ID?額", "channel": "OPEN", "fields": [{"dir": "?", "name": "口座ID"}, {"dir": "?", "name": "額"}], "guard": "g", "post": "p", "line": 0},
         "origin": "promote: local/ACCOUNT.puml",
         "guard": [gen("not_in_dom", "口座ID ∉ dom accounts")],
         "post": [gen("create", "accounts' = accounts ∪ {口座ID ↦ 〈開設済み〉}", state="開設済み"),
                  {"kind": "local", "form": "text", "text": "残高' = 額", "map": "accounts", "path": "local/ACCOUNT.puml", "line": 6}]},
        {"edge": {"src": "running", "dst": "running", "event": "DEPOSIT?口座ID?額", "channel": "DEPOSIT", "fields": [{"dir": "?", "name": "口座ID"}, {"dir": "?", "name": "額"}], "guard": "g", "post": "p", "line": 0},
         "origin": "promote: local/ACCOUNT.puml",
         "guard": [gen("in_dom", "口座ID ∈ dom accounts"), gen("at_state", "accounts(口座ID) ∈ 〈開設済み〉", state="開設済み"),
                   {"kind": "local", "form": "text", "text": "0 < 額", "map": "accounts", "path": "local/ACCOUNT.puml", "line": 7},
                   {"kind": "constrain", "form": "text", "text": "残高 + 額 ≤ 上限", "formal": "accounts[口座ID].残高 + 額 <= 3", "line": 20}],
         "post": [gen("update", "accounts' = accounts ⊕ {口座ID ↦ 〈開設済み〉}", state="開設済み"),
                  {"kind": "local", "form": "text", "text": "残高' = 残高 + 額", "map": "accounts", "path": "local/ACCOUNT.puml", "line": 7}]},
        {"edge": {"src": "running", "dst": "running", "event": "CLOSE?口座ID", "channel": "CLOSE", "fields": [{"dir": "?", "name": "口座ID"}], "guard": "g", "post": "p", "line": 0},
         "origin": "promote: local/ACCOUNT.puml",
         "guard": [gen("in_dom", "口座ID ∈ dom accounts"), gen("at_state", "accounts(口座ID) ∈ 〈開設済み〉", state="開設済み"),
                   {"kind": "local", "form": "text", "text": "残高 = 0", "map": "accounts", "path": "local/ACCOUNT.puml", "line": 8}],
         "post": [gen("delete", "accounts' = {口座ID} ⩤ accounts")]},
    ],
    "promotes": [{"map": "accounts", "id_param": "口座ID", "type": "Account", "path": "local/ACCOUNT.puml", "alias": "runningAccounts", "in": "running", "line": 4}],
    "locals": {"local/ACCOUNT.puml": LOCAL},
}


# The expansion with every clause written in the reference Python glue's
# language, as a front end renders the formal labels in place of the prose.
FORMAL = {
    "accounts は空": "[{'accounts': FrozenMap()}]",
    "残高' = 額": "[{'残高': 額}]",
    "0 < 額": "0 < 額",
    "残高' = 残高 + 額": "[{'残高': 残高 + 額}]",
    "残高 = 0": "残高 == 0",
    "残高 + 額 ≤ 上限": "accounts[口座ID].残高 + 額 <= 3",
    "残高' = 残高": "[{'残高': 残高}]",
}


def relabel(value):
    """value with every clause written in Python in place of its prose."""
    if isinstance(value, dict):
        return {k: (FORMAL.get(v, v) if k in ("text", "post", "guard") and isinstance(v, str) else relabel(v)) for k, v in value.items()}
    if isinstance(value, list):
        return [relabel(v) for v in value]
    return value


GLUE_EXPANSION = relabel(EXPANSION)

# The checks of the expansion: the domains of the received fields.
GLOBAL_CHECKS = Checks(types={"口座ID": ["a1", "a2"], "額": [0, 1, 2]})


class PromotedModelTest(unittest.TestCase):
    def test_an_expansion_is_recognised_and_keeps_its_clauses(self):
        d = fixture.diagram(EXPANSION)
        self.assertEqual(("accounts",), d.vars["running"])
        self.assertEqual({"accounts"}, set(d.promotes))
        self.assertEqual("口座ID", d.promotes["accounts"].id_param)
        self.assertEqual(("残高",), d.locals["local/ACCOUNT.puml"].vars_of("開設済み"))
        deposit = next(e for e in d.edges if e.name == "DEPOSIT")
        self.assertEqual(["in_dom", "at_state", "local", "constrain"], [c.kind for c in deposit.guard])


class PromotedExplorerTest(unittest.TestCase):
    def test_instances_are_created_moved_and_deleted_in_the_map(self):
        result = fixture.explorer(GLUE_EXPANSION, GLOBAL_CHECKS).run()
        maps = {s[1][0] for s in result.states}
        self.assertIn(FrozenMap(), maps)
        self.assertIn(FrozenMap({"a1": FrozenMap({"state": "開設済み", "残高": 1})}), maps)
        self.assertIn(FrozenMap({"a1": FrozenMap({"state": "開設済み", "残高": 1}), "a2": FrozenMap({"state": "開設済み", "残高": 2})}), maps)
        # DEPOSIT keeps 残高 ≤ 3 (the constrain) and CLOSE needs 残高 = 0.
        self.assertTrue(all(r.残高 <= 3 for m in maps for r in m.values()))
        # Both IDs used, both balances at the cap: DEPOSIT is barred by the
        # constrain and CLOSE needs 残高 = 0, so nothing can move.
        full = FrozenMap({"a1": FrozenMap({"state": "開設済み", "残高": 3}), "a2": FrozenMap({"state": "開設済み", "残高": 3})})
        self.assertEqual([("running", (full,), ())], result.deadlocks)
        self.assertEqual([], result.unconstrained)

    def test_the_trace_names_the_instance(self):
        result = fixture.explorer(GLUE_EXPANSION, GLOBAL_CHECKS).run()
        target = ("running", (FrozenMap({"a2": FrozenMap({"state": "開設済み", "残高": 0})}),), ())
        self.assertIn(target, result.states)
        self.assertEqual(["OPEN(口座ID=a2, 額=0)"], result.trace_to(target))

    def test_a_local_post_that_misses_a_local_variable_is_a_finding(self):
        exp = {**GLUE_EXPANSION, "edges": [dict(e) for e in GLUE_EXPANSION["edges"]]}
        opened = exp["edges"][0]
        opened["post"] = [dict(c, text="[{}]") if c["kind"] == "local" else c for c in opened["post"]]
        result = fixture.explorer(exp, GLOBAL_CHECKS).run()
        self.assertTrue(result.unconstrained)
        self.assertEqual(("accounts[a1].残高",), result.unconstrained[0][2])

    def test_a_tau_edge_ranges_over_the_instances(self):
        tau = {"src": "running", "dst": "running", "event": "tau", "channel": "tau", "fields": [], "guard": "g", "post": "p", "line": 0}
        exp = {**EXPANSION, "diagram": {**EXPANSION["diagram"], "edges": EXPANSION["diagram"]["edges"] + [tau]}, "edges": EXPANSION["edges"] + [
            {"edge": tau,
             "origin": "promote: local/ACCOUNT.puml",
             "guard": [gen("in_dom", "口座ID ∈ dom accounts"), gen("at_state", "accounts(口座ID) ∈ 〈開設済み〉", state="開設済み")],
             "post": [gen("update", "accounts' = accounts ⊕ {口座ID ↦ 〈開設済み〉}", state="開設済み"),
                      {"kind": "local", "form": "text", "text": "[{'残高': 残高}]", "map": "accounts", "path": "local/ACCOUNT.puml", "line": 9}]}]}
        result = fixture.explorer(relabel(exp), GLOBAL_CHECKS).run()
        self.assertTrue(result.tau_cycles)

    def test_a_tau_does_not_choose_an_instance_s_own_variable(self):
        # Setup: 残高 has a domain, but the family's clause reads the
        # instance's own 残高, which the tau must not choose.
        tau = {"src": "running", "dst": "running", "event": "tau", "channel": "tau", "fields": [], "guard": "g", "post": "p", "line": 0}
        exp = {**EXPANSION, "diagram": {**EXPANSION["diagram"], "edges": EXPANSION["diagram"]["edges"] + [tau]}, "edges": EXPANSION["edges"] + [
            {"edge": tau, "origin": "promote: local/ACCOUNT.puml",
             "guard": [{"kind": "local", "form": "text", "text": "残高 == 0", "map": "accounts", "path": "local/ACCOUNT.puml", "line": 8}], "post": []}]}
        explorer = fixture.explorer(relabel(exp), Checks(types=dict(GLOBAL_CHECKS.types, 残高=[0, 1])))

        # Execute
        chosen = explorer._chosen_by_tau(explorer.d.edges[-1])

        # Assert
        self.assertNotIn("残高", chosen)

    def test_a_dropped_clause_is_taken_as_true_and_reported(self):
        # csdfpromote -only lists a guard clause that read a left-out map as
        # dropped: the walk takes it as true and says that it did.
        exp = {**GLUE_EXPANSION, "edges": [dict(e) for e in GLUE_EXPANSION["edges"]]}
        dep = exp["edges"][1]
        dep["guard"] = dep["guard"] + [{"kind": "dropped", "text": "取引所が開いている", "maps": ["markets"], "line": 21}]
        result = fixture.explorer(exp, GLOBAL_CHECKS).run()
        self.assertEqual(fixture.explorer(GLUE_EXPANSION, GLOBAL_CHECKS).run().states, result.states)
        self.assertEqual([{"text": "取引所が開いている", "maps": ["markets"]}], result.to_json()["dropped_clauses"])


class EmptyStartTest(unittest.TestCase):
    """csdfpromote carries on the start edge that every map starts empty, as
    a clause whose meaning is built in."""

    def expansion(self, start) -> dict:
        x = {**GLUE_EXPANSION, "start": start}
        x["diagram"] = {**GLUE_EXPANSION["diagram"], "start_edge": {"dst": "running", "post": "accounts は空", "line": 5}}
        return x

    def test_the_maps_start_empty(self):
        for name, start in [
            ("alone", [{"kind": "empty", "text": "accounts = ∅", "map": "accounts"}]),
            ("beside an author clause that agrees",
             [{"kind": "local", "form": "text", "text": "[{'accounts': FrozenMap()}]", "line": 5}, {"kind": "empty", "text": "accounts = ∅", "map": "accounts"}]),
        ]:
            with self.subTest(name):
                explorer = fixture.explorer(self.expansion(start), GLOBAL_CHECKS)
                self.assertEqual([("running", (FrozenMap(),), ())], explorer.initial())

    def test_an_expansion_from_before_start_is_read_from_the_start_edge_text(self):
        x = {k: v for k, v in self.expansion([]).items() if k != "start"}
        self.assertEqual(("accounts は空",), tuple(c.text for c in fixture.diagram(x).start_post))

    def test_a_start_that_is_not_a_list_is_refused(self):
        with self.assertRaises(ValueError):
            fixture.diagram(self.expansion(None))


if __name__ == "__main__":
    unittest.main()
