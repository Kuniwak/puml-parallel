"""csdfbmc shapes: the attribute trees of the values that the formal clauses
reveal, read from the clauses' syntax alone."""
import io
import json
import os
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout

from csdfbmc.shapes import main, shapes
from csdfbmc.values import record, sample
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

    def test_every_variable_is_listed_with_its_type_and_a_map_type_is_no_shape(self):
        # Setup
        f = family("m", "none", {"none": [], "on": ["r"]}, types={"r": "Role"})
        d = diagram({"s": ["m", "n"]}, "s", edges=[edge("s", "s", "go?k", guard=[atom("$n == 0 and $m[$k] == $m[$k]")])],
                    families=[f], types={"m": "K ⇸ Inst", "n": "Nat", "k": "K"})

        # Execute
        got = shapes(d)

        # Verify
        self.assertEqual(got, {"types": {"Nat": {"kind": "scalar", "examples": [0]}, "Inst": {"kind": "scalar"},
                                         "K": {"kind": "scalar"}},
                               "vars": {"m": "K ⇸ Inst", "n": "Nat", "r": "Role"}, "declared": []})

    def test_a_type_types_gives_as_a_record_has_its_fields_values_in_place_of_what_the_clauses_reveal(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", init_post=[atom("$r' == {'level': 'low', 'other': 1}")], types={"r": "Role"})

        # Execute
        got = shapes(d, {"Role": record(level=["low", "high"], size=range(2))})

        # Verify
        self.assertEqual(got["types"]["Role"], {"kind": "record", "fields": {
            "level": {"kind": "scalar", "values": ["low", "high"]},
            "size": {"kind": "scalar", "values": [0, 1], "numeric": True}}})

    def test_a_sample_gives_examples_and_values_it_cannot_list_give_a_bare_scalar(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", types={"r": "Role"})
        given = {"Id": sample(["a"], beyond=["b"]), "Big": (n for n in range(3)),
                 "Pair": [(1, 2)], "Map": [{"a": 1}]}

        # Execute
        got = shapes(d, given)

        # Verify
        self.assertEqual(got["types"], {"Id": {"kind": "scalar", "examples": ["a"]}, "Big": {"kind": "scalar"},
                                        "Pair": {"kind": "scalar"}, "Map": {"kind": "scalar"}})


class CommandTest(unittest.TestCase):
    def run_main(self, *args):
        out, err = io.StringIO(), io.StringIO()
        with redirect_stdout(out), redirect_stderr(err):
            code = main(list(args))
        return code, out.getvalue(), err.getvalue()

    def test_the_command_prints_the_shapes_of_the_diagram_as_json(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", init_post=[atom("$r' == {'level': 'low'}")], types={"r": "Role"})
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        path = os.path.join(tmp.name, "d.json")
        with open(path, "w", encoding="utf-8") as f:
            json.dump(d, f)

        # Execute
        code, out, _ = self.run_main(path)

        # Verify
        self.assertEqual((code, json.loads(out)), (0, shapes(d)))

    def test_the_command_takes_the_shapes_of_the_types_a_checks_module_gives_from_it(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", init_post=[atom("$r' == {'level': 'low'}")], types={"r": "Role"})
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        path, checks = os.path.join(tmp.name, "d.json"), os.path.join(tmp.name, "defs.py")
        with open(path, "w", encoding="utf-8") as f:
            json.dump(d, f)
        with open(checks, "w", encoding="utf-8") as f:
            f.write("TYPES = {'Role': ['low', 'high']}\n")

        # Execute
        code, out, _ = self.run_main(path, checks)

        # Verify
        self.assertEqual((code, json.loads(out)["types"]["Role"]), (0, {"kind": "scalar", "values": ["low", "high"]}))

    def test_names_the_types_whose_shapes_come_from_types_and_not_the_others(self):
        # Setup
        d = diagram({"s": ["r", "n"]}, "s", init_post=[atom("$n' > 0")], types={"r": "Role", "n": "Nat"})

        # Execute
        got = shapes(d, {"Role": ["low", "high"]})

        # Verify
        self.assertEqual(got["declared"], ["Role"])

    def test_the_command_exits_2_on_a_checks_module_outside_the_pure_subset(self):
        # Setup
        d = diagram({"s": ["r"]}, "s", types={"r": "Role"})
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        path, checks = os.path.join(tmp.name, "d.json"), os.path.join(tmp.name, "defs.py")
        with open(path, "w", encoding="utf-8") as f:
            json.dump(d, f)
        with open(checks, "w", encoding="utf-8") as f:
            f.write("import os\nTYPES = {}\n")

        # Execute
        code, _, err = self.run_main(path, checks)

        # Verify
        self.assertEqual(code, 2)
        self.assertIn("csdfbmc shapes:", err)

    def test_the_command_exits_2_on_a_diagram_it_cannot_read(self):
        # Execute
        code, _, err = self.run_main("/no/such/diagram.json")

        # Verify
        self.assertEqual(code, 2)
        self.assertIn("csdfbmc shapes:", err)
