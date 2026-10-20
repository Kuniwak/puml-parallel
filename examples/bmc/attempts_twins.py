"""The global twins of examples/promote/ATTEMPTS.puml, for its expansion
(csdfpromote -format json examples/promote/ATTEMPTS.puml).

The map `attempts` holds, per 試行ID, the attempt's local state (under
"state") and its local variables; the local clauses have their own twins in
attempt_local_twins.py. The fairness assumption of the diagram becomes the
finite set of IDs: a run cannot register fresh attempts forever.
"""
from csdfbmc.values import FrozenMap, record

IDS = ["t1", "t2", "t3"]

DOMAINS = {
    "試行ID": IDS,
    # A transaction to send carries its fee; a replacement also names the
    # attempt it replaces.
    "未署名Tx": record(手数料=[1]),
    "置換Tx": record(置換元=IDS, 手数料=[2, 3]),
}

CLAUSES = {
    "attempts は空": lambda: [{"attempts": FrozenMap()}],
    "置換Tx の置換元 は〈滞留〉にあり、置換Tx の手数料 > 置換元 の手数料":
        lambda attempts, 置換Tx: 置換Tx.置換元 in attempts
        and attempts[置換Tx.置換元].state == "滞留"
        and 置換Tx.手数料 > attempts[置換Tx.置換元].手数料,
    "attempts のすべてが〈確定〉": lambda attempts: len(attempts) > 0 and all(r.state == "確定" for r in attempts.values()),
}

INVARIANTS = {
    # A replacement pays more than what it replaces, so fees never go down
    # along a chain of replacements; every attempt's fee is at least the base.
    "手数料は 1 以上": lambda attempts: all(r.手数料 >= 1 for r in attempts.values()),
}
