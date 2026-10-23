"""The diagram as csdfparse prints it: states, edges with their events, and
clauses as formulas."""
import json
import unittest
from pathlib import Path

from csdfbmc.atoms import continues_name
from csdfbmc.model import Atom, Bound, Builtin, Diagram, Exists, Free, Index
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, builtin, diagram, edge, end, exists, idx

ROOT = Path(__file__).resolve().parents[2]


class DiagramTest(unittest.TestCase):
    def test_reads_states_edges_and_their_clauses(self):
        # Setup
        doc = diagram({"s0": ["n"], "s1": []}, "s0", init_post=[atom("$n' == 0")],
                      edges=[edge("s0", "s1", "c?x!n", guard=[atom("$x > $n")],
                                  post=[exists("T", builtin("at", idx("m'", fixture.bound(0)), state="open"))], line=4)],
                      ends=[end("s1")])

        # Execute
        d = Diagram.from_json(doc)

        # Assert
        e = d.edges[0]
        self.assertEqual(("s0", ("n",), "s0"), (d.states[0], d.vars["s0"], d.start))
        self.assertEqual(("c", (("?", "x"), ("!", "n")), "c?x!n", 4), (e.name, e.fields, e.event, e.line))
        self.assertEqual(("x",), e.params)
        self.assertEqual(("n",), e.sent)
        self.assertEqual(Atom(text=("", " > ", ""), holes=(0, 1), primed=(False, False), function="$_0 > $_1",
                              args=(Free("x"), Free("n"))), e.guard[0])
        self.assertEqual(Exists("T", (Builtin("at", (Index(Free("m", True), Bound(0)),), "open"),)), e.post[0])
        self.assertEqual("s1", d.ends[0].src)

    def test_refuses_what_it_cannot_read(self):
        tests = {
            "another version": (lambda d: d.update(version=2), "version 2"),
            "a conjunct of an unknown kind": (lambda d: d["init_post"].append({"kind": "or"}), "kind 'or'"),
            "a term of an unknown kind": (lambda d: d["init_post"].append(builtin("empty", {"kind": "var"})), "kind 'var'"),
            "a field of no direction": (lambda d: d["edges"].append(dict(edge("s0", "s0", "c"), fields=[{"dir": "=", "name": "x"}])), "direction '='"),
        }
        for name, (edit, said) in tests.items():
            with self.subTest(name):
                # Setup
                doc = diagram({"s0": []}, "s0")
                edit(doc)

                # Execute, Assert
                with self.assertRaisesRegex(ValueError, said):
                    Diagram.from_json(doc)


class FixtureTest(unittest.TestCase):
    """The diagrams the tests write by hand are those csdfparse prints."""

    def test_an_atom_is_written_as_csdfparse_writes_it(self):
        # Setup
        atoms = []
        for name in ("withdraw_formal.json", "wallet_formal.json"):
            printed = json.loads((ROOT / "examples" / "bmc" / name).read_text(encoding="utf-8"))
            atoms += [c for e in printed["edges"] for c in e["guard"] + e["post"] if c["kind"] == "atom"]
        self.assertGreater(len(atoms), 10)

        for a in atoms:
            with self.subTest(a["function"]):
                # Execute
                written = atom(a["pieces"][0] + "".join(_hole(a["args"][h], p) + p for h, p in zip(a["holes"], a["pieces"][1:])))

                # Assert
                self.assertEqual(a, written)


def _hole(t, after):
    """The hole of t as a test writes it, ended by a space where the text
    after it could continue its name."""
    return "$" + t["name"] + ("'" if t.get("primed") else "") + (" " if after and continues_name(after[0]) else "")


if __name__ == "__main__":
    unittest.main()
