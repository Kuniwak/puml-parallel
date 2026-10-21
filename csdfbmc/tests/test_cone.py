"""The variables outside the cone of influence are left out of the walk:
nothing that decides whether a step can be taken, and no property, reads
them, so the walk finds the same without them."""
import io
import json
import os
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from csdfbmc.tests import fixture

from csdfbmc import cone
from csdfbmc.cone import Cone
from csdfbmc.explore import Explorer
from csdfbmc.meaning import GlueMeaning, standard
from csdfbmc.model import Diagram
from csdfbmc.protocol import checked, start_module
from csdfbmc.twins import Twins

# A counter with an audit log nobody reads; the log may record either bit.
AUDITED = {
    "states": {
        "a": {"name": "counting", "vars": [{"name": "n"}, {"name": "log"}], "line": 2},
        "b": {"name": "stopped", "vars": [], "line": 3},
    },
    "start_edge": {"dst": "a", "post": "[{'n': 2}] ∧ [{'log': 0}]", "line": 4},
    "edges": [
        {"src": "a", "dst": "a", "event": "dec", "channel": "dec", "fields": [], "guard": "n > 0",
         "post": "[{'n': n - 1}] ∧ [{'log': v} for v in (0, 1)]", "line": 5},
        {"src": "a", "dst": "b", "event": "stop", "channel": "stop", "fields": [], "guard": "n == 0", "post": "", "line": 6},
    ],
    "end_edge": None,
    "assumptions": [],
}

NO_TWINS = Twins(domains={}, clauses={})


def memoed():
    """The formal expansion of test_labels with a memo nothing reads, which
    every edge carries across with a frame."""
    from csdfbmc.tests.test_labels import relabel
    from csdfbmc.tests.test_promoted import EXPANSION
    x = json.loads(json.dumps(relabel(EXPANSION)))
    x["diagram"]["states"]["running"]["vars"].append({"name": "memo"})
    x["diagram"]["start_edge"]["post"] += " ∧ [{'memo': 0}]"
    for e in x["edges"]:
        e["post"].append({"kind": "frame", "text": "memo' = memo", "var": "memo"})
    return x


# A family nothing reads, created into a local state whose variable no local
# post gives a value: the full walk reports it unconstrained.
UNREAD_FAMILY = {
    "diagram": {
        "states": {"r": {"name": "r", "vars": [{"name": "notes"}], "line": 1}},
        "start_edge": {"dst": "r", "post": "[{'notes': FrozenMap()}]", "line": 1},
        "edges": [{"src": "r", "dst": "r", "event": "NOTE?i", "channel": "NOTE", "fields": [{"dir": "?", "name": "i"}], "guard": "g", "post": "p", "line": 2}],
        "end_edge": None, "assumptions": [],
    },
    "edges": [{"edge": {"src": "r", "dst": "r", "event": "NOTE?i", "channel": "NOTE", "fields": [{"dir": "?", "name": "i"}], "guard": "g", "post": "p", "line": 2},
               "guard": [], "post": [{"kind": "create", "text": "notes' = …", "map": "notes", "id": "i", "state": "S"}]}],
    "promotes": [{"map": "notes", "id_param": "i", "path": "local/NOTE.puml", "in": "r"}],
    "locals": {"local/NOTE.puml": {"states": {"s": {"name": "S", "vars": [{"name": "n"}]}}}},
}


def edit(diagram: dict, which: str, **fields) -> dict:
    """diagram with the fields of the edge on event which replaced."""
    return dict(diagram, edges=[dict(e, **fields) if e["event"] == which else e for e in diagram["edges"]])


class ConeTest(unittest.TestCase):
    def walk(self, diagram: dict, *, reduce: bool, twins: Twins = NO_TWINS, **options):
        client = start_module("csdfbmc.python_glue", [])
        self.addCleanup(client.close)
        d = fixture.diagram(diagram)
        meaning = standard(GlueMeaning(checked(client)))
        c = cone.compute(d, twins, meaning, writes=GlueMeaning(checked(client)).writes) if reduce else Cone.keep_all()
        return Explorer(d, twins, meaning, cone=c, **options).run()

    def test_a_variable_nothing_reads_is_left_out_of_the_states(self):
        reduced = self.walk(AUDITED, reduce=True)
        self.assertEqual(6, len(self.walk(AUDITED, reduce=False).states))
        self.assertEqual({("a", (2, None), ()), ("a", (1, None), ()), ("a", (0, None), ()), ("b", (), ())}, set(reduced.states))
        self.assertEqual(("n",), reduced.relevant)

    def test_a_left_out_variable_is_listed_with_the_edges_that_set_it(self):
        self.assertEqual({"log": ["start", "a -dec-> a (line 5)"]}, self.walk(AUDITED, reduce=True).dropped)

    def test_the_findings_are_the_same_without_the_left_out_variables(self):
        for name, diagram in [
            ("an audit log", AUDITED),
            ("a post that admits no successor at n = 1", edit(AUDITED, "dec", post="[{'n': n - 1}] ∧ [{'log': v} for v in range(n - 1)]")),
            ("a post that leaves the log without a value", edit(AUDITED, "dec", post="[{'n': n - 1}]")),
            ("a tau cycle at the end", edit(AUDITED, "stop", event="tau", channel="tau", fields=[], dst="a", guard="n == 0", post="[{'n': n}] ∧ [{'log': 1}]")),
            ("an expansion that frames a memo", memoed()),
            ("a family nothing reads, missing a local value", UNREAD_FAMILY),
        ]:
            twins = Twins(domains={"口座ID": ["a1", "a2"], "額": [0, 1, 2], "i": ["a1"]}, clauses={})
            with self.subTest(name):
                full = self.walk(diagram, reduce=False, twins=twins).groups()
                reduced = self.walk(diagram, reduce=True, twins=twins).groups()
                self.assertEqual(self.shapes(full), self.shapes(reduced))

    @staticmethod
    def shapes(groups: list[dict]) -> list[tuple]:
        """The findings by shape and where, without the values of a state."""
        return [(g["kind"], g.get("at"), g.get("through"), g.get("trace"), g.get("variables")) for g in groups]

    def test_what_a_post_reads_is_kept_since_it_decides_the_step(self):
        appended = edit(AUDITED, "dec", post="[{'n': n - 1}] ∧ [{'log': log + 1}]")
        self.assertEqual(("n", "log"), self.walk(appended, reduce=True).relevant)

    def test_a_variable_an_invariant_or_a_ghost_reads_is_kept(self):
        for name, twins in [
            ("an invariant", Twins(domains={}, clauses={}, invariants={"log is a bit": lambda log: log in (0, 1)})),
            ("a ghost", Twins(domains={}, clauses={}, ghosts={"seen": (0, {"dec": lambda seen, log: seen + log})})),
        ]:
            with self.subTest(name):
                self.assertEqual(("n", "log"), self.walk(AUDITED, reduce=True, twins=twins, max_states=50).relevant)

    def test_what_may_read_anything_keeps_the_cone_from_being_computed(self):
        for name, diagram, twins, blocked in [
            ("an opaque post", edit(AUDITED, "dec", post="[{'n': n - 1}] ∧ @opaque"), NO_TWINS, "@opaque"),
            ("an invariant taking any keyword", AUDITED,
             Twins(domains={}, clauses={}, invariants={"anything": lambda **env: True}), "the invariant 'anything'"),
        ]:
            with self.subTest(name):
                reduced = self.walk(diagram, reduce=True, twins=twins, opaque="true", max_states=50)
                self.assertIsNone(reduced.relevant)
                self.assertEqual((blocked,), reduced.cone_blocked_by)


class FramedAndWrittenTest(unittest.TestCase):
    """A post that frames a variable and also writes it must agree on its
    value, which reads it; such a variable is kept."""

    def test_a_variable_framed_and_written_in_one_post_is_kept(self):
        for name, writes in [("a clause that writes it", lambda c: {"log"}), ("a clause whose writes are unknown", cone.NO_WRITES)]:
            post = [{"kind": "frame", "text": "log' = log", "var": "log"}, {"kind": "local", "form": "text", "text": "[{'log': 1}]"}]
            x = {"diagram": {**AUDITED}, "edges": [{"edge": AUDITED["edges"][0], "guard": [{"kind": "local", "form": "text", "text": "n > 0"}], "post": post}]}
            d = fixture.diagram(x)
            meaning = standard(GlueMeaning(checked(self.client())))
            with self.subTest(name):
                self.assertIn("log", cone.compute(d, NO_TWINS, meaning, writes=writes).relevant)

    def client(self):
        client = start_module("csdfbmc.python_glue", [])
        self.addCleanup(client.close)
        return client


class TwinsConeTest(unittest.TestCase):
    def test_a_twin_taking_any_keyword_keeps_the_cone_from_being_computed(self):
        from csdfbmc.meaning import TwinsMeaning
        d = fixture.diagram(edit(AUDITED, "dec", guard="positive"))
        twins = Twins(domains={}, clauses={"positive": lambda **env: env["n"] > 0})
        c = cone.compute(d, twins, standard(TwinsMeaning(d, twins)), writes=cone.NO_WRITES)
        self.assertIsNone(c.relevant)
        self.assertIn("positive", c.blocked_by)


class ConeCommandTest(unittest.TestCase):
    def run_main(self, diagram: dict, *args) -> str:
        from csdfbmc.__main__ import main
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        path, twins = os.path.join(d.name, "d.json"), os.path.join(d.name, "t.py")
        with open(path, "w", encoding="utf-8") as f:
            json.dump(fixture.parsed(diagram), f)
        with open(twins, "w", encoding="utf-8") as f:
            f.write("DOMAINS = {}\n")
        out = io.StringIO()
        with redirect_stdout(out), redirect_stderr(io.StringIO()):
            main([path, twins, *args])
        return out.getvalue()

    def test_the_report_says_what_was_left_out(self):
        out = self.run_main(AUDITED, "--evaluator-module", "csdfbmc.python_glue", "--cone")
        self.assertIn("states: 4\n", out)
        self.assertIn("relevant: n\n", out)
        self.assertIn("left out (nothing that decides a step or a property reads it): log, set by start; a -dec-> a (line 5)\n", out)

    def test_the_report_says_why_the_cone_was_not_computed(self):
        diagram = edit(AUDITED, "dec", post="[{'n': n - 1}] ∧ @opaque")
        out = self.run_main(diagram, "--evaluator-module", "csdfbmc.python_glue", "--cone", "--opaque", "true")
        self.assertIn("cone of influence not computed: what @opaque reads is unknown\n", out)


if __name__ == "__main__":
    unittest.main()
