"""A glue speaks the protocol its hello agreed on, and only that one."""
import unittest

from csdfbmc.protocol import NO_SYMMETRIC, atom_handler, handler, speaking
from csdfbmc.python_glue import evaluator

ATOM = {"text": ["", " > 0"], "holes": [0], "primed": [False]}


def protocol_1():
    return handler("fake", analyse=lambda c, r: None, guard=lambda c, b: True, post=lambda c, b: [])


def protocol_2():
    return atom_handler("fake", guard=lambda a, vs: True, post=lambda a, vs: [], symmetric=NO_SYMMETRIC)


class NegotiationTest(unittest.TestCase):
    def test_a_request_of_the_protocol_not_agreed_is_refused(self):
        tests = {
            "an atom after hello of protocol 1": (evaluator([]), {"op": "hello"},
                                                  {"op": "guard", "atom": ATOM, "values": [[1]]}, "protocol 1"),
            "analyse after hello of protocol 2": (evaluator([]), {"op": "hello", "protocol": 2},
                                                  {"op": "analyse", "clause": "n > 0", "role": "guard"}, "protocol 2"),
            "beyond after hello of protocol 2": (evaluator([]), {"op": "hello", "protocol": 2},
                                                 {"op": "beyond", "clause": "n > 0", "role": "guard", "own_id": None}, "protocol 2"),
            "a clause after hello of protocol 2": (evaluator([]), {"op": "hello", "protocol": 2},
                                                   {"op": "guard", "clause": "n > 0", "bindings": [{"n": 1}]}, "protocol 2"),
            "a request before hello": (evaluator([]), None, {"op": "guard", "atom": ATOM, "values": [[1]]}, "hello"),
        }
        for name, (handle, hello, request, said) in tests.items():
            with self.subTest(name):
                # Setup
                if hello is not None:
                    handle(hello)

                # Execute
                answer = handle(request)

                # Assert
                self.assertIn(said, answer.get("error", ""))

    def test_a_hello_asking_a_protocol_the_glue_does_not_speak_is_refused(self):
        tests = {
            "protocol 2 of a glue without atoms": (speaking({1: protocol_1()}), {"op": "hello", "protocol": 2}),
            "protocol 1 of a glue of atoms only": (speaking({2: protocol_2()}), {"op": "hello"}),
            "protocol 3": (evaluator([]), {"op": "hello", "protocol": 3}),
        }
        for name, (handle, hello) in tests.items():
            with self.subTest(name):
                # Execute
                answer = handle(hello)

                # Assert
                self.assertIn("speaks protocol", answer.get("error", ""))

    def test_a_glue_of_atoms_without_symmetric_supports_nothing(self):
        # Setup
        handle = speaking({2: protocol_2()})

        # Execute
        hello = handle({"op": "hello", "protocol": 2})
        symmetric = handle({"op": "symmetric", "atom": ATOM, "params": [0], "maps": [], "groups": [["a", "b"]]})

        # Assert
        self.assertEqual({"language": "fake", "protocol": 2}, hello)
        self.assertIn("symmetric", symmetric.get("error", ""))


if __name__ == "__main__":
    unittest.main()
