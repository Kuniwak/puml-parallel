"""csdfbmc shapes: the attribute trees of the values that the formal clauses
reveal, read from the clauses' syntax alone."""
import unittest

from csdfbmc.shapes import shapes
from csdfbmc.tests.fixture import atom, diagram, edge, eq, family, fld, idx


class ShapesTest(unittest.TestCase):
    def test_an_attribute_a_guard_reads_is_a_field_of_the_variables_type(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", edges=[edge("s", "s", "go", guard=[atom("$r['level'] == 'high'")])],
                    types={"r": "Role"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got["types"]["Role"],
                         {"kind": "record", "fields": {"level": {"kind": "scalar", "examples": ["high"]}}})

    def test_the_keys_of_a_dict_a_post_builds_are_fields_of_the_variables_type(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", edges=[edge("s", "s", "go", post=[atom("$r' == {'scope': 'all', 'level': $r['level']}")])],
                    types={"r": "Role"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got["types"]["Role"],
                         {"kind": "record", "fields": {"scope": {"kind": "scalar", "examples": ["all"]},
                                                       "level": {"kind": "scalar"}}})

    def test_an_attribute_ordered_or_used_in_arithmetic_is_a_number(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", edges=[edge("s", "s", "go", guard=[atom("$r['rank'] < 3"), atom("$r['size'] + 1 == 2")])],
                    types={"r": "Role"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got["types"]["Role"]["fields"],
                         {"rank": {"kind": "scalar", "examples": [3], "numeric": True},
                          "size": {"kind": "scalar", "numeric": True}})

    def test_an_attribute_of_an_event_field_is_a_field_of_the_channels_type(self):
        # Setup
        d = diagram({"s": []}, "s", edges=[edge("s", "s", "go?j", guard=[atom("$j['pages'] == 1")])], types={"j": "Job"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got["types"]["Job"]["fields"], {"pages": {"kind": "scalar", "examples": [1]}})

    def test_a_builtin_terms_attribute_is_a_field_of_its_type(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", edges=[edge("s", "s", "go", guard=[eq(fld("r", "owner"), "r"), atom("${} == 'me'", fld(fld("r", "scope"), "kind"))])],
                    types={"r": "Role"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got["types"]["Role"]["fields"],
                         {"owner": {"kind": "scalar"},
                          "scope": {"kind": "record", "fields": {"kind": {"kind": "scalar", "examples": ["me"]}}}})

    def test_an_attribute_of_an_instances_variable_is_a_field_of_the_variables_type(self):
        # Setup
        f = family("m", "none", {"none": [], "on": ["r"]}, types={"r": "Role"})
        d = diagram({"s": ["m"]}, "s", edges=[edge("s", "s", "go?k", guard=[atom("${}['level'] == 'high'", fld(idx("m", "k"), "r"))])],
                    families=[f], types={"m": "K ⇸ Inst"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got["types"]["Role"]["fields"], {"level": {"kind": "scalar", "examples": ["high"]}})

    def test_an_attribute_of_a_maps_value_is_a_field_of_the_maps_value_type(self):
        # Setup
        d = diagram({"s": ["m"]}, "s", edges=[edge("s", "s", "go?k", guard=[atom("$m[$k]['level'] == 'high'")])],
                    types={"m": "K ⇸ Role"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got["types"]["Role"]["fields"], {"level": {"kind": "scalar", "examples": ["high"]}})

    def test_the_constants_an_attribute_is_found_among_are_its_examples(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", edges=[edge("s", "s", "go", guard=[atom("$r['level'] in ('low', 'high')")])],
                    types={"r": "Role"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got["types"]["Role"]["fields"], {"level": {"kind": "scalar", "examples": ["low", "high"]}})

    def test_an_attribute_a_constant_is_looked_for_in_is_a_collection_of_it(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", edges=[edge("s", "s", "go", guard=[atom("'admin' not in $r['tags']")])],
                    types={"r": "Role"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got["types"]["Role"]["fields"],
                         {"tags": {"kind": "collection", "items": {"kind": "scalar", "examples": ["admin"]}}})

    def test_the_start_states_post_reveals_attributes_too(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", init_post=[atom("$r' == {'level': 'low'}")], types={"r": "Role"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got["types"]["Role"]["fields"], {"level": {"kind": "scalar", "examples": ["low"]}})
