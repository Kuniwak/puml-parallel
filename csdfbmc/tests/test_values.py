"""Hashable values for records and finite partial functions, and the builders
that enumerate their domains from the domains of their fields."""
import unittest

from csdfbmc.values import FrozenMap, freeze, record, partial_map


class FrozenMapTest(unittest.TestCase):
    def test_is_a_hashable_mapping_equal_by_content(self):
        a = FrozenMap({"x": 1, "y": (2, 3)})
        b = FrozenMap({"y": (2, 3), "x": 1})
        self.assertEqual(a, b)
        self.assertEqual(hash(a), hash(b))
        self.assertEqual(1, a["x"])
        self.assertEqual({"x", "y"}, set(a))
        self.assertIn(a, {b})

    def test_updates_return_new_maps(self):
        a = FrozenMap({"x": 1})
        b = a.set("y", 2)
        c = b.without("x")
        self.assertEqual(FrozenMap({"x": 1}), a)
        self.assertEqual(FrozenMap({"x": 1, "y": 2}), b)
        self.assertEqual(FrozenMap({"y": 2}), c)

    def test_attribute_access_reads_a_field(self):
        self.assertEqual(3, FrozenMap({"数量": 3}).数量)

    def test_freeze_turns_containers_into_their_hashable_forms(self):
        v = freeze({"a": [1, {2, 3}], "b": {"c": [4]}})
        self.assertIsInstance(v, FrozenMap)
        self.assertEqual((1, frozenset({2, 3})), v["a"])
        self.assertIsInstance(v["b"], FrozenMap)
        hash(v)

    def test_freeze_refuses_what_cannot_be_hashed(self):
        class Unhashable:
            __hash__ = None
        with self.assertRaises(TypeError):
            freeze(Unhashable())
        with self.assertRaises(TypeError):
            freeze({"k": Unhashable()})


class DomainBuilderTest(unittest.TestCase):
    def test_record_enumerates_the_product_of_its_fields(self):
        rs = record(状態=["A", "B"], 残高=range(0, 3))
        self.assertEqual(6, len(rs))
        self.assertIn(FrozenMap({"状態": "B", "残高": 2}), rs)
        self.assertTrue(all(isinstance(r, FrozenMap) for r in rs))

    def test_partial_map_enumerates_every_partial_function_up_to_the_size(self):
        ms = partial_map(["t1", "t2"], ["x", "y"], max_size=1)
        # the empty map, and one entry for each of two keys times two values
        self.assertEqual(1 + 4, len(ms))
        self.assertIn(FrozenMap({}), ms)
        self.assertIn(FrozenMap({"t2": "y"}), ms)
        full = partial_map(["t1", "t2"], ["x", "y"])
        self.assertEqual(1 + 4 + 4, len(full))
        self.assertIn(FrozenMap({"t1": "x", "t2": "y"}), full)
