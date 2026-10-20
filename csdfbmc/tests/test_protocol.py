"""The evaluator protocol carries values in a tagged JSON encoding, since
JSON has no tuples, sets or maps with keys other than strings."""
import json
import unittest

from csdfbmc.protocol import from_wire, to_wire
from csdfbmc.values import FrozenMap


class WireEncodingTest(unittest.TestCase):
    def test_scalars_are_themselves(self):
        for v in (0, -3, "残高", True, False, None):
            with self.subTest(v=v):
                self.assertEqual(v, to_wire(v))
                self.assertEqual(v, from_wire(to_wire(v)))

    def test_containers_are_tagged_and_come_back_equal(self):
        v = FrozenMap({"t1": FrozenMap({"state": "署名済み", "手数料": 2}), 3: (1, frozenset({"a"}))})
        wire = json.loads(json.dumps(to_wire(v)))
        self.assertEqual(v, from_wire(wire))
        self.assertEqual({"tuple": [1, 2]}, to_wire((1, 2)))
        self.assertEqual({"set": [1, 2]}, to_wire(frozenset({2, 1})))
        self.assertEqual({"map": [["k", 1]]}, to_wire(FrozenMap({"k": 1})))

    def test_a_boolean_stays_a_boolean(self):
        self.assertIs(True, from_wire(to_wire(True)))
        self.assertIsNot(True, from_wire(to_wire(1)))

    def test_an_untagged_array_or_object_is_refused(self):
        for wire in ([1, 2], {"a": 1}, {"set": [1], "map": []}, 1.5):
            with self.subTest(wire=wire), self.assertRaises(ValueError):
                from_wire(wire)

    def test_a_value_outside_the_model_is_refused(self):
        with self.assertRaises(TypeError):
            to_wire(1.5)


if __name__ == "__main__":
    unittest.main()
