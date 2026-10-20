"""A finding carries what a REPL needs to replay it: the initial values and,
per step, the edge taken and the values entered."""
import json
import unittest

from csdfbmc.model import Diagram
from csdfbmc.explore import Explorer
from csdfbmc.tests.test_explore import COUNTER, COUNTER_TWINS
from csdfbmc.tests.test_promoted import EXPANSION, GLOBAL_TWINS


class ReplayTest(unittest.TestCase):
    def test_a_deadlock_carries_its_replay(self):
        result = Explorer(Diagram.from_json(COUNTER), COUNTER_TWINS).run()
        out = result.to_json()
        replay = out["deadlocks"][0]["replay"]
        self.assertEqual([2], replay["initial"])
        self.assertEqual(
            [{"event": "dec", "src": "a", "dst": "a", "line": 6, "values": [1]},
             {"event": "dec", "src": "a", "dst": "a", "line": 6, "values": [0]},
             {"event": "stop", "src": "a", "dst": "b", "line": 7, "values": []}],
            replay["steps"])
        json.dumps(out)  # every value is JSON

    def test_map_values_are_encoded_as_objects(self):
        result = Explorer(Diagram.from_json(EXPANSION), GLOBAL_TWINS).run()
        out = result.to_json()
        replay = out["deadlocks"][0]["replay"]
        self.assertEqual([{}], replay["initial"])
        first = replay["steps"][0]
        self.assertEqual("OPEN(口座ID, 額)", first["event"])
        (record,) = first["values"][0].values()
        self.assertEqual("開設済み", record["state"])
        self.assertIn("残高", record)
        json.dumps(out)


if __name__ == "__main__":
    unittest.main()
