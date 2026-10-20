"""What a clause reads, as the reductions of the walk take it."""
import unittest

from csdfbmc.meaning import OPAQUE, GlueMeaning, TwinsMeaning
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


if __name__ == "__main__":
    unittest.main()
