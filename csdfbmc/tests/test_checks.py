import dataclasses
import os
import tempfile
import textwrap
import unittest

from csdfbmc.checks import Checks, load_checks, PurityError
from csdfbmc.values import sample


class TypesTest(unittest.TestCase):
    def test_a_name_ranges_over_the_type_typeof_gives_it(self):
        # Setup
        checks = Checks(types={"口座ID": ["a1", "a2"]}, typeof={"相手ID": "口座ID"})

        # Execute
        values = checks.domain("相手ID")

        # Assert
        self.assertEqual(["a1", "a2"], values)
        self.assertTrue(checks.declares("相手ID"))

    def test_the_samples_follow_the_types_they_are_replaced_by(self):
        # Setup
        checks = Checks(types={"ID": sample(["a1"], beyond=["a2"])})

        # Execute
        replaced = dataclasses.replace(checks, types={"ID": ["a1", "a2"]})

        # Assert
        self.assertEqual((frozenset({"ID"}), frozenset()), (checks.samples, replaced.samples))

    def test_a_name_typeof_does_not_name_takes_the_type_of_its_own_name(self):
        # Setup
        checks = Checks(types={"額": [1, 2]}, typeof={"相手ID": "口座ID"})

        # Execute
        values = checks.domain("額")

        # Assert
        self.assertEqual([1, 2], values)
        self.assertFalse(checks.declares("相手ID"))


class LoadChecksTest(unittest.TestCase):
    def write(self, source):
        d = tempfile.mkdtemp()
        path = os.path.join(d, "checks.py")
        with open(path, "w", encoding="utf-8") as f:
            f.write(textwrap.dedent(source))
        return path

    def test_loads_the_types_ghosts_and_invariants_of_a_module(self):
        # Setup
        path = self.write('''
            UNIT = 2
            TYPES = {"n": range(0, 4), "額": [2, 4]}
            INVARIANTS = {"額 は最小単位の倍数": lambda 額: 額 % UNIT == 0}
        ''')

        # Execute
        checks = load_checks(path)

        # Assert
        self.assertIsInstance(checks, Checks)
        self.assertEqual([2, 4], list(checks.types["額"]))
        self.assertTrue(checks.invariants["額 は最小単位の倍数"](額=4))

    def test_refuses_a_module_that_is_not_pure(self):
        for name, body in {
            "assignment inside a function": "def f(n):\n    m = n\n    return m\n",
            "a loop": "def f(n):\n    for i in range(n):\n        pass\n    return True\n",
            "a float": "X = 0.5\n",
            "input or output": "print(1)\n",
            "randomness": "import random\n",
        }.items():
            with self.subTest(name):
                with self.assertRaises(PurityError):
                    load_checks(self.write("TYPES = {}\n" + body))

    def test_loads_the_types_and_typeof_of_a_module(self):
        # Setup
        path = self.write('''
            TYPES = {"口座ID": ["a1", "a2"]}
            TYPEOF = {"相手ID": "口座ID"}
        ''')

        # Execute
        checks = load_checks(path)

        # Assert
        self.assertEqual(["a1", "a2"], checks.domain("相手ID"))

    def test_a_type_given_as_a_sample_ranges_over_its_values_and_is_known_as_a_sample(self):
        tests = {
            "a sample": ('TYPES = {"ID": sample(["a1", "a2"], beyond=["a3"]), "額": [1]}\n', {"ID"}),
            "plain values only": ('TYPES = {"ID": ["a1", "a2"], "額": [1]}\n', set()),
            "a record over a sample": ('IDS = sample(["a1", "a2"], beyond=["a3"])\nTYPES = {"ID": IDS, "額": [r.id for r in record(id=IDS)]}\n', {"ID"}),
            "a record built of a sample": ('IDS = sample(["a1", "a2"], beyond=["a3"])\nTYPES = {"ID": IDS, "Tx": record(id=IDS)}\n', {"ID", "Tx"}),
        }
        for name, (body, want) in tests.items():
            with self.subTest(name):
                # Setup
                path = self.write("from csdfbmc.values import record, sample\n" + body)

                # Execute
                checks = load_checks(path)

                # Assert
                self.assertEqual(["a1", "a2"], checks.domain("ID"))
                self.assertEqual(want, set(checks.samples))

    def test_a_sample_with_no_value_beyond_it_or_one_of_its_own_is_refused(self):
        tests = {
            "no value beyond": ('TYPES = {"ID": sample(["a1"], beyond=[])}\n', "at least one value beyond"),
            "a value of the sample beyond it": ('TYPES = {"ID": sample(["a1"], beyond=["a1"])}\n', "which is a value of it"),
        }
        for name, (body, said) in tests.items():
            with self.subTest(name):
                # Setup
                path = self.write("from csdfbmc.values import sample\n" + body)

                # Execute, Assert
                with self.assertRaisesRegex(ValueError, said):
                    load_checks(path)

    def test_loads_the_environment_of_a_module(self):
        # Setup
        path = self.write('''
            TYPES = {}
            ENVIRONMENT = {"在庫 が 十分": lambda 在庫: 在庫 > 1, "$n が 正": True, "$m が 負": None}
        ''')

        # Execute
        checks = load_checks(path)

        # Assert
        self.assertEqual({"在庫 が 十分", "$n が 正", "$m が 負"}, set(checks.environment))
        self.assertIsNone(checks.environment["$m が 負"])
        self.assertTrue(checks.environment["在庫 が 十分"](在庫=2))

    def test_refuses_a_module_that_would_otherwise_be_misread(self):
        # Setup
        cases = [
            ("an environment value neither a truth value nor a function", 'TYPES = {}\nENVIRONMENT = {"p": 1}\n', "'p'"),
            ("an environment name that is not text", 'TYPES = {}\nENVIRONMENT = {1: True}\n', "names a function by 1"),
            ("a module without types", "INVARIANTS = {}\n", "TYPES"),
            ("the DOMAINS an earlier csdfbmc read, which would be ignored", 'DOMAINS = {"額": [1]}\n', "TYPES"),
            ("a TYPEOF naming no type", 'TYPES = {}\nTYPEOF = {"相手ID": "口座ID"}\n', "口座ID"),
            ("the CLAUSES an earlier csdfbmc read, which a glue reads now", "TYPES = {}\nCLAUSES = {}\n", "glue"),
        ]
        for name, source, message in cases:
            with self.subTest(name):
                path = self.write(source)

                # Execute, Assert
                with self.assertRaisesRegex(PurityError, message):
                    load_checks(path)

    def test_an_exists_ranges_over_the_type_it_names(self):
        # Setup
        checks = Checks(types={"口座ID": ["a1", "a2"]}, typeof={"相手ID": "口座ID"})

        # Execute, Assert
        self.assertEqual(["a1", "a2"], checks.carrier("口座ID"))
        with self.assertRaisesRegex(KeyError, "相手ID"):
            checks.carrier("相手ID")


if __name__ == "__main__":
    unittest.main()
