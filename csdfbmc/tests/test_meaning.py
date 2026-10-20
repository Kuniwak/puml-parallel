"""What a clause reads, as the reductions of the walk take it."""
import unittest

from csdfbmc.meaning import TwinsMeaning
from csdfbmc.model import Clause, Diagram
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


if __name__ == "__main__":
    unittest.main()
