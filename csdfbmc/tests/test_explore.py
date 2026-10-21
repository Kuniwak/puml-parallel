"""The explorer walks the product of control states and valuations.

The diagram is the JSON csdfparse prints, its clauses written in the language
of the reference Python glue; the checks give the domains the values of
received fields are drawn from.
"""
import unittest
from csdfbmc.tests import fixture

from csdfbmc.model import Diagram
from csdfbmc.protocol import EvaluatorError
from csdfbmc.checks import Checks
from csdfbmc import environment as environments


COUNTER = {
    "states": {
        "a": {"name": "counting", "vars": [{"name": "n", "type": "Nat"}], "line": 2},
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


class ExplorerTest(unittest.TestCase):
    def test_reaches_every_valuation_the_clauses_admit(self):
        result = fixture.explorer(COUNTER).run()
        self.assertEqual(
            {("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())},
            set(result.states),
        )

    def test_reports_a_state_with_no_way_out_as_a_deadlock(self):
        result = fixture.explorer(COUNTER).run()
        self.assertEqual([("b", (), ())], result.deadlocks)
        self.assertEqual(["dec", "dec", "stop"], result.trace_to(("b", (), ())))

    def test_an_end_edge_is_a_way_out(self):
        diagram = dict(COUNTER, end_edge={"src": "b", "guard": "", "line": 8})
        result = fixture.explorer(diagram).run()
        self.assertEqual([], result.deadlocks)

    def test_finds_a_tau_cycle_among_the_reachable_states(self):
        diagram = dict(COUNTER, edges=COUNTER["edges"] + [
            {"src": "a", "dst": "a", "event": "tau", "channel": "tau", "fields": [], "guard": "n == 0", "post": "[{'n': n}]", "line": 9},
        ])
        result = fixture.explorer(diagram).run()
        self.assertEqual([[("a", (0,), ())]], result.tau_cycles)

    def test_event_parameters_range_over_their_domain(self):
        diagram = dict(COUNTER, edges=[
            {"src": "a", "dst": "a", "event": "add?k", "channel": "add", "fields": [{"dir": "?", "name": "k"}], "guard": "n + k <= 3", "post": "[{'n': n + k}]", "line": 6},
        ])
        result = fixture.explorer(diagram, Checks(types={"k": [1, 2]})).run()
        self.assertEqual({("a", (2,), ()), ("a", (3,), ())}, set(result.states))
        self.assertEqual(["add(k=1)"], result.trace_to(("a", (3,), ())))

    def without_its_guard(self):
        """COUNTER with dec's guard not formalised yet: an atom of its holes."""
        return dict(COUNTER, edges=[dict(COUNTER["edges"][0], guard="$n が 正"), COUNTER["edges"][1]])

    def test_the_tool_word_opaque_is_refused(self):
        # Setup
        diagram = dict(COUNTER, edges=[dict(COUNTER["edges"][0], guard="@opaque"), COUNTER["edges"][1]])

        # Execute, Assert
        with self.assertRaisesRegex(ValueError, "write the clause as an atom"):
            fixture.diagram(diagram)

    def with_an_unread_variable(self):
        """without_its_guard, with a variable x that nothing reads."""
        d = self.without_its_guard()
        a = dict(d["states"]["a"], vars=d["states"]["a"]["vars"] + [{"name": "x"}])
        start = dict(d["start_edge"], post="[{'n': 2, 'x': 0}]")
        dec = dict(d["edges"][0], post="[{'n': n - 1, 'x': x}]")
        return dict(d, states=dict(d["states"], a=a), start_edge=start, edges=[dec, d["edges"][1]])

    def test_an_atom_is_the_function_its_text_names_whatever_its_holes_are_called(self):
        # Setup
        cases = [
            ("given by its name", {"$_ が 正": lambda n: n > 0}),
            ("given as the atom", {"$n が 正": lambda n: n > 0}),
            ("given under another hole name", {"$m が 正": lambda m: m > 0}),
        ]
        for name, given in cases:
            with self.subTest(name):
                env = environments.over({}, given, None)

                # Execute
                result = fixture.explorer(self.without_its_guard(), environment=env, max_states=10).run()

                # Assert
                self.assertEqual({("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())}, set(result.states))
                self.assertEqual(["$_ が 正"], result.to_json()["environment"]["decided"])

    def test_a_seed_reads_only_the_atoms_holes(self):
        # Setup
        def asked(diagram):
            seen, seeded = [], environments.over({}, {}, 7)
            env = environments.Environment(decide=lambda name, args: seen.append((name, tuple(args))) or seeded.decide(name, args),
                                           seed=7, given={})
            fixture.explorer(diagram, environment=env, max_states=10).run()
            return seen

        # Execute
        plain, with_x = asked(self.without_its_guard()), asked(self.with_an_unread_variable())

        # Assert
        self.assertEqual(plain, with_x)
        self.assertEqual({("$_ が 正", (2,))}, set(plain[:1]))

    def with_two_atoms(self):
        """COUNTER with both guards not formalised yet."""
        return dict(COUNTER, edges=[dict(COUNTER["edges"][0], guard="$n が 正"), dict(COUNTER["edges"][1], guard="$n が 零")])

    def test_what_the_environment_cannot_decide_is_refused_before_the_walk(self):
        # Setup
        cases = [
            ("a function nothing decides, each listed", self.with_two_atoms(), environments.NONE,
             "nothing decides the environment's functions '$_ が 正', '$_ が 零'"),
            ("a given name that names no function", self.without_its_guard(), environments.over({}, {"$n が 零": True, "$n が 正": True}, None),
             "names no function of the diagram: '$_ が 零'"),
            ("a post not formalised", dict(COUNTER, edges=[dict(COUNTER["edges"][0], post="$n が 減る"), COUNTER["edges"][1]]),
             environments.over({}, {}, 7), "the post clause '$n が 減る' is not formalised"),
            ("a given function taking other than the atom's holes", self.without_its_guard(),
             environments.over({}, {"$n が 正": lambda n, m: True}, None), "'$_ が 正' has 1 hole, but the function given takes"),
            ("a hole the step does not bind", dict(COUNTER, edges=[dict(COUNTER["edges"][0], guard="$k が 正"), COUNTER["edges"][1]]),
             environments.over({}, {}, 7), "the atom '$k が 正' reads k, which the step does not bind"),
        ]
        for name, diagram, env, message in cases:
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(ValueError, message.replace("$", r"\$")):
                    fixture.explorer(diagram, environment=env)

    def test_a_function_the_environment_leaves_undecided_is_an_error_not_false(self):
        # Setup
        env = environments.Environment(decide=environments.nothing, seed=7, given={})

        # Execute, Assert
        with self.assertRaisesRegex(ValueError, r"decided nothing for '\$_ が 正'"):
            fixture.explorer(self.without_its_guard(), environment=env, max_states=10).run()

    def test_a_given_function_giving_other_than_a_truth_value_is_refused(self):
        # Setup
        env = environments.over({}, {"$n が 正": lambda n: n}, None)

        # Execute, Assert
        with self.assertRaisesRegex(ValueError, "gave 2; it gives True or False"):
            fixture.explorer(self.without_its_guard(), environment=env, max_states=10).run()

    def test_a_seed_decides_every_function(self):
        # Execute
        result = fixture.explorer(self.with_two_atoms(), environment=environments.over({}, {}, 7), max_states=10).run()

        # Assert
        self.assertEqual(["$_ が 正", "$_ が 零"], sorted(result.to_json()["environment"]["decided"]))

    def test_the_environment_decides_a_clause_not_formalised(self):
        # Setup
        cases = {
            "the function that is always false": (False, {("a", (2,), ())}),
            "a function of the names it reads": (lambda n: n > 0, {("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())}),
        }
        for name, (given, want) in cases.items():
            with self.subTest(name):
                environment = environments.over({}, {"$n が 正": given}, None)

                # Execute
                result = fixture.explorer(self.without_its_guard(), environment=environment, max_states=10).run()

                # Assert
                self.assertEqual(want, set(result.states))

    def test_a_seed_walks_the_same_states_again_and_is_reported(self):
        # Setup
        def walk():
            return fixture.explorer(self.without_its_guard(), environment=environments.over({}, {}, 7), max_states=10).run()

        # Execute
        first, again = walk(), walk()

        # Assert
        self.assertEqual(first.states, again.states)
        self.assertEqual({"seed": 7, "given": [], "decided": ["$_ が 正"]}, first.to_json()["environment"])

    def test_the_bound_on_states_stops_the_walk(self):
        result = fixture.explorer(COUNTER, max_states=2).run()
        self.assertEqual(2, len(result.states))
        self.assertTrue(result.truncated)


class PostTest(unittest.TestCase):
    """A post clause is a collection of partial assignments to the variables
    after the step; the successors of an edge are their natural join."""

    def diagram(self, post, init="[{'x': 0, 'y': 0}]"):
        return fixture.diagram({
            "states": {"a": {"name": "a", "vars": [{"name": "x"}, {"name": "y"}], "line": 2}},
            "start_edge": {"dst": "a", "post": init, "line": 3},
            "edges": [{"src": "a", "dst": "a", "event": "step", "channel": "step", "fields": [], "guard": "", "post": post, "line": 4}],
            "end_edge": None, "assumptions": [],
        })

    def test_the_successors_are_the_join_of_the_clauses(self):
        result = fixture.explorer(self.diagram("[{'x': x + 1}] ∧ [{'y': y}, {'y': y + 1}]"), max_states=6).run()
        self.assertIn(("a", (1, 0), ()), result.states)
        self.assertIn(("a", (1, 1), ()), result.states)

    def test_two_clauses_constraining_one_variable_must_agree(self):
        result = fixture.explorer(self.diagram("[{'x': 1}, {'x': 2}] ∧ [{'x': 2}, {'x': 3}] ∧ [{'y': y}]"), max_states=6).run()
        self.assertEqual({("a", (0, 0), ()), ("a", (2, 0), ())}, set(result.states))

    def test_a_variable_no_clause_constrains_is_a_finding(self):
        result = fixture.explorer(self.diagram("[{'x': x + 1}]"), max_states=6).run()
        self.assertEqual([(("a", (0, 0), ()), "step", ("y",))], result.unconstrained)
        self.assertEqual([("a", (0, 0), ())], result.states)

    def test_a_post_that_returns_no_successor_is_reported(self):
        result = fixture.explorer(self.diagram("[]")).run()
        self.assertEqual([(("a", (0, 0), ()), "step")], result.empty_posts)
        self.assertEqual([("a", (0, 0), ())], result.deadlocks)

    def test_successor_values_are_frozen(self):
        result = fixture.explorer(self.diagram("[{'x': x, 'y': y}]", init="[{'x': {'k': [1]}, 'y': 0}]")).run()
        x = result.states[0][1][0]
        self.assertEqual({"k": (1,)}, dict(x))
        hash(x)

    def test_a_post_must_be_assignments(self):
        with self.assertRaisesRegex(EvaluatorError, "list or set of assignments"):
            fixture.explorer(self.diagram("x == y")).run()


class EventFieldsTest(unittest.TestCase):
    def test_an_edge_without_its_channel_and_fields_is_refused(self):
        diagram = {"states": {"a": {"name": "a", "vars": []}}, "start_edge": {"dst": "a", "post": ""},
                   "edges": [{"src": "a", "dst": "a", "event": "c?x", "guard": "", "post": ""}],
                   "end_edge": None, "assumptions": []}
        with self.assertRaisesRegex(ValueError, "no channel and fields"):
            Diagram.from_json(diagram)


if __name__ == "__main__":
    unittest.main()
