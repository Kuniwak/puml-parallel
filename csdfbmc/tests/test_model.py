"""The diagram as csdfparse prints it: states, edges with their events, and
clauses as formulas."""
import json
import unittest
from pathlib import Path

from csdfbmc.model import Atom, Bound, Builtin, Diagram, Exists, Free, Index
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import continues_name, atom, builtin, diagram, edge, end, exists, family, idx

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

    def test_reads_the_local_states_of_each_family(self):
        # Setup
        doc = diagram({"G": ["m"]}, "G", families=[family("m", "none", {"none": [], "open": ["bal", "fee"]}, key="acct")])

        # Execute
        d = Diagram.from_json(doc)

        # Assert
        f = d.families["m"]
        self.assertEqual(("acct", "none", ("none", "open"), {"none": (), "open": ("bal", "fee")}), (f.key, f.init, f.states, f.vars))

    def test_refuses_families_that_do_not_fit_the_diagram(self):
        at = lambda state: edge("G", "G", "c?k", post=[builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state=state)])
        tests = {
            "a family of a map no state holds": (diagram({"G": []}, "G", families=[family("m", "none", {"none": []})]),
                                                 "m is no variable of any state"),
            "two families of one map": (diagram({"G": ["m"]}, "G", families=[family("m", "none", {"none": []})] * 2),
                                        "two families of m"),
            "a start state that is none of its states": (diagram({"G": ["m"]}, "G", families=[family("m", "gone", {"none": []})]),
                                                         "its start state gone is none of its states"),
            "a start state that holds variables": (diagram({"G": ["m"]}, "G", families=[family("m", "none", {"none": ["bal"]})]),
                                                   "its start state none, which is absence, holds variables"),
            "variables of a state not its own": (diagram({"G": ["m"]}, "G", families=[dict(family("m", "none", {"none": []}), vars={"none": [], "x": []})]),
                                                 "its variables are of the states"),
            "a family without its key": (diagram({"G": ["m"]}, "G", families=[{k: v for k, v in family("m", "none", {"none": []}).items() if k != "key"}]),
                                         "lacks \\['key'\\]"),
            "a key that names nothing": (diagram({"G": ["m"]}, "G", families=[family("m", "none", {"none": []}, key="")]),
                                         "no key names the IDs of its map"),
            "a key a family does not have": (diagram({"G": ["m"]}, "G", families=[dict(family("m", "none", {"none": []}), ids="k")]),
                                             "keys it does not read, \\['ids'\\]"),
            "@at of a state of no family's": (diagram({"G": ["m"]}, "G", edges=[at("shut")], families=[family("m", "none", {"none": [], "open": []})]),
                                              "@at\\(m'\\[…\\], shut\\), but shut is no state of the family of m"),
            "@at of the start state, which is absence": (diagram({"G": ["m"]}, "G", edges=[at("none")],
                                                                 families=[family("m", "none", {"none": [], "open": []})]),
                                                         "none is the start state of the family of m, which is absence; say @notin"),
        }
        for name, (doc, said) in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(ValueError, said):
                    Diagram.from_json(doc)

    def test_refuses_what_it_cannot_read(self):
        tests = {
            "another version": (lambda d: d.update(version=1), "version 1"),
            "a conjunct of an unknown kind": (lambda d: d["init_post"].append({"kind": "or"}), "kind 'or'"),
            "a term of an unknown kind": (lambda d: d["init_post"].append(builtin("empty", {"kind": "var"})), "kind 'var'"),
            "an unknown key": (lambda d: d.update(promotes=[]), "keys it does not read, \\['promotes'\\]"),
            "a missing key": (lambda d: d.pop("labels"), "lacks \\['labels'\\]"),
            "a key of another kind": (lambda d: d["init_post"].append(dict(builtin("empty", "m'"), type="T")), "keys it does not read, \\['type'\\]"),
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
