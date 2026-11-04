"""pruned_product: the product of the levels, a prefix a check rejects
dropped, the checks asked as asking them of every complete tuple would."""
import itertools
import unittest

from csdfbmc.product import pruned_product


def run(levels, checks):
    """The tuples, the checks asked as (depth, prefix), and the prefixes
    pruned."""
    asked, pruned = [], []

    def at(d, check):
        return None if check is None else lambda prefix: asked.append((d, prefix)) or check(prefix)

    out = list(pruned_product([lambda prefix, vs=vs: vs(prefix) if callable(vs) else vs for vs in levels],
                              [at(d, c) for d, c in enumerate(checks)], pruned.append))
    return out, asked, pruned


def full(levels, checks):
    """The checks asked of every complete tuple in turn, each up to the
    first false, duplicates removed, and the tuples all hold of."""
    asked, out = [], []
    for t in itertools.product(*levels):
        for d, c in enumerate(checks):
            if c is None:
                continue
            if (d, t[:d]) not in asked:
                asked.append((d, t[:d]))
            if not c(t[:d]):
                break
        else:
            out.append(t)
    return out, asked


class PrunedProductTest(unittest.TestCase):
    def test_the_tuples_and_the_checks_asked_are_those_of_the_full_product_without_duplicates(self):
        tests = {
            "no checks": ([[1, 2], ["a", "b"]], [None, None, None]),
            "a check of the empty prefix": ([[1, 2], ["a"]], [lambda p: True, None, None]),
            "a false check of the empty prefix": ([[1, 2], ["a"]], [lambda p: False, None, None]),
            "mixed prefixes": ([[1, 2, 3], ["a", "b"], [0, 1]],
                               [None, lambda p: p[0] != 2, lambda p: p[1] == "b" or p[0] == 3, lambda p: p[2] == 1]),
            "a check only of complete tuples": ([[1, 2], [0, 1]], [None, None, lambda p: p == (2, 1)]),
        }
        for name, (levels, checks) in tests.items():
            with self.subTest(name):
                # Execute
                out, asked, _ = run(levels, checks)

                # Assert
                self.assertEqual(full(levels, checks), (out, asked))

    def test_a_prefix_rejected_is_told_and_nothing_below_it_is_enumerated(self):
        # Setup
        seen = []

        def second(prefix):
            seen.append(prefix)
            return [0, 1]

        # Execute
        out, _, pruned = run([[1, 2], second], [None, lambda p: p == (2,), None])

        # Assert
        self.assertEqual(([(2, 0), (2, 1)], [(1,)], [(1,), (2,)]), (out, pruned, seen))

    def test_a_check_is_not_asked_of_a_prefix_no_complete_tuple_extends(self):
        # Execute
        out, asked, pruned = run([[1, 2], lambda p: [] if p == (1,) else [0]], [None, lambda p: True, None])

        # Assert
        self.assertEqual(([(2, 0)], [(1, (2,))], []), (out, asked, pruned))


if __name__ == "__main__":
    unittest.main()
