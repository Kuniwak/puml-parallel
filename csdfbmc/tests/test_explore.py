"""The walk: the states a diagram reaches, and what it finds among them."""
import unittest

from csdfbmc import environment
from csdfbmc.checks import Checks
from csdfbmc.independence import check_independent
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, bound, builtin, diagram, edge, end, eq, exists, family, fld, idx
from csdfbmc.values import FrozenMap, Rec, Sample, sample

# A counter from 2 down to 0, then stop: n is decremented while positive.
COUNTER = diagram({"a": ["n"], "b": []}, "a", init_post=[atom("$n' == 2")], edges=[
    edge("a", "a", "dec", guard=[atom("$n > 0")], post=[atom("$n' == $n - 1")]),
    edge("a", "b", "stop", guard=[atom("$n == 0")]),
])

# COUNTER with a bump back up at n = 0, so the walk can avoid stop forever.
BUMPING = dict(COUNTER, edges=COUNTER["edges"] + [edge("a", "a", "bump", guard=[atom("$n == 0")], post=[atom("$n' == $n + 1")])])

# A balance that deposits raise by amount minus fee and withdrawals reduce by
# amount plus fee.
LEDGER = diagram({"a": ["残高"]}, "a", init_post=[atom("$残高' == 0")], edges=[
    edge("a", "a", "入庫?額", post=[atom("$残高' == $残高 + $額 - 1")]),
    edge("a", "a", "出庫?額", guard=[atom("$額 + 1 <= $残高")], post=[atom("$残高' == $残高 - $額 - 1")]),
])


def ledger_checks(**extra):
    return Checks(types={"額": [1, 2, 3]}, **extra)


def inst(state, **fields):
    return Rec(state, FrozenMap(fields))


# One family by hand, as csdfpromote writes it: OPEN creates an instance,
# CLOSE moves it, DROP deletes it.
ACCOUNTS = diagram({"r": ["m"]}, "r", init_post=[builtin("empty", "m'")], edges=[
    edge("r", "r", "OPEN?k", guard=[builtin("notin", "k", "m")],
         post=[builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="open"), atom("${} == 0", fld(idx("m'", "k"), "bal"))]),
    edge("r", "r", "CLOSE?k", guard=[builtin("at", idx("m", "k"), state="open")],
         post=[builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="closed"),
               eq(fld(idx("m'", "k"), "bal"), fld(idx("m", "k"), "bal"))]),
    edge("r", "r", "DROP?k", guard=[builtin("at", idx("m", "k"), state="closed")],
         post=[builtin("frame", "m'", "m", "k"), builtin("notin", "k", "m'")]),
], families=[family("m", "none", {"none": [], "open": ["bal"], "closed": ["bal"]})])


class WalkTest(unittest.TestCase):
    def test_reaches_every_valuation_the_clauses_admit(self):
        # Execute
        result = fixture.explorer(COUNTER).run()

        # Assert
        self.assertEqual([("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())], result.states)

    def test_reports_a_state_with_no_way_out_as_a_deadlock(self):
        # Execute
        result = fixture.explorer(COUNTER).run()

        # Assert
        self.assertEqual([("b", (), ())], result.deadlocks)
        self.assertEqual(["dec", "dec", "stop"], result.trace_to(result.deadlocks[0]))

    def test_an_end_edge_whose_guard_holds_is_a_way_out(self):
        tests = {"an end edge": ([end("b")], []), "one whose guard does not hold": ([end("b", [atom("1 > 2")])], [("b", (), ())])}
        for name, (ends, want) in tests.items():
            with self.subTest(name):
                # Execute
                result = fixture.explorer(dict(COUNTER, ends=ends)).run()

                # Assert
                self.assertEqual(want, result.deadlocks)

    def test_finds_a_tau_cycle_among_the_reachable_states(self):
        # Setup
        d = diagram({"a": [], "b": []}, "a", edges=[edge("a", "b", "tau"), edge("b", "a", "tau")])

        # Execute
        result = fixture.explorer(d).run()

        # Assert
        self.assertEqual([[("a", (), ()), ("b", (), ())]], result.tau_cycles)

    def test_the_bound_on_states_stops_the_walk(self):
        # Execute
        result = fixture.explorer(LEDGER, ledger_checks(), max_states=5).run()

        # Assert
        self.assertTrue(result.truncated)
        self.assertEqual(5, len(result.states))

    def test_a_post_that_admits_no_successor_is_reported(self):
        # Setup
        d = diagram({"a": ["n"]}, "a", init_post=[atom("$n' == 0")], edges=[edge("a", "a", "go", post=[atom("$n' in ()")])])

        # Execute
        result = fixture.explorer(d).run()

        # Assert
        self.assertEqual([(("a", (0,), ()), "go")], result.empty_posts)
        self.assertEqual([("a", (0,), ())], result.deadlocks)

    def test_a_second_run_reports_what_the_first_did(self):
        # Setup
        d = diagram({"a": ["n"]}, "a", init_post=[atom("$n' == 0")], edges=[edge("a", "a", "go", post=[atom("$n' in ()")])])
        explorer = fixture.explorer(d)
        first = explorer.run()

        # Execute
        again = explorer.run()

        # Assert
        self.assertEqual(first.empty_posts, again.empty_posts)
        self.assertEqual(1, len(again.empty_posts))


class FieldTest(unittest.TestCase):
    def test_a_received_field_ranges_over_its_type(self):
        # Setup
        d = diagram({"a": ["n"]}, "a", init_post=[atom("$n' == 0")], edges=[edge("a", "a", "set?x", post=[eq("n'", "x")])])

        # Execute
        result = fixture.explorer(d, Checks(types={"x": [1, 2]})).run()

        # Assert
        self.assertEqual({0, 1, 2}, {s[1][0] for s in result.states})

    def test_a_field_an_equality_of_the_guard_determines_is_computed_not_enumerated(self):
        # Setup: x has no type, and 5 would lie outside one anyway (Q5).
        d = diagram({"a": ["n"]}, "a", init_post=[atom("$n' == 5")], edges=[
            edge("a", "a", "report?x", guard=[eq("x", "n")], post=[eq("n'", "n")])])

        # Execute
        result = fixture.explorer(d).run()

        # Assert
        self.assertEqual(["report(x=5)"], [label for _, label, _, _ in fixture.explorer(d).successors(result.states[0])])

    def test_a_sent_field_sends_the_value_the_step_binds(self):
        # Setup
        d = diagram({"a": ["n"]}, "a", init_post=[atom("$n' == 3")], edges=[edge("a", "a", "show!n", post=[eq("n'", "n")])])

        # Execute
        labels = [label for _, label, _, _ in fixture.explorer(d).successors(("a", (3,), ()))]

        # Assert
        self.assertEqual(["show(n=3)"], labels)

    def test_a_field_the_step_cannot_take_is_refused(self):
        tests = {
            "a sent field the step does not bind": (diagram({"a": []}, "a", edges=[edge("a", "a", "show!y")]), "sends 'y'"),
            "a received field named like a state variable": (diagram({"a": ["n"]}, "a", edges=[edge("a", "a", "set?n")]),
                                                             "receives 'n', which names a state variable"),
        }
        for name, (d, said) in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(KeyError, said):
                    fixture.explorer(d)

    def test_a_hidden_field_is_chosen_under_its_exists(self):
        # Setup: hiding c?x leaves ∃x in the guard and the post.
        d = diagram({"a": ["n"]}, "a", init_post=[atom("$n' == 0")], edges=[
            edge("a", "a", "tau", guard=[exists("x", atom("${} > $n", fixture.bound(0)))],
                 post=[exists("x", atom("${} > $n", fixture.bound(0)), eq("n'", fixture.bound(0)))])])

        # Execute
        result = fixture.explorer(d, Checks(types={"x": [1, 2]})).run()

        # Assert
        self.assertEqual({0, 1, 2}, {s[1][0] for s in result.states})


class FamilyTest(unittest.TestCase):
    """A map of instances, as csdfpromote writes it, walked by the builtins."""

    def test_instances_are_created_moved_and_deleted(self):
        # Execute
        result = fixture.explorer(ACCOUNTS, Checks(types={"k": ["a"]})).run()

        # Assert
        self.assertEqual([("r", (FrozenMap(),), ()), ("r", (FrozenMap({"a": inst("open", bal=0)}),), ()),
                          ("r", (FrozenMap({"a": inst("closed", bal=0)}),), ())], result.states)
        self.assertEqual([], result.deadlocks)

    def test_a_glue_never_sees_an_instance_s_state(self):
        # Setup: OPEN only while every instance the glue sees is its fields.
        d = dict(ACCOUNTS, edges=[dict(ACCOUNTS["edges"][0], guard=ACCOUNTS["edges"][0]["guard"] + [atom("all(set(r) == {'bal'} for r in $m.values())")])]
                 + ACCOUNTS["edges"][1:])

        # Execute
        result = fixture.explorer(d, Checks(types={"k": ["a", "b"]})).run()

        # Assert
        self.assertIn(("r", (FrozenMap({"a": inst("open", bal=0), "b": inst("open", bal=0)}),), ()), result.states)


class EnvironmentTest(unittest.TestCase):
    """An atom the checks module declares is a function of the environment;
    every other atom is the glue's."""

    FROZEN = diagram({"a": ["n"]}, "a", init_post=[atom("$n' == 0")], edges=[
        edge("a", "a", "inc", guard=[atom("$n が凍結されていない"), atom("$n < 2")], post=[atom("$n' == $n + 1")])])

    def test_the_environment_decides_an_atom_it_declares(self):
        tests = {"given true": (True, 3), "given false": (False, 1), "given as a function": (lambda n: n < 1, 2)}
        for name, (given, states) in tests.items():
            with self.subTest(name):
                # Setup
                env = environment.over({"$_0 が凍結されていない": given}, {}, None)

                # Execute
                result = fixture.explorer(self.FROZEN, environment=env).run()

                # Assert
                self.assertEqual(states, len(result.states))
                self.assertEqual(["$_0 が凍結されていない"], result.environment["decided"])

    def test_an_atom_the_environment_does_not_declare_is_the_glue_s(self):
        with self.assertRaisesRegex(Exception, "が凍結されていない"):
            fixture.explorer(self.FROZEN).run()

    def test_a_seed_chooses_only_what_is_declared_and_walks_the_same_states_again(self):
        # Setup
        env = environment.over({"$_0 が凍結されていない": None}, {}, 7)

        # Execute
        first, again = fixture.explorer(self.FROZEN, environment=env).run(), fixture.explorer(self.FROZEN, environment=env).run()

        # Assert
        self.assertEqual(first.states, again.states)
        self.assertEqual(7, first.environment["seed"])

    def test_what_the_environment_cannot_decide_is_refused_before_the_walk(self):
        tests = {
            "a declared function nothing gives, with no seed": ({"$_0 が凍結されていない": None}, "nothing decides the environment's functions '\\$_0 が凍結されていない'"),
            "a declared name of no atom": ({"$_0 が凍結されていない": True, "$_0 が赤い": True}, "names no atom of the diagram"),
            "a function of other arguments": ({"$_0 が凍結されていない": lambda a, b: True}, "has 1 argument"),
        }
        for name, (module, said) in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(ValueError, said):
                    fixture.explorer(self.FROZEN, environment=environment.over(module, {}, None))


class PropertyTest(unittest.TestCase):
    def test_a_broken_invariant_names_the_state_and_the_trace(self):
        # Setup
        checks = ledger_checks(invariants={"残高は 4 未満": lambda 残高: 残高 < 4})

        # Execute
        result = fixture.explorer(LEDGER, checks, max_states=200).run()

        # Assert
        name, state = result.violations[0]
        self.assertEqual(("残高は 4 未満", ("a", (4,), ())), (name, state))
        self.assertEqual(["入庫(額=3)", "入庫(額=3)"], result.trace_to(state))

    def test_ghosts_follow_the_events_and_invariants_may_read_them(self):
        # Setup
        checks = ledger_checks(
            ghosts={"総入庫": (0, {"入庫": lambda 総入庫, 額: 総入庫 + 額}), "総出庫": (0, {"出庫": lambda 総出庫, 額: 総出庫 + 額}),
                    "総fee": (0, {"入庫": lambda 総fee: 総fee + 1, "出庫": lambda 総fee: 総fee + 1})},
            invariants={"帳簿は総和に一致": lambda 残高, 総入庫, 総出庫, 総fee: 残高 == 総入庫 - 総出庫 - 総fee})

        # Execute
        result = fixture.explorer(LEDGER, checks, max_states=300).run()

        # Assert
        self.assertEqual([], result.violations)
        self.assertTrue(any(ghost == (3, 0, 1) for _, _, ghost in result.states))

    def test_a_cycle_that_avoids_the_event_is_reported_within_the_states_named(self):
        tests = {
            "a counter that bumps back up": (BUMPING, None, 1),
            "one that only counts down": (COUNTER, None, 0),
            "within the state it loops in": (BUMPING, ["a"], 1),
            "within another state": (BUMPING, ["b"], 0),
        }
        for name, (d, within, cycles) in tests.items():
            with self.subTest(name):
                # Execute
                result = fixture.explorer(d, avoid="stop", within=within).run()

                # Assert
                self.assertEqual(cycles, len(result.avoiding_cycles))

    def test_findings_of_one_shape_are_one_group_with_a_count(self):
        # Setup: a deadlock at b after stop, met from each start value.
        d = dict(COUNTER, init_post=[atom("$n' in (0, 1)")])

        # Execute
        groups = fixture.explorer(d).run().groups()

        # Assert
        self.assertEqual([("deadlock", "b", "stop", 1)], [(g["kind"], g["at"], g["after"], g["count"]) for g in groups])

    def test_first_stops_the_walk_at_the_first_finding(self):
        # Execute
        result = fixture.explorer(LEDGER, ledger_checks(invariants={"残高は 1 未満": lambda 残高: 残高 < 1}), first=True).run()

        # Assert
        self.assertTrue(result.stopped_early)
        self.assertEqual(("残高は 1 未満", ("a", (1,), ())), result.violations[0])
        self.assertEqual(3, len(result.states))


# ACCOUNTS without DROP, so an ID once opened is never given back; and a STOP
# out of the map, after which nothing happens whatever the IDs.
OPENING = dict(ACCOUNTS, states=["r", "s"], vars={"r": ["m"], "s": []}, edges=ACCOUNTS["edges"][:2] + [
    edge("r", "s", "STOP", guard=[builtin("at", idx("m", "k"), state="closed")]) | {"fields": [{"dir": "?", "name": "k"}]},
])


class SampleTest(unittest.TestCase):
    """A type given as a sample has more values than the walk ranges over: a
    state with no way out that a value occurring nowhere would give one is
    where the bound was reached, not a deadlock."""

    def test_a_dead_end_is_told_from_a_deadlock_by_a_value_of_the_sample_occurring_nowhere(self):
        closed = ("r", (FrozenMap({"a": inst("closed", bal=0)}),), ())
        tests = {
            "the bound reached when every ID is used up": (["r"], sample(["a"]), [], [closed]),
            "a deadlock however many IDs there are": (["r", "s"], sample(["a"]), [("s", (), ())], []),
            "a deadlock when the type is not a sample": (["r"], ["a"], [closed], []),
        }
        for name, (states, keys, deadlocks, reached) in tests.items():
            with self.subTest(name):
                # Setup
                d = dict(OPENING, edges=[e for e in OPENING["edges"] if e["dst"] in states])
                checks = Checks(types={"k": keys}, samples=frozenset({"k"}) if isinstance(keys, Sample) else frozenset())

                # Execute
                result = fixture.explorer(d, checks).run()

                # Assert
                self.assertEqual(deadlocks, result.deadlocks)
                self.assertEqual(reached, result.bound_reached)

    def test_a_value_of_the_sample_occurring_nowhere_adds_no_state(self):
        tests = {"a sample": (sample(["a", "b"]), {"k"}), "the same values, not a sample": (["a", "b"], set())}
        for name, (keys, samples) in tests.items():
            with self.subTest(name):
                # Setup
                d = dict(OPENING, edges=OPENING["edges"][:2])

                # Execute
                result = fixture.explorer(d, Checks(types={"k": keys}, samples=frozenset(samples))).run()

                # Assert
                self.assertEqual(9, len(result.states))


class DataIndependenceTest(unittest.TestCase):
    """One value occurring nowhere stands for every value beyond the sample
    only if the diagram tells the values of a sample type apart by nothing
    but =, ≠ and the keys of maps: anything else is refused before the walk."""

    def opening(self, *guard):
        e = OPENING["edges"][0]
        return dict(OPENING, edges=[dict(e, guard=e["guard"] + list(guard))] + OPENING["edges"][1:2])

    def test_a_sample_type_told_apart_by_more_than_equality_is_refused(self):
        tests = {
            "ordered by an atom": ([atom("$k > 'a'")], {}, "argument 0 is ordered"),
            "compared with a constant by an atom": ([atom("$k != 'a'")], {}, "the constant 'a'"),
            "a map keyed by it sorted by an atom": ([atom("sorted($m) == sorted($m)")], {}, "sorted orders"),
            "bound by an exists and ordered": ([exists("k", atom("${} > 'a'", bound(0)))], {}, "argument 0 is ordered"),
            "applied by an environment function": ([eq({"kind": "app", "fn": "f", "args": [{"kind": "free", "name": "k"}]}, "k")],
                                                   {"f": lambda k: k}, "the environment function f"),
            "read by an atom of the environment": ([atom("$k が赤い")], {"$_0 が赤い": True}, "a function of the environment"),
        }
        for name, (guard, given, said) in tests.items():
            with self.subTest(name):
                # Setup
                checks = Checks(types={"k": sample(["a"])}, samples=frozenset({"k"}), environment=given)
                env = environment.over({n: g for n, g in given.items() if n.startswith("$")}, {}, None)

                # Execute, Assert
                with self.assertRaisesRegex(ValueError, rf"OPEN\?k \(line 0\), guard: .*{said}"):
                    check_independent(fixture.load(self.opening(*guard)), checks, fixture.glue(), env)

    def test_a_sample_type_told_apart_by_equality_and_keys_only_is_walked(self):
        tick = edge("r", "r", "TICK?n", guard=[atom("$n > 0")], post=[eq("m'", "m")])
        tests = {
            "the builtins": self.opening(builtin("eq", "k", "k"), builtin("notin", "k", "m")),
            "an atom equating it": self.opening(atom("$k == $k")),
            "an atom counting the map keyed by it": self.opening(atom("len($m) < 3")),
            "an atom ordering a value of no sample type": dict(OPENING, edges=OPENING["edges"][:2] + [tick]),
        }
        for name, d in tests.items():
            with self.subTest(name):
                # Setup
                checks = Checks(types={"k": sample(["a"]), "n": [1]}, samples=frozenset({"k"}))

                # Execute
                check_independent(fixture.load(d), checks, fixture.glue(), environment.NONE)
                result = fixture.explorer(d, checks).run()

                # Assert
                self.assertEqual([], result.deadlocks)


class ReplayTest(unittest.TestCase):
    def test_a_deadlock_carries_its_replay(self):
        # Execute
        result = fixture.explorer(COUNTER).run()

        # Assert
        self.assertEqual({"initial": [2], "steps": [
            {"event": "dec", "src": "a", "dst": "a", "line": 0, "values": [1]},
            {"event": "dec", "src": "a", "dst": "a", "line": 0, "values": [0]},
            {"event": "stop", "src": "a", "dst": "b", "line": 0, "values": []},
        ]}, result.replay_to(result.deadlocks[0]))

    def test_an_instance_is_encoded_with_its_state(self):
        # Execute
        result = fixture.explorer(ACCOUNTS, Checks(types={"k": ["a"]})).run()

        # Assert
        self.assertEqual([{"a": {"at": "open", "fields": {"bal": 0}}}], result.replay_to(result.states[1])["steps"][0]["values"])


class CandidatesTest(unittest.TestCase):
    """What a REPL asks at a value prompt."""

    def test_initial_candidates_are_the_start_post_s_valuations(self):
        self.assertEqual([[2]], fixture.explorer(COUNTER).initial_candidates())

    def test_candidates_of_an_edge_come_per_value_of_the_fields(self):
        # Execute
        got = fixture.explorer(LEDGER, ledger_checks()).candidates("a", [3], 1)

        # Assert
        self.assertEqual([{"params": {"額": 1}, "values": [1]}, {"params": {"額": 2}, "values": [0]}], got)

    def test_a_bad_edge_index_is_refused(self):
        with self.assertRaisesRegex(IndexError, "no edge 5"):
            fixture.explorer(COUNTER).candidates("a", [1], 5)


if __name__ == "__main__":
    unittest.main()
