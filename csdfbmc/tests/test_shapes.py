"""csdfbmc shapes: the attribute trees of the values that the formal clauses
reveal, read from the clauses' syntax alone."""
import unittest

from csdfbmc.shapes import shapes
from csdfbmc.tests.fixture import atom, diagram, edge


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
