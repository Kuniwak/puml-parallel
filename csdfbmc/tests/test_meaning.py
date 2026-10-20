"""What a clause reads, as the reductions of the walk take it."""
import unittest

from csdfbmc.meaning import OPAQUE, GlueMeaning, ReservedOpaque, TwinsMeaning
from csdfbmc.model import Clause, Diagram
from csdfbmc.protocol import Analysis
from csdfbmc.tests.test_protocol import Recording
from csdfbmc.twins import Twins

EMPTY = Diagram.from_json({"states": {"a": {"name": "a", "vars": [], "line": 1}},
                           "start_edge": {"dst": "a", "line": 1}, "edges": [], "end_edge": None})


class TwinsMeaningTest(unittest.TestCase):
    def test_a_formal_expression_reads_the_names_it_does_not_bind(self):
        meaning = TwinsMeaning(EMPTY, Twins(domains={}, clauses={}))
        clause = Clause("constrain", "どれも正", formal="all(v > 0 for v in xs) and (lambda w: w)(y)")
        names = meaning.names(clause, "guard")
        self.assertLessEqual({"xs", "y"}, names)
        self.assertFalse({"v", "w"} & names)


class GlueMeaningTest(unittest.TestCase):
    def setUp(self):
        self.client = Recording({"n > 0": Analysis(opaque=False, reads=("n",)), "OPAQUE": Analysis(opaque=True)},
                                lambda b: b["n"] > 0)
        self.meaning = GlueMeaning(self.client)

    def test_a_clause_is_evaluated_with_only_the_names_it_reads(self):
        self.assertEqual(True, self.meaning.evaluate(Clause("local", "n > 0"), "guard", {"n": 1, "m": 9}))
        self.assertEqual([[{"n": 1}]], self.client.asked)

    def test_an_opaque_clause_has_no_value_and_reads_nothing_known(self):
        clause = Clause("local", "OPAQUE")
        self.assertIs(OPAQUE, self.meaning.evaluate(clause, "guard", {}))
        self.assertIsNone(self.meaning.names(clause, "guard"))

    def test_a_name_the_step_does_not_bind_is_refused(self):
        with self.assertRaises(KeyError):
            self.meaning.evaluate(Clause("local", "n > 0"), "guard", {})

    def test_prefetch_asks_about_every_env_in_one_request(self):
        self.meaning.prefetch(Clause("local", "n > 0"), "guard", [{"n": 1}, {"n": 2}])
        self.assertEqual([[{"n": 1}, {"n": 2}]], self.client.asked)


OPAQUE_CLAUSE = Clause("constrain", "opaque")
FORMAL_CLAUSE = Clause("constrain", "n > 0")


class ReservedOpaqueTest(unittest.TestCase):
    """The reserved clause opaque has no meaning to ask anyone about; any
    other clause is the inner meaning's."""

    def test_each_question_about_a_clause(self):
        for name, clause, ask, want, want_asked in [
            ("opaque has no value", OPAQUE_CLAUSE, lambda m, c: m.evaluate(c, "guard", {"n": 1}), OPAQUE, []),
            ("opaque reads nothing known", OPAQUE_CLAUSE, lambda m, c: m.names(c, "guard"), None, []),
            ("opaque reads nothing beyond", OPAQUE_CLAUSE, lambda m, c: m.names_beyond(c, None), None, []),
            ("opaque is not prefetched", OPAQUE_CLAUSE, lambda m, c: m.prefetch(c, "guard", [{"n": 1}]), None, []),
            ("another clause is evaluated by the inner meaning", FORMAL_CLAUSE, lambda m, c: m.evaluate(c, "guard", {"n": 1}), True, [[{"n": 1}]]),
            ("another clause reads what the inner meaning says", FORMAL_CLAUSE, lambda m, c: m.names(c, "guard"), {"n"}, []),
            ("another clause is prefetched by the inner meaning", FORMAL_CLAUSE, lambda m, c: m.prefetch(c, "guard", [{"n": 1}]), None, [[{"n": 1}]]),
        ]:
            with self.subTest(name):
                client = Recording({"n > 0": Analysis(opaque=False, reads=("n",))}, lambda b: b["n"] > 0)
                meaning = ReservedOpaque(GlueMeaning(client))
                got = ask(meaning, clause)
                self.assertEqual(want, got)
                self.assertEqual(want_asked, client.asked)

    def test_opaque_is_not_formalised_even_when_the_twins_have_it(self):
        meaning = ReservedOpaque(TwinsMeaning(EMPTY, Twins(domains={}, clauses={"opaque": lambda: True})))
        self.assertIs(OPAQUE, meaning.evaluate(OPAQUE_CLAUSE, "guard", {}))

    def test_the_message_names_the_reserved_clause(self):
        meaning = ReservedOpaque(GlueMeaning(Recording({}, lambda b: True)))
        self.assertIn("'opaque' is not formalised yet", meaning.opaque_message(OPAQUE_CLAUSE))


if __name__ == "__main__":
    unittest.main()
