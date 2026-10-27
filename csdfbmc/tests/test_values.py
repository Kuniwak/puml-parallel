"""Hashable values for records and finite partial functions, and the builders
that enumerate their domains from the domains of their fields."""
import unittest

from csdfbmc.values import FrozenMap, FrozenSet, Rec, Strict, same_value, encode, freeze, partial_map, record, sample, samples_in, visible


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
        rs = record(状態=["A", "B"], 在庫=range(0, 3))
        self.assertEqual(6, len(rs))
        self.assertIn(FrozenMap({"状態": "B", "在庫": 2}), rs)
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

    def test_a_type_is_built_of_the_samples_record_and_partial_map_were_given(self):
        # Setup
        ids, other = sample(["t1"], beyond=["t2"]), sample([1], beyond=[2])
        tests = {
            "a sample": (ids, [ids]),
            "a record of a field of a sample": (record(id=ids, 額=[1]), [ids]),
            "a record of a record of a sample": (record(r=record(id=ids)), [ids]),
            "a map keyed by a sample, its values of another": (partial_map(ids, other), [ids, other]),
            "the same values listed by hand": ([FrozenMap({"id": "t1"})], []),
            "a record of values of no sample": (record(id=["t1"]), []),
        }
        for name, (values, want) in tests.items():
            with self.subTest(name):
                # Execute
                found = samples_in(values)

                # Assert
                self.assertEqual([id(x) for x in want], [id(x) for x in found])

    def test_a_sample_takes_values_beyond_it_that_are_not_in_it(self):
        tests = {
            "a value beyond": (["t1"], ["t2"]),
            "values beyond, of records": ([FrozenMap({"id": "t1"})], [FrozenMap({"id": "t2"})]),
            "1 beyond a sample of True": ([True], [1]),
        }
        for name, (values, beyond) in tests.items():
            with self.subTest(name):
                # Execute
                s = sample(values, beyond=beyond)

                # Assert
                self.assertEqual(tuple(beyond), s.beyond)

    def test_a_sample_with_no_value_beyond_it_or_one_of_its_own_is_refused(self):
        tests = {
            "no value beyond": ((["t1"], []), "at least one value beyond"),
            "a value beyond that is in the sample": ((["t1", "t2"], ["t3", "t2"]), "'t2' beyond the sample, which is a value of it"),
        }
        for name, ((values, beyond), refused) in tests.items():
            with self.subTest(name):
                # Execute / Assert
                with self.assertRaisesRegex(ValueError, refused):
                    sample(values, beyond=beyond)


class SameValueTest(unittest.TestCase):
    def test_tells_values_apart_by_kind(self):
        tests = {
            "an integer and the float it equals": (1, 1.0, False),
            "a truth value and the number it equals": (True, 1, False),
            "an integer and the float it equals in a tuple": ((1,), (1.0,), False),
            "an integer and the float it equals in a map": (FrozenMap(a=1), FrozenMap(a=1.0), False),
            "a float and itself": (1.5, 1.5, True),
        }
        for name, (a, b, want) in tests.items():
            with self.subTest(name):
                self.assertEqual(want, same_value(a, b))


class KindTest(unittest.TestCase):
    def test_containers_tell_values_apart_by_kind(self):
        tests = {
            "the keys of a map": lambda: len(FrozenMap([(1, "a"), (True, "b"), (1.0, "c")])),
            "the elements of a set": lambda: len(FrozenSet([1, True, 1.0])),
            "the elements of a frozen set": lambda: len(freeze([{1}, {True}, {1.0}])[0] | freeze({True}) | freeze({1.0})),
            "the states in a set": lambda: len({Strict(("a", (1,), ())), Strict(("a", (True,), ())), Strict(("a", (1.0,), ()))}),
        }
        for name, count in tests.items():
            with self.subTest(name):
                self.assertEqual(3, count())

    def test_a_map_reads_the_entry_of_the_key_of_its_kind(self):
        m = FrozenMap([(1, "a"), (True, "b")])
        self.assertEqual("b", m[True])
        self.assertEqual("a", m[1])
        self.assertNotIn(1.0, m)

    def test_maps_and_sets_of_other_kinds_are_other_values(self):
        tests = {
            "maps": (FrozenMap({1: 0}), FrozenMap({True: 0})),
            "sets": (FrozenSet([1]), FrozenSet([1.0])),
            "states": (Strict(("a", (1,), ())), ("a", (True,), ())),
        }
        for name, (a, b) in tests.items():
            with self.subTest(name):
                self.assertNotEqual(a, b)

    def test_negative_zero_is_zero_in_a_container(self):
        self.assertIn(-0.0, FrozenSet([0.0]))
        self.assertEqual(Strict(("a", (0.0,), ())), Strict(("a", (-0.0,), ())))

    def test_a_container_equal_to_a_plain_one_hashes_alike(self):
        tests = {
            "a state and a tuple": (Strict(("a", (1,), ())), ("a", (1,), ())),
            "a set and a frozenset": (FrozenSet([1, "x"]), frozenset({1, "x"})),
        }
        for name, (strict, plain) in tests.items():
            with self.subTest(name):
                # Execute
                found = plain in {strict}

                # Assert
                self.assertTrue(found)

    def test_the_items_of_a_map_tell_kinds_apart(self):
        # Setup
        m = FrozenMap({1: 0})

        # Execute
        found = (True, 0) in m.items()

        # Assert
        self.assertFalse(found)
