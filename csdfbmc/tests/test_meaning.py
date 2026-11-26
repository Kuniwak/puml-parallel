import unittest

from csdfbmc.environment import Environment
from csdfbmc.meaning import AtomMeaning
from csdfbmc.model import Atom, Seen
from csdfbmc.values import FrozenMap, Instance, Rec

GREATER = Atom(text=("", " > ", ""), holes=(0, 1), primed=(False, False), function="gt", args=())
NEXT = Atom(text=("", " == ", " + 1"), holes=(0, 1), primed=(True, False), function="next", args=())
NO_ENVIRONMENT = Environment(decide=lambda f, args: None, declared=frozenset(), given={}, env={})
SEEN = FrozenMap({"live": Seen(None, frozenset({"live"}), frozenset({"live", "gone"})),
                  "gone": Seen(None, frozenset({"gone"}), frozenset({"live", "gone"}))})


class CountingClient:
    """A glue that answers every guard true and every post with the value
    before plus one, and counts the lists of values it is asked about."""

    language = "fake"
    supports = frozenset()

    def __init__(self):
        self.asked = []

    def guard(self, atom, values):
        self.asked.extend(values)
        return [True for _ in values]

    def post(self, atom, values):
        self.asked.extend(values)
        return [[[vs[0] + 1]] for vs in values]


def meaning(client):
    return AtomMeaning(client, NO_ENVIRONMENT, carrier=lambda t: [], seen=SEEN)


class AtomMeaningTest(unittest.TestCase):
    def test_a_guard_is_asked_once_per_list_of_values(self):
        # Setup
        client = CountingClient()
        m = meaning(client)

        # Execute
        m.guard(GREATER, [3, 2])
        m.guard(GREATER, [3, 2])
        m.guard(GREATER, [1, 2])

        # Assert
        self.assertEqual([[3, 2], [1, 2]], client.asked)

    def test_a_post_is_asked_once_per_list_of_values_before_the_step(self):
        # Setup
        client = CountingClient()
        m = meaning(client)

        # Execute
        got = [m.post(NEXT, [2], []), m.post(NEXT, [2], [])]

        # Assert
        self.assertEqual(([[(3,)], [(3,)]], [[2]]), (got, client.asked))

    def test_a_guard_is_asked_again_of_an_instance_at_another_state(self):
        # Setup
        client = CountingClient()
        m = meaning(client)
        fields = FrozenMap({"cnt": 1})

        # Execute
        m.guard(GREATER, [FrozenMap(a=Rec("live", fields)), 0])
        m.guard(GREATER, [FrozenMap(a=Rec("gone", fields)), 0])
        m.guard(GREATER, [FrozenMap(a=Rec("live", fields)), 0])

        # Assert
        self.assertEqual(2, len(client.asked))

    def test_a_truth_value_is_not_the_number_it_equals(self):
        # Setup
        client = CountingClient()
        m = meaning(client)

        # Execute
        m.guard(GREATER, [1, 0])
        m.guard(GREATER, [True, 0])

        # Assert
        self.assertEqual(2, len(client.asked))

    def test_a_post_answer_holding_an_instance_holds_the_map_of_its_fields(self):
        # Setup
        live = Instance(FrozenMap({"cnt": 1}), at={"live"}, views={"live", "gone"})
        class Answering(CountingClient):
            def post(self, atom, values):
                return [[[FrozenMap(a=live)]] for _ in values]
        m = meaning(Answering())

        # Execute
        (after,), = m.post(NEXT, [2], [])

        # Assert
        self.assertIs(FrozenMap, type(after["a"]))


if __name__ == "__main__":
    unittest.main()
