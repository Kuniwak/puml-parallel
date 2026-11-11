"""The walk: the states a diagram reaches, and what it finds among them."""
import itertools
import unittest

from csdfbmc import environment
from csdfbmc.checks import Checks
from csdfbmc.formula import PostError
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, bound, builtin, diagram, edge, end, eq, exists, family, fld, idx, lit
from csdfbmc.values import FrozenMap, Rec, kind_key, record, sample

# A counter from 2 down to 0, then stop: n is decremented while positive.
COUNTER = diagram({"a": ["n"], "b": []}, "a", init_post=[atom("$n' == 2")], edges=[
    edge("a", "a", "dec", guard=[atom("$n > 0")], post=[atom("$n' == $n - 1")]),
    edge("a", "b", "stop", guard=[atom("$n == 0")]),
])

# COUNTER with a bump back up at n = 0, so the walk can avoid stop forever.
BUMPING = dict(COUNTER, edges=COUNTER["edges"] + [edge("a", "a", "bump", guard=[atom("$n == 0")], post=[atom("$n' == $n + 1")])])

# A stock that a delivery raises by its size minus a loss of one and a
# dispatch lowers by its size plus a loss of one.
TALLY = diagram({"a": ["在庫"]}, "a", init_post=[atom("$在庫' == 0")], edges=[
    edge("a", "a", "搬入?量", post=[atom("$在庫' == $在庫 + $量 - 1")]),
    edge("a", "a", "搬出?量", guard=[atom("$量 + 1 <= $在庫")], post=[atom("$在庫' == $在庫 - $量 - 1")]),
])


def tally_checks(**extra):
    return Checks(types={"量": [1, 2, 3]}, **extra)


def inst(state, **fields):
    return Rec(state, FrozenMap(fields))


# One family by hand, as csdfpromote writes it: OPEN creates an instance,
# CLOSE moves it, DROP deletes it.
MEMBERS = diagram({"r": ["m"]}, "r", init_post=[builtin("empty", "m'")], edges=[
    edge("r", "r", "OPEN?k", guard=[builtin("notin", "k", "m")],
         post=[builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="open"), atom("${} == 0", fld(idx("m'", "k"), "cnt"))]),
    edge("r", "r", "CLOSE?k", guard=[builtin("at", idx("m", "k"), state="open")],
         post=[builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="closed"),
               eq(fld(idx("m'", "k"), "cnt"), fld(idx("m", "k"), "cnt"))]),
    edge("r", "r", "DROP?k", guard=[builtin("at", idx("m", "k"), state="closed")],
         post=[builtin("frame", "m'", "m", "k"), builtin("notin", "k", "m'")]),
], families=[family("m", "none", {"none": [], "open": ["cnt"], "closed": ["cnt"]})])


def parts(value):
    """value and every container in it, all the way down."""
    if isinstance(value, Rec):
        yield value
        yield from parts(value.fields)
    elif isinstance(value, FrozenMap):
        yield value
        for v in value.values():
            yield from parts(v)
    elif isinstance(value, tuple):
        yield value
        for v in value:
            yield from parts(v)


class WalkTest(unittest.TestCase):
    def test_reaches_every_valuation_the_clauses_admit(self):
        # Execute
        result = fixture.explorer(COUNTER).run()

        # Assert
        self.assertEqual([("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())], result.states)

    def test_the_states_met_share_their_equal_values(self):
        # Execute
        result = fixture.example("attempts").run()

        # Assert
        kept = {}
        for s in result.states:
            for v in parts(s[1]):
                self.assertIs(kept.setdefault(kind_key(v), v), v)

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
        result = fixture.explorer(TALLY, tally_checks(), max_states=5).run()

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

    def test_values_equal_in_python_but_of_other_kinds_are_other_states(self):
        tests = {
            "a number and a truth value": (1, True),
            "an integer and a float": (1, 1.0),
            "inside a tuple": ((1,), (True,)),
            "as a key of a map": ({1: 0}, {True: 0}),
            "inside a set": (frozenset({1}), frozenset({1.0})),
        }
        for name, (first, then) in tests.items():
            with self.subTest(name):
                # Setup
                d = diagram({"a": ["n"]}, "a", init_post=[eq("n'", lit(0))], edges=[
                    edge("a", "a", "set?x", post=[eq("n'", "x")]),
                ])
                checks = Checks(types={"x": [first, then]})

                # Execute
                result = fixture.explorer(d, checks).run()

                # Assert
                self.assertEqual(3, len(result.states))
                self.assertEqual(3, len(result.parents))

    def test_a_ghost_is_frozen_and_told_apart_by_kind(self):
        # Setup
        d = diagram({"a": []}, "a", edges=[edge("a", "a", "go")])
        checks = Checks(types={}, ghosts={"g": (0, {"go": lambda g: {1} if g == 0 else {True}})})

        # Execute
        result = fixture.explorer(d, checks).run()

        # Assert
        self.assertEqual(3, len(result.states))

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

    def test_the_parts_of_a_record_field_equalities_of_the_guard_determine_are_computed(self):
        # Setup: the count the state holds, 8, lies outside the record's part.
        to, count = eq(idx("w", lit("to")), "t"), eq(idx("w", lit("count")), "n")
        guards = {"together": [to, count], "apart by a conjunct over the state": [to, atom("$n > 0"), count]}
        types = {
            "a record": record(to=["p", "q"], count=[0, 1, 2, 3]),
            "a list of records": [FrozenMap({"to": x, "count": c}) for x in ("p", "q") for c in range(4)],
        }
        for (gname, guard), (tname, w) in itertools.product(guards.items(), types.items()):
            with self.subTest(guard=gname, type=tname):
                d = diagram({"a": ["t", "n"]}, "a", init_post=[eq("t'", lit("p")), eq("n'", lit(8))], edges=[
                    edge("a", "a", "warn?w", guard=guard, post=[eq("t'", "t"), eq("n'", "n")])],
                    types={"t": "T", "n": "N"}, channels={"warn": ["W"]})
                checks = Checks(types={"T": ["p", "q"], "N": list(range(9)), "W": w})

                # Execute
                labels = [label for _, label, _, _ in fixture.explorer(d, checks).successors(("a", ("p", 8), ()))]

                # Assert
                self.assertEqual(["warn(w={'to': 'p', 'count': 8})"], labels)

    def test_an_equality_determines_its_field_after_a_conjunct_over_a_later_field(self):
        # Setup
        d = diagram({"a": ["n"]}, "a", init_post=[eq("n'", lit(5))], edges=[
            edge("a", "a", "c?v?w", guard=[atom("$w > 0"), eq("v", "n")], post=[eq("n'", "n")])],
            channels={"c": ["V", "W"]})
        x = fixture.explorer(d, Checks(types={"V": [4, 5], "W": [0, 1, 2]}), stats=True)

        # Execute
        labels = [label for _, label, _, _ in x.successors(("a", (5,), ()))]

        # Assert
        self.assertEqual(["c(v=5, w=1)", "c(v=5, w=2)"], labels)
        self.assertEqual((3, ["?w"]), (x.stats[0].tried, sorted(x.stats[0].ranged)))

    def test_a_part_no_equality_determines_ranges_over_the_values_of_the_part(self):
        # Setup
        d = diagram({"a": ["n"]}, "a", init_post=[eq("n'", lit(8))], edges=[
            edge("a", "a", "warn?w", guard=[eq(idx("w", lit("count")), "n")], post=[eq("n'", "n")])],
            types={"n": "N"}, channels={"warn": ["W"]})
        checks = Checks(types={"N": list(range(9)), "W": record(to=["p", "q"], count=[0, 1])})

        # Execute
        labels = [label for _, label, _, _ in fixture.explorer(d, checks).successors(("a", (8,), ()))]

        # Assert
        self.assertEqual(["warn(w={'to': 'p', 'count': 8})", "warn(w={'to': 'q', 'count': 8})"], labels)

    def test_a_list_of_records_some_part_of_which_no_equality_determines_is_enumerated(self):
        # Setup
        d = diagram({"a": ["n"]}, "a", init_post=[eq("n'", lit(8))], edges=[
            edge("a", "a", "warn?w", guard=[eq(idx("w", lit("count")), "n")], post=[eq("n'", "n")])],
            types={"n": "N"}, channels={"warn": ["W"]})
        checks = Checks(types={"N": list(range(9)), "W": [FrozenMap({"to": "p", "count": c}) for c in range(4)]})

        # Execute
        labels = [label for _, label, _, _ in fixture.explorer(d, checks).successors(("a", (8,), ()))]

        # Assert
        self.assertEqual([], labels)

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

    def test_a_literal_no_value_of_the_type_it_is_compared_with_is_refused(self):
        tests = {
            "a state variable after the step": (edge("a", "a", "c", post=[eq("n'", lit(5))]), {"n": [0, 1]}, "n' with the literal 5"),
            "a state variable before the step, by @ne": (edge("a", "a", "c", guard=[builtin("ne", lit(1.0), "n")]), {"n": [0, 1]}, "n with the literal 1.0"),
            "a received field of the type its channel declares": (edge("a", "a", "c?x", guard=[eq("x", lit("1"))]), {"n": [0, 1], "x": [0]}, 'x with the literal "1"'),
            "a variable ∃ binds": (edge("a", "a", "c", guard=[exists("n", eq(fixture.bound(0), lit(True)))]), {"n": [0, 1]}, "with the literal true"),
        }
        for name, (e, types, said) in tests.items():
            with self.subTest(name):
                # Setup
                d = diagram({"a": ["n"]}, "a", init_post=[eq("n'", lit(0))], edges=[e])

                # Execute, Assert
                with self.assertRaisesRegex(ValueError, said + ", which no value of its type"):
                    fixture.explorer(d, Checks(types=types))

    def test_a_literal_a_value_of_its_type_is_or_may_be_is_kept(self):
        tests = {
            "a value of the type": ({"n": [0, 1]}, lit(1)),
            "a value beyond the sample of the type": ({"n": sample([0, 1], [2])}, lit(5)),
            "a type TYPES does not declare": ({"m": [0, 1]}, lit(5)),
        }
        for name, (types, value) in tests.items():
            with self.subTest(name):
                # Setup
                d = diagram({"a": ["n"]}, "a", init_post=[eq("n'", lit(0))], edges=[edge("a", "a", "c", post=[eq("n'", value)])])

                # Execute
                got = fixture.explorer(d, Checks(types=types))

                # Assert
                self.assertIs(d["edges"][0]["post"][0]["args"][1], value)
                self.assertIsNotNone(got)

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
        result = fixture.explorer(MEMBERS, Checks(types={"k": ["a"]})).run()

        # Assert
        self.assertEqual([("r", (FrozenMap(),), ()), ("r", (FrozenMap({"a": inst("open", cnt=0)}),), ()),
                          ("r", (FrozenMap({"a": inst("closed", cnt=0)}),), ())], result.states)
        self.assertEqual([], result.deadlocks)

    def test_a_glue_never_sees_an_instance_s_state(self):
        # Setup: OPEN only while every instance the glue sees is its fields.
        d = dict(MEMBERS, edges=[dict(MEMBERS["edges"][0], guard=MEMBERS["edges"][0]["guard"] + [atom("all(set(r) == {'cnt'} for r in $m.values())")])]
                 + MEMBERS["edges"][1:])

        # Execute
        result = fixture.explorer(d, Checks(types={"k": ["a", "b"]})).run()

        # Assert
        self.assertIn(("r", (FrozenMap({"a": inst("open", cnt=0), "b": inst("open", cnt=0)}),), ()), result.states)


class PruningTest(unittest.TestCase):
    """The guard drops the values of the fields so far once its leading
    conjuncts, which read only the fields bound, are false; it is evaluated
    in the order and on the values evaluating it in full would."""

    # c?id?op?r: @at(m[id], s) reads only id; the atom reads r too.
    FAMILY = diagram({"q": ["m"]}, "q", init_post=[builtin("empty", "m'")], edges=[
        edge("q", "q", "c?id?op?r", guard=[builtin("at", idx("m", "id"), state="s"), atom("${} == ${}", "r", fld(idx("m", "id"), "v"))],
             post=[builtin("frame", "m'", "m")])],
        families=[family("m", "none", {"none": [], "s": ["v"]}, key="id")], channels={"c": ["id", "OP", "R"]})
    FAMILY_TYPES = {"id": [1, 2], "OP": ["x", "y", "z"], "R": [0, 1, 2, 3], "v": [0]}

    # c?id?r: the first conjunct reads r, the last field, so @at, which
    # reads only id and would refuse m[id], no instance, is not evaluated.
    LATE_FIRST = diagram({"q": ["m"]}, "q", init_post=[eq("m'", lit({}))], edges=[
        edge("q", "q", "c?id?r", guard=[atom("$r == 9"), builtin("at", idx("m", "id"), state="s")], post=[eq("m'", "m")])],
        channels={"c": ["id", "R"]})

    def test_the_steps_and_the_values_tried_are_those_of_the_full_product(self):
        tests = {
            "the leading conjunct false for every id": (self.FAMILY, self.FAMILY_TYPES, ("q", (FrozenMap(),), ()), 0, 24),
            "the leading conjunct true for one id": (self.FAMILY, self.FAMILY_TYPES,
                                                     ("q", (FrozenMap({1: inst("s", v=2)}),), ()), 3, 24),
            "a bound-only conjunct after one reading a later field": (self.LATE_FIRST, {"id": [1, 2], "R": [0, 1]},
                                                                      ("q", (FrozenMap({1: 5, 2: 5}),), ()), 0, 4),
        }
        for name, (d, types, state, steps, tried) in tests.items():
            with self.subTest(name):
                # Setup
                x = fixture.explorer(d, Checks(types=types))

                # Execute
                transitions = x.transitions(state)

                # Assert
                self.assertEqual((steps, tried), (len(transitions), x.tried))

    def test_the_stats_say_how_many_values_of_those_tried_were_dropped_unenumerated(self):
        # Setup: id 1 is at s, so only the 12 values with id 2 are dropped.
        x = fixture.explorer(self.FAMILY, Checks(types=self.FAMILY_TYPES), stats=True)

        # Execute
        x.transitions(("q", (FrozenMap({1: inst("s", v=2)}),), ()))

        # Assert
        self.assertEqual((24, 12), (x.stats[0].tried, x.stats[0].pruned))

    # c?id?r: the last conjunct reads only the part k of r, the first of its record.
    PART = diagram({"q": ["m"]}, "q", init_post=[eq("m'", lit({}))], edges=[
        edge("q", "q", "c?id?r", guard=[builtin("notin", "id", "m"), builtin("in", fld("r", "k"), "m")], post=[eq("m'", "m")])],
        channels={"c": ["ID", "R"]})
    PART_TYPES = {"ID": [1, 2], "R": record(k=[0, 1, 2], a=[0, 1], b=[0, 1])}

    def test_a_conjunct_reading_one_part_of_a_record_drops_the_values_of_its_other_parts(self):
        # Setup: m is empty, so @in(r.k, m) is false once r.k is bound.
        x = fixture.explorer(self.PART, Checks(types=self.PART_TYPES), stats=True)

        # Execute
        x.transitions(("q", (FrozenMap(),), ()))

        # Assert
        self.assertEqual((24, 24), (x.stats[0].tried, x.stats[0].pruned))

    def test_a_part_a_membership_decides_is_drawn_from_the_keys_of_the_map(self):
        # Setup: m has key 1 only, so @in(r.k, m) is asked of r.k = 1 only,
        # for the id 2 @notin lets through.
        x = fixture.explorer(self.PART, Checks(types=self.PART_TYPES), stats=True)
        asked = []
        holds = x.ev.holds
        x.ev.holds = lambda clause, before, *rest: asked.extend(c.op for c in clause) or holds(clause, before, *rest)

        # Execute
        transitions = x.transitions(("q", (FrozenMap({1: 0}),), ()))

        # Assert
        self.assertEqual((["notin", "notin", "in"], 4, (24, 20)),
                         (asked, len(transitions), (x.stats[0].tried, x.stats[0].pruned)))

    # c?r: some entry of m has id r.k and x r.a.
    ENTRY = diagram({"q": ["m"]}, "q", init_post=[eq("m'", "m")], edges=[
        edge("q", "q", "c?r", guard=[exists("K", builtin("in", bound(0), "m"), eq(fld("r", "k"), fld(idx("m", bound(0)), "id")),
                                            eq(fld("r", "a"), fld(idx("m", bound(0)), "x")))], post=[eq("m'", "m")])],
        channels={"c": ["R"]})
    ENTRY_TYPES = {"K": [1, 2, 3], "R": record(k=list(range(20)), a=list(range(5)), b=[0, 1])}

    def test_the_parts_an_entry_of_a_map_decides_are_drawn_from_its_entries(self):
        # Setup: two entries, so the ∃ is asked once of each of their two
        # values of (r.k, r.a), and each goes with both values of r.b.
        x = fixture.explorer(self.ENTRY, Checks(types=self.ENTRY_TYPES))
        asked = []
        holds = x.ev.holds
        x.ev.holds = lambda clause, before, *rest: asked.extend(c for c in clause if hasattr(c, "body")) or holds(clause, before, *rest)
        m = FrozenMap({1: FrozenMap({"id": 4, "x": 3}), 2: FrozenMap({"id": 7, "x": 1})})

        # Execute
        transitions = x.transitions(("q", (m,), ()))

        # Assert
        self.assertEqual((2, 4), (len(asked), len(transitions)))

    def test_every_part_an_entry_of_a_map_decides_is_drawn_from_its_entries(self):
        # Setup: R has no part the ∃ leaves, so it is asked of the two entries only.
        x = fixture.explorer(self.ENTRY, Checks(types=dict(self.ENTRY_TYPES, R=record(k=list(range(20)), a=list(range(5))))))
        asked = []
        holds = x.ev.holds
        x.ev.holds = lambda clause, before, *rest: asked.extend(c for c in clause if hasattr(c, "body")) or holds(clause, before, *rest)
        m = FrozenMap({1: FrozenMap({"id": 4, "x": 3}), 2: FrozenMap({"id": 7, "x": 1})})

        # Execute
        transitions = x.transitions(("q", (m,), ()))

        # Assert
        self.assertEqual((2, 2), (len(asked), len(transitions)))

    def test_a_conjunct_reading_a_record_whole_waits_for_every_part(self):
        tests = {
            "the record a hole of an atom": atom("${}['k'] in ${}", "r", "m"),
            "the record a term": builtin("ne", "r", "m"),
        }
        for name, conjunct in tests.items():
            with self.subTest(name):
                # Setup
                d = dict(self.PART, edges=[dict(self.PART["edges"][0], guard=[builtin("notin", "id", "m"), conjunct])])
                x = fixture.explorer(d, Checks(types=self.PART_TYPES), stats=True)

                # Execute
                x.transitions(("q", (FrozenMap(),), ()))

                # Assert
                self.assertEqual((24, 0), (x.stats[0].tried, x.stats[0].pruned))

    def test_the_leading_conjuncts_are_evaluated_once_for_the_fields_they_read(self):
        # Setup
        x = fixture.explorer(self.FAMILY, Checks(types=self.FAMILY_TYPES))
        calls = []
        holds = x.ev.holds
        x.ev.holds = lambda clause, before, *rest: calls.append(len(clause)) or holds(clause, before, *rest)

        # Execute
        x.transitions(("q", (FrozenMap(),), ()))

        # Assert
        self.assertEqual([1, 1], calls)

    def test_a_term_an_equality_determines_is_not_computed_below_values_the_leading_conjuncts_reject(self):
        # Setup: r is t[op].x, which t["y"], no record, does not have; the
        # first op reaches a complete value, the guard rejects the id there,
        # and t["y"].x is never read.
        d = diagram({"q": ["m", "t"]}, "q", init_post=[builtin("empty", "m'"), eq("t'", "t")], edges=[
            edge("q", "q", "c?id?op?r", guard=[builtin("at", idx("m", "id"), state="s"), eq("r", fld(idx("t", "op"), "x"))],
                 post=[builtin("frame", "m'", "m"), eq("t'", "t")])],
            families=[family("m", "none", {"none": [], "s": ["v"]}, key="id")], channels={"c": ["id", "OP", "R"]})
        x = fixture.explorer(d, Checks(types={"id": [1, 2], "OP": ["x", "y"], "v": [0]}))

        # Execute
        transitions = x.transitions(("q", (FrozenMap(), FrozenMap({"x": FrozenMap({"x": 0}), "y": 5})), ()))

        # Assert
        self.assertEqual([], transitions)

    def test_nothing_is_evaluated_when_a_later_field_has_no_value(self):
        tests = {
            "a conjunct": [atom("$k は f"), atom("$r == 0")],
            "a conjunct before an equality": [atom("$k は f"), eq("v", "k"), atom("$r == 0")],
        }
        for name, guard in tests.items():
            with self.subTest(name):
                # Setup
                d = diagram({"q": ["n"]}, "q", init_post=[eq("n'", lit(0))], edges=[
                    edge("q", "q", "c?k?v?r", guard=guard, post=[eq("n'", "n")])],
                    channels={"c": ["K", "V", "R"]})
                x = fixture.explorer(d, Checks(types={"K": [1, 2], "V": [1, 2], "R": []}),
                                     environment=environment.over({"$_0 は f": True}, {}))

                # Execute
                transitions = x.transitions(("q", (0,), ()))

                # Assert
                self.assertEqual(([], {}), (transitions, x.meaning.decided))

    # c?id?v: v is m[id].y, which only an instance at b has.
    DETERMINED = diagram({"q": ["m"]}, "q", init_post=[builtin("empty", "m'")], edges=[
        edge("q", "q", "c?id?v", guard=[builtin("at", idx("m", "id"), state="b"), eq("v", fld(idx("m", "id"), "y"))],
             post=[builtin("frame", "m'", "m")])],
        families=[family("m", "none", {"none": [], "a": ["x"], "b": ["x", "y"]}, key="id")], channels={"c": ["id", "V"]})
    DETERMINED_TYPES = {"id": [1], "V": [0, 1]}

    def test_a_term_an_equality_determines_is_computed_only_where_the_conjuncts_before_it_hold(self):
        tests = {
            "the instance elsewhere, so the conjunct before is false": (inst("a", x=0), []),
            "the instance there": (inst("b", x=0, y=7), ["c(id=1, v=7)"]),
        }
        for name, (instance, labels) in tests.items():
            with self.subTest(name):
                # Setup
                x = fixture.explorer(self.DETERMINED, Checks(types=self.DETERMINED_TYPES))

                # Execute
                got = [label for _, label, _, _ in x.successors(("q", (FrozenMap({1: instance}),), ()))]

                # Assert
                self.assertEqual(labels, got)

    def test_a_term_that_fails_where_the_guard_reaches_it_is_an_error(self):
        # Setup: the instance is at b without y.
        x = fixture.explorer(self.DETERMINED, Checks(types=self.DETERMINED_TYPES))

        # Execute / Assert
        with self.assertRaisesRegex(ValueError, "has no field y"):
            x.transitions(("q", (FrozenMap({1: inst("b", x=0)}),), ()))

    def test_why_stuck_names_the_conjunct_before_the_equality_that_is_false(self):
        # Setup
        x = fixture.explorer(self.DETERMINED, Checks(types=self.DETERMINED_TYPES))

        # Execute
        why = x.why_stuck(("q", (FrozenMap({1: inst("a", x=0)}),), ()))

        # Assert
        self.assertEqual([("fields", ["id"], 1)], [(w["reason"], w["fields"], w["tried"]) for w in why])

    def test_why_stuck_counts_every_value_of_the_full_product(self):
        # Setup
        x = fixture.explorer(self.FAMILY, Checks(types=self.FAMILY_TYPES))

        # Execute
        why = x.why_stuck(("q", (FrozenMap(),), ()))

        # Assert
        self.assertEqual([24], [w["tried"] for w in why])


def some_entry():
    """∃ an entry of m whose id is p.k and whose x is p.j."""
    return exists("K", builtin("in", bound(0), "m"), eq(fld("p", "k"), fld(idx("m", bound(0)), "id")),
                  eq(fld("p", "j"), fld(idx("m", bound(0)), "x")))


def entries(*pairs):
    """A map from 1 on of entries with the ids and xs given."""
    return FrozenMap({n: FrozenMap({"id": i, "x": x}) for n, (i, x) in enumerate(pairs, 1)})


def refuse_three(j):
    """An environment function that fails at 3."""
    if j == 3:
        raise ValueError("e refuses 3")
    return True


def naive(x, state, e):
    """The guard of e evaluated in full, as the language says, apart from
    the walk: every value of the product of the fields' carriers, in the
    event's order, each conjunct left to right up to the first false. The
    labels it holds of, or the error it raises."""
    labels = []
    try:
        for combo in itertools.product(*(x._field_values(e, p) for p in e.params)):
            before = x._before(state, dict(zip(e.params, combo)))
            if all(x.ev.holds((c,), before) for c in e.guard):
                labels.append(x._label(e, before))
    except ValueError as err:
        return str(err)
    return labels


def pruned(x, state):
    """The labels of the steps the walk takes from state, or the error it raises."""
    try:
        return [label for _, label, _, _ in x.successors(state)]
    except ValueError as err:
        return str(err)


class OracleTest(unittest.TestCase):
    """The walk against the guard evaluated over the full product, the
    values equalities determine lying in their carriers."""

    def test_the_steps_errors_and_decided_functions_are_those_of_the_full_product(self):
        # e fails at 3, f holds of k = 2 only, g of j = 1 only; used: the functions a
        # case's guard applies.
        fns = {"$_0 は e": refuse_three, "$_0 は f": lambda k: k == 2, "$_0 は g": lambda j: j == 1, "$_0 は h": True, "$_0 $_1 は i": True}
        family_m = [family("m", "none", {"none": [], "a": ["x"], "b": ["x", "y"]}, key="k")]
        at_b, term_y = builtin("at", idx("m", "k"), state="b"), eq("v", fld(idx("m", "k"), "y"))
        tests = {
            "conjuncts at different depths": (
                "c?k?j", ["K", "J"], [atom("$k は f"), atom("$j は g"), atom("$k は h"), atom("$k $j は i")], [], (0,), "fghi"),
            "a binding whose term reads another bound field": (
                "c?k?v?j", ["K", "V", "J"], [atom("$k は f"), eq("v", "k"), eq("j", "v"), atom("$v $j は i")], [], (0,), "fi"),
            "a binding after conjuncts at a deeper depth": (
                "c?k?v?j", ["K", "V", "J"], [atom("$j は g"), atom("$k は f"), eq("v", "k"), atom("$v は h")], [], (0,), "fgh"),
            "an empty carrier after a binding": (
                "c?k?v?r", ["K", "V", "R"], [atom("$k は f"), eq("v", "k"), atom("$r == 0")], [], (0,), "f"),
            "the #157 shape, the conjunct before false": (
                "c?k?v", ["K", "V"], [at_b, term_y], family_m, (FrozenMap({1: inst("a", x=0)}),), ""),
            "the #157 shape, the conjunct before true": (
                "c?k?v", ["K", "V"], [at_b, term_y], family_m, (FrozenMap({1: inst("b", x=0, y=2)}),), ""),
            "an error after a false conjunct": (
                "c?k?v", ["K", "V"], [at_b, term_y], family_m, (FrozenMap({1: inst("a", x=0), 2: inst("b", x=0)}),), ""),
            "a conjunct reading one part of a record": (
                "c?j?p", ["J", "P"], [atom("$j は g"), atom("${} は f", fld("p", "k")), atom("${} は h", fld("p", "j"))], [], (0,), "fgh"),
            "a conjunct reading the whole record after one reading a part": (
                "c?p?j", ["P", "J"], [atom("${} は f", fld("p", "k")), atom("$p は h"), atom("$j は g")], [], (0,), "fgh"),
            "a part read as an entry": (
                "c?j?p", ["J", "P"], [atom("$j は g"), atom("${} は f", idx("p", lit("k"))), atom("${} は h", fld("p", "j"))], [], (0,), "fgh"),
            "a part read by a key that is not a literal": (
                "c?j?p", ["J", "P"], [atom("${} は f", fld("p", "k")), atom("${} は g", idx("p", "j"))], [], (0,), "fg"),
            "a builtin reading the whole record after a part": (
                "c?p?j", ["P", "J"], [atom("${} は f", fld("p", "k")), eq("p", lit({"k": 2, "j": 1})), atom("$j は g")], [], (0,), "fg"),
            "a record with a determined part and one whose parts range": (
                "c?q?p", ["P", "P"], [eq(idx("q", lit("k")), lit(2)), atom("${} は f", fld("p", "k")), atom("${} は g", fld("q", "j")),
                                      atom("${} は h", fld("p", "j"))], [], (0,), "fgh"),
            "an equality from a part": (
                "c?p?v", ["P", "V"], [atom("${} は f", fld("p", "k")), eq("v", fld("p", "j")), atom("$v は g")], [], (0,), "fg"),
            "a membership drawing a field": (
                "c?k?j", ["K", "J"], [builtin("in", "k", "m"), atom("$k は f"), atom("$j は g")], [], (FrozenMap({2: 0, 3: 0}),), "fg"),
            "a membership drawing a part": (
                "c?j?p", ["J", "P"], [atom("$j は g"), builtin("in", fld("p", "k"), "m"), atom("${} は h", fld("p", "j"))], [],
                (FrozenMap({1: 0}),), "gh"),
            "a membership after a conjunct reading a later field": (
                "c?k?j", ["K", "J"], [atom("$j は g"), builtin("in", "k", "m"), atom("$k は f")], [], (FrozenMap({2: 0}),), "fg"),
            "a membership after a conjunct reading a later field, before one reading both": (
                "c?k?j", ["K", "J"], [atom("$j は e"), builtin("in", "k", "m"), atom("$k $j は i")], [], (FrozenMap({2: 0}),), "ei"),
            "a membership of an empty map after a conjunct over an earlier field": (
                "c?j?k", ["J", "K"], [atom("$j は g"), builtin("in", "k", "m"), atom("$k は f")], [], (FrozenMap(),), "fg"),
            "a membership of no map": (
                "c?k", ["K"], [builtin("in", "k", "m")], [], (5,), ""),
            "parts drawn from entries": (
                "c?p?j", ["P", "J"], [some_entry(), atom("${} は h", fld("p", "j")), atom("$j は g")], [],
                (entries((1, 2), (3, 1), (3, 2)),), "gh"),
            "parts drawn from entries, one outside the type": (
                "c?p", ["P"], [some_entry(), atom("${} は h", fld("p", "k"))], [], (entries((9, 1), (2, 2)),), "h"),
            "no entry, after a conjunct over an earlier field": (
                "c?j?p", ["J", "P"], [atom("$j は g"), some_entry(), atom("${} は h", fld("p", "k"))], [], (FrozenMap(),), "gh"),
            "an entry without the field, after a false conjunct": (
                "c?j?p", ["J", "P"], [atom("$j は f"), some_entry()], [], (FrozenMap({1: FrozenMap({"id": 1})}),), "f"),
            "an entry without the field": (
                "c?p", ["P"], [some_entry()], [], (FrozenMap({1: FrozenMap({"id": 2})}),), ""),
            "entries after a conjunct reading a later field, before one reading both": (
                "c?p?j", ["P", "J"], [atom("$j は e"), some_entry(), atom("${} $j は i", fld("p", "k"))], [],
                (entries((2, 1)),), "ei"),
            "entries of no map": (
                "c?p", ["P"], [some_entry()], [], (5,), ""),
            "an error reading a part of a part after a false conjunct": (
                "c?p", ["P"], [atom("${} は f", fld("p", "k")), atom("${} は g", fld(fld("p", "j"), "z"))], [], (0,), "fg"),
        }
        types = {"K": [1, 2, 3], "V": [1, 2, 3], "J": [1, 2, 3], "R": [], "P": record(k=[1, 2, 3], j=[1, 2]), "x": [0], "y": [0]}
        for name, (event, channel, guard, families, values, used) in tests.items():
            with self.subTest(name):
                # Setup
                d = diagram({"q": ["m"]}, "q", init_post=[eq("m'", "m")], edges=[
                    edge("q", "q", event, guard=guard, post=[eq("m'", "m")])], families=families, channels={"c": channel})

                def explorer():
                    return fixture.explorer(d, Checks(types=types), environment=environment.over({k: f for k, f in fns.items() if k[-1] in used}, {}))
                state = ("q", values, ())
                full, x = explorer(), explorer()
                want = naive(full, state, full.d.edges[0])

                # Execute
                got = pruned(x, state)

                # Assert
                self.assertEqual(want, got)
                self.assertEqual(list(full.meaning.decided), list(x.meaning.decided))


class EnvironmentTest(unittest.TestCase):
    """An atom the checks module declares is a function of the environment;
    every other atom is the glue's."""

    FROZEN = diagram({"a": ["n"]}, "a", init_post=[atom("$n' == 0")], edges=[
        edge("a", "a", "inc", guard=[atom("$n が停止されていない"), atom("$n < 2")], post=[atom("$n' == $n + 1")])])

    def test_the_environment_decides_an_atom_it_declares(self):
        tests = {"given true": (True, 3), "given false": (False, 1), "given as a function": (lambda n: n < 1, 2)}
        for name, (given, states) in tests.items():
            with self.subTest(name):
                # Setup
                env = environment.over({"$_0 が停止されていない": given}, {})

                # Execute
                result = fixture.explorer(self.FROZEN, environment=env).run()

                # Assert
                self.assertEqual(states, len(result.states))
                self.assertEqual(["$_0 が停止されていない"], result.environment["decided"])

    def test_an_atom_the_environment_does_not_declare_is_the_glue_s(self):
        with self.assertRaisesRegex(Exception, "が停止されていない"):
            fixture.explorer(self.FROZEN).run()

    # FROZEN guarded further by a term applying the environment function next.
    FROZEN_NEXT = diagram({"a": ["n"]}, "a", init_post=[atom("$n' == 0")], edges=[
        edge("a", "a", "inc", guard=[atom("$n が停止されていない"), atom("$n < 2"),
                                     builtin("ne", {"kind": "app", "fn": "next", "args": [fixture.term("n")]}, "n")],
             post=[atom("$n' == $n + 1")])])

    @staticmethod
    def frozen_applying(fn, *args):
        """FROZEN guarded further by a term applying fn to args."""
        return diagram({"a": ["n"]}, "a", init_post=[atom("$n' == 0")], edges=[
            edge("a", "a", "inc", guard=[atom("$n が停止されていない"), atom("$n < 2"),
                                         builtin("ne", {"kind": "app", "fn": "next", "args": [fixture.term("n")]}, "n"),
                                         builtin("ne", {"kind": "app", "fn": fn, "args": [fixture.term(a) for a in args]}, "n")],
                 post=[atom("$n' == $n + 1")])])

    def test_a_function_named_with_two_arities_is_refused_before_the_walk(self):
        tests = {
            "a function applied with two arities": (
                self.frozen_applying("next", "n", "n"), "^the function 'next' is applied with 1 and 2 arguments; one name has one arity"),
            "an applied function named like an atom function of other arguments": (
                self.frozen_applying("$_0 が停止されていない", "n", "n"),
                "^the function '\\$_0 が停止されていない' is applied with 1 and 2 arguments; one name has one arity"),
        }
        for name, (d, said) in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(ValueError, said):
                    fixture.explorer(d, environment=environment.over({"$_0 が停止されていない": True, "next": lambda *a: 0}, {}))

    def test_what_the_environment_cannot_decide_is_refused_before_the_walk(self):
        tests = {
            "a declared function nothing gives": (self.FROZEN, {"$_0 が停止されていない": None}, "nothing decides the environment's functions '\\$_0 が停止されていない'"),
            "a declared name of no atom": (self.FROZEN, {"$_0 が停止されていない": True, "$_0 が赤い": True}, "names no function of the diagram"),
            "a function of other arguments": (self.FROZEN, {"$_0 が停止されていない": lambda a, b: True}, "has 1 argument"),
            "an applied function nothing gives, listed with a declared one": (
                self.FROZEN_NEXT, {"$_0 が停止されていない": None},
                "^nothing decides the environment's functions '\\$_0 が停止されていない', 'next'; "),
            "an applied function given a truth value": (
                self.FROZEN_NEXT, {"$_0 が停止されていない": True, "next": True}, "^the environment function 'next' is applied in a term, so it needs a function"),
        }
        for name, (d, module, said) in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(ValueError, said):
                    fixture.explorer(d, environment=environment.over(module, {}))

    def test_an_applied_function_is_given_in_the_environment(self):
        # Setup
        env = environment.over({"$_0 が停止されていない": True, "next": lambda n: n + 1}, {})

        # Execute
        result = fixture.explorer(self.FROZEN_NEXT, environment=env).run()

        # Assert
        self.assertEqual(3, len(result.states))


class PropertyTest(unittest.TestCase):
    def test_a_broken_invariant_names_the_state_and_the_trace(self):
        # Setup
        checks = tally_checks(invariants={"在庫は 4 未満": lambda 在庫: 在庫 < 4})

        # Execute
        result = fixture.explorer(TALLY, checks, max_states=200).run()

        # Assert
        name, state = result.violations[0]
        self.assertEqual(("在庫は 4 未満", ("a", (4,), ())), (name, state))
        self.assertEqual(["搬入(量=3)", "搬入(量=3)"], result.trace_to(state))

    def test_ghosts_follow_the_events_and_invariants_may_read_them(self):
        # Setup
        checks = tally_checks(
            ghosts={"総搬入": (0, {"搬入": lambda 総搬入, 量: 総搬入 + 量}), "総搬出": (0, {"搬出": lambda 総搬出, 量: 総搬出 + 量}),
                    "総損耗": (0, {"搬入": lambda 総損耗: 総損耗 + 1, "搬出": lambda 総損耗: 総損耗 + 1})},
            invariants={"在庫は総和に一致": lambda 在庫, 総搬入, 総搬出, 総損耗: 在庫 == 総搬入 - 総搬出 - 総損耗})

        # Execute
        result = fixture.explorer(TALLY, checks, max_states=300).run()

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
        result = fixture.explorer(TALLY, tally_checks(invariants={"在庫は 1 未満": lambda 在庫: 在庫 < 1}), first=True).run()

        # Assert
        self.assertTrue(result.stopped_early)
        self.assertEqual(("在庫は 1 未満", ("a", (1,), ())), result.violations[0])
        self.assertEqual(3, len(result.states))


# MEMBERS without DROP, so an ID once opened is never given back; and a STOP
# out of the map, after which nothing happens whatever the IDs.
OPENING = fixture.redeclared(dict(MEMBERS, states=["r", "s"], vars=fixture.typed_vars({"r": ["m"], "s": []}), edges=MEMBERS["edges"][:2] + [
    edge("r", "s", "STOP", guard=[builtin("at", idx("m", "k"), state="closed")]) | {"fields": [{"dir": "?", "name": "k"}]},
]))


class SampleTest(unittest.TestCase):
    """A type given as a sample has more values than the walk ranges over: a
    state with no way out that a value beyond the sample gives one is where
    the bound was reached, with that step as the witness, not a deadlock."""

    def test_a_dead_end_is_told_from_a_deadlock_by_the_values_beyond_the_sample(self):
        closed = ("r", (FrozenMap({"a": inst("closed", cnt=0)}),), ())
        ordered = [atom("$k >= 'a'")]
        tests = {
            "the bound reached when every ID is used up": (["r"], [], sample(["a"], beyond=["b"]), [], [(closed, "OPEN(k=b)")]),
            "the first of the values beyond that enables a step": (["r"], [atom("$k != 'b'")], sample(["a"], beyond=["b", "c"]), [],
                                                                    [(closed, "OPEN(k=c)")]),
            "a deadlock however many IDs there are": (["r", "s"], [], sample(["a"], beyond=["b"]), [("s", (), ())], []),
            "a deadlock when no value beyond enables a step": (["r"], ordered, sample(["a"], beyond=["0"]), [closed], []),
            "a glue atom ordering the values is walked": (["r"], ordered, sample(["a"], beyond=["b"]), [], [(closed, "OPEN(k=b)")]),
            "a deadlock when the type is not a sample": (["r"], [], ["a"], [closed], []),
        }
        for name, (states, guard, keys, deadlocks, reached) in tests.items():
            with self.subTest(name):
                # Setup
                e = OPENING["edges"][0]
                edges = [dict(e, guard=e["guard"] + guard)] + [e for e in OPENING["edges"][1:] if e["dst"] in states]
                checks = Checks(types={"k": keys})

                # Execute
                result = fixture.explorer(dict(OPENING, edges=edges), checks).run()

                # Assert
                self.assertEqual((deadlocks, reached), (result.deadlocks, result.bound_reached))

    def test_an_end_edge_the_values_beyond_enable_is_the_witness(self):
        # Setup
        # The end edge is open only for a key b not in m, which no value of
        # the sample is.
        d = dict(OPENING, edges=OPENING["edges"][:2], ends=[end("r", guard=[exists("k", builtin("notin", bound(0), "m"), atom("${} == 'b'", bound(0)))])])
        checks = Checks(types={"k": sample(["a"], beyond=["b"])})

        # Execute
        result = fixture.explorer(d, checks).run()

        # Assert
        self.assertEqual(([], ["the end edge"]), (result.deadlocks, [w for _, w in result.bound_reached]))

    def test_a_type_built_of_a_sample_takes_the_values_beyond_it(self):
        # OPEN receives a record whose id is the key, not the key itself.
        rid = fld("r", "id")
        opens = edge("r", "r", "OPEN?r", guard=[builtin("notin", rid, "m")],
                     post=[builtin("frame", "m'", "m", rid), builtin("at", idx("m'", rid), state="open"),
                           atom("${} == 0", fld(idx("m'", rid), "cnt"))])
        ids = sample(["a"], beyond=["b"])
        opened = ("r", (FrozenMap({"a": inst("open", cnt=0)}),), ())
        tests = {
            "a record of the sample": (ids, [], [(opened, "OPEN(r={'id': 'b'})")]),
            "a record of values listed by hand": (["a"], [opened], []),
        }
        for name, (keys, deadlocks, reached) in tests.items():
            with self.subTest(name):
                # Setup
                types = {"k": ids, "r": record(id=keys), "cnt": [0]}
                checks = Checks(types=types)

                # Execute
                result = fixture.explorer(fixture.redeclared(dict(OPENING, edges=[opens])), checks).run()

                # Assert
                self.assertEqual((deadlocks, reached), (result.deadlocks, result.bound_reached))

    def test_the_values_beyond_the_sample_add_no_state(self):
        used_up = ("r", (FrozenMap({"a": inst("closed", cnt=0), "b": inst("closed", cnt=0)}),), ())
        tests = {"a sample": (sample(["a", "b"], beyond=["c"]), [], [(used_up, "OPEN(k=c)")]),
                 "the same values, not a sample": (["a", "b"], [used_up], [])}
        for name, (keys, deadlocks, reached) in tests.items():
            with self.subTest(name):
                # Setup
                d = dict(OPENING, edges=OPENING["edges"][:2])

                # Execute
                result = fixture.explorer(d, Checks(types={"k": keys})).run()

                # Assert
                self.assertEqual((9, deadlocks, reached), (len(result.states), result.deadlocks, result.bound_reached))


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
        result = fixture.explorer(MEMBERS, Checks(types={"k": ["a"]})).run()

        # Assert
        self.assertEqual([{"a": {"at": "open", "fields": {"cnt": 0}}}], result.replay_to(result.states[1])["steps"][0]["values"])


class CandidatesTest(unittest.TestCase):
    """What a REPL asks at a value prompt."""

    def test_initial_candidates_are_the_start_post_s_valuations(self):
        self.assertEqual([[2]], fixture.explorer(COUNTER).initial_candidates())

    def test_candidates_of_an_edge_come_per_value_of_the_fields(self):
        # Execute
        got = fixture.explorer(TALLY, tally_checks()).candidates("a", [3], 1)

        # Assert
        self.assertEqual([{"params": {"量": 1}, "values": [1]}, {"params": {"量": 2}, "values": [0]}], got)

    def test_a_bad_edge_index_is_refused(self):
        with self.assertRaisesRegex(IndexError, "no edge 5"):
            fixture.explorer(COUNTER).candidates("a", [1], 5)


if __name__ == "__main__":
    unittest.main()


class DeclaredTypesTest(unittest.TestCase):
    def test_a_variable_and_a_field_range_over_their_declared_types_not_their_names(self):
        # Setup: n and x are named apart from their type, Bit.
        d = diagram({"a": ["n"]}, "a", init_post=[eq("n'", lit(0))], edges=[edge("a", "a", "set?x", post=[eq("n'", "x")])],
                    types={"n": "Bit"}, channels={"set": ["Bit"]})

        # Execute
        result = fixture.explorer(d, Checks(types={"Bit": [0, 1]})).run()

        # Assert
        self.assertEqual({("a", (0,), ()), ("a", (1,), ())}, set(result.states))

    def test_a_value_of_any_a_clause_gives_is_walked(self):
        # Setup
        d = diagram({"a": ["memo"]}, "a", init_post=[eq("memo'", lit("x"))], edges=[edge("a", "a", "go", post=[eq("memo'", "memo")])],
                    types={"memo": "any"})

        # Execute
        result = fixture.explorer(d, Checks(types={})).run()

        # Assert
        self.assertEqual([("a", ("x",), ())], list(result.states))

    def test_a_value_of_any_it_would_enumerate_is_refused(self):
        tests = {
            "a variable the post leaves free": (diagram({"a": ["memo"]}, "a", types={"memo": "any"}), "the type any constrains nothing"),
            "a field received": (diagram({"a": []}, "a", edges=[edge("a", "a", "put?x")], channels={"put": ["any"]}), "the type any constrains nothing"),
            "an ∃": (diagram({"a": []}, "a", edges=[edge("a", "a", "go", guard=[exists("any", eq(bound(0), lit(1)))])]),
                     "an ∃ ranges over the type any"),
        }
        for name, (d, said) in tests.items():
            with self.subTest(name):
                # Setup
                checks = Checks(types={})

                # Execute, Assert
                with self.assertRaisesRegex(PostError, said):
                    fixture.explorer(d, checks).run()

    def test_a_literal_compared_with_a_value_of_any_is_not_refused(self):
        # Setup
        d = diagram({"a": ["memo"]}, "a", init_post=[eq("memo'", lit(1.5))], types={"memo": "any"})

        # Execute
        result = fixture.explorer(d, Checks(types={})).run()

        # Assert
        self.assertEqual([("a", (1.5,), ())], list(result.states))
