"""What a clause reads, as the reductions of the walk take it."""
import unittest
from csdfbmc.tests import fixture

from csdfbmc.meaning import OPAQUE, GlueMeaning, Renaming, standard
from csdfbmc.model import Clause, Diagram
from csdfbmc.protocol import Analysis
from csdfbmc.tests.test_protocol import Recording


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




RENAMED = Clause("local", "y > 0", rename=(("y", "x"),))


class RenamingTest(unittest.TestCase):
    """A renamed clause reads its own name for a value the predicate it is in
    names otherwise: the clause is asked about with that name bound to it."""

    def setUp(self):
        self.client = Recording({"y > 0": Analysis(opaque=False, reads=("y",))}, lambda b: b["y"] > 0)
        self.meaning = standard(GlueMeaning(self.client))

    def test_the_clause_reads_the_names_of_the_predicate_it_is_in(self):
        self.assertEqual({"x"}, self.meaning.names(RENAMED, "guard"))

    def test_the_clause_is_evaluated_with_its_name_bound_to_the_value(self):
        self.assertIs(True, self.meaning.evaluate(RENAMED, "guard", {"x": 1}))
        self.assertEqual([[{"y": 1}]], self.client.asked)

    def test_a_clause_without_a_renaming_is_left_alone(self):
        self.assertEqual({"y"}, self.meaning.names(Clause("local", "y > 0"), "guard"))

    def test_a_renaming_is_read_from_json(self):
        self.assertEqual(RENAMED, Clause.from_json({"kind": "local", "form": "text", "text": "y > 0", "rename": {"y": "x"}}))


class RenamedWritesTest(unittest.TestCase):
    def test_what_a_renamed_post_writes_is_named_as_its_predicate_names_it(self):
        client = Recording({"[{'y': 1}]": Analysis(opaque=False, reads=(), writes=("y",))}, lambda b: [{"y": 1}])
        clause = Clause("local", "[{'y': 1}]", rename=(("y", "x"),))
        self.assertEqual({"x"}, standard(GlueMeaning(client)).writes(clause))


class RenamedOntoNothingTest(unittest.TestCase):
    def test_a_renaming_onto_a_name_the_step_does_not_bind_is_refused(self):
        meaning = standard(GlueMeaning(Recording({"y > 0": Analysis(opaque=False, reads=("y",))}, lambda b: True)))
        with self.assertRaisesRegex(KeyError, "renamed onto"):
            meaning.evaluate(RENAMED, "guard", {"y": 1})



EQUAL = Clause("local", "@eq(y, z)", eq=("y", "z"))


class EqualityTest(unittest.TestCase):
    """@eq(y, z) is the tools' own clause: the values named y and z are equal.
    No glue is asked about it."""

    def setUp(self):
        self.client = Recording({}, lambda b: True)
        self.meaning = standard(GlueMeaning(self.client))

    def test_it_reads_its_two_names(self):
        self.assertEqual({"y", "z"}, self.meaning.names(EQUAL, "guard"))

    def test_it_holds_where_the_two_values_are_equal(self):
        self.assertEqual([True, False], [self.meaning.evaluate(EQUAL, "guard", {"y": 1, "z": z}) for z in (1, 2)])
        self.assertEqual([], self.client.asked)

    def test_it_is_read_from_json(self):
        self.assertEqual(EQUAL, Clause.from_json({"kind": "local", "form": "eq", "eq": ["y", "z"]}))

    def test_in_a_post_it_admits_one_assignment_of_nothing_where_the_values_are_equal(self):
        self.assertEqual([[{}], []], [self.meaning.evaluate(EQUAL, "post", {"y": 1, "z": z}) for z in (1, 2)])

    def test_it_writes_nothing(self):
        self.assertEqual(set(), self.meaning.writes(EQUAL))

    def test_values_of_another_kind_are_not_equal(self):
        self.assertIs(False, self.meaning.evaluate(EQUAL, "guard", {"y": 1, "z": True}))

    def test_a_name_the_step_does_not_bind_is_refused(self):
        with self.assertRaisesRegex(KeyError, "'z'"):
            self.meaning.evaluate(EQUAL, "guard", {"y": 1})



class Beyond:
    """A meaning that says, for any clause, it reads everything but a map
    read at own_id, and how a clause tells IDs apart: by naming one of the
    params it is asked about."""
    readable = by_prose = True

    def names_beyond(self, clause, role, own_id):
        return {"x", "y"} - {own_id}

    def tells_apart(self, clause, role, params, maps, groups):
        return "named" if "y" in params else None


class RenamingReductionTest(unittest.TestCase):
    """What the reductions ask of a renamed clause is asked in the clause's
    own names."""

    def test_names_beyond_asks_with_the_clause_s_name_for_the_own_id(self):
        for name, rename, own_id, want in [
            ("a name renamed onto the own ID", (("y", "x"),), "x", {"x"}),
            ("the own ID, not renamed", (), "x", {"y"}),
            ("the own ID's name renamed away: no name in the clause is it", (("x", "z"),), "x", {"z", "y"}),
            ("two names renamed onto the own ID", (("x", "w"), ("y", "w")), "w", {"w"}),
        ]:
            with self.subTest(name):
                # Setup
                clause = Clause("local", "c", rename=rename)

                # Execute
                got = Renaming(Beyond()).names_beyond(clause, "guard", own_id)

                # Assert
                self.assertEqual(want, got)

    def test_tells_apart_asks_with_the_clause_s_names_for_the_params(self):
        for name, rename, params, want in [
            ("a name renamed onto a param is one", (("y", "x"),), {"x"}, "named"),
            ("a param's name renamed away is not", (("y", "z"),), {"y"}, None),
        ]:
            with self.subTest(name):
                # Setup
                clause = Clause("local", "c", rename=rename)

                # Execute
                got = Renaming(Beyond()).tells_apart(clause, "guard", params, set(), [])

                # Assert
                self.assertEqual(want, got)

    def test_the_tool_words_tell_no_ids_apart(self):
        for name, clause in [("@eq", EQUAL)]:
            with self.subTest(name):
                # Execute
                got = standard(Beyond()).tells_apart(clause, "guard", {"y"}, set(), [])

                # Assert
                self.assertIsNone(got)


if __name__ == "__main__":
    unittest.main()
