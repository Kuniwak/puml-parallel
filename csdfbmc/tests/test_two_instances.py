"""A sync that moves two instances of one family: the distinct guard, the
moves chained on one map, and partial-order reduction left out of it."""
import json
import unittest
from pathlib import Path

from csdfbmc.model import Clause, Diagram
from csdfbmc.reduce import moves_one_instance

MOVE = Path(__file__).resolve().parents[2] / "examples" / "bmc" / "move" / "move.json"


def move_edge():
    d = Diagram.from_json(json.loads(MOVE.read_text()))
    return next(e for e in d.edges if e.name == "MOVE")


class TwoInstancesTest(unittest.TestCase):
    def test_the_distinct_clause_names_both_ids(self):
        distinct = [c for c in move_edge().guard if c.kind == "distinct"]
        self.assertEqual([("from", "to")], [c.ids for c in distinct])

    def test_each_local_clause_says_which_instance_it_is_about(self):
        ids = [c.id for c in move_edge().post if c.kind == "local" and c.map == "locations"]
        self.assertEqual(["from", "to"], ids)

    def test_a_move_of_two_instances_is_not_one_of_a_single_instance(self):
        self.assertFalse(moves_one_instance(move_edge()))

    def test_a_local_clause_that_does_not_say_its_instance_is_refused(self):
        # Setup
        x = json.loads(MOVE.read_text())
        for e in x["edges"]:
            for c in e["post"]:
                c.pop("id", None) if c["kind"] == "local" else None

        # Execute, Assert
        with self.assertRaisesRegex(ValueError, "run csdfpromote again"):
            Diagram.from_json(x)

    def test_a_distinct_clause_reads_from_json(self):
        c = Clause.from_json({"kind": "distinct", "text": "a ≠ b", "map": "m", "ids": ["a", "b"]})
        self.assertEqual(("a", "b"), c.ids)


if __name__ == "__main__":
    unittest.main()
