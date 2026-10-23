"""The checks module of examples/bmc/attempts/ATTEMPTS.puml, for its expansion
(csdfpromote -format json examples/bmc/attempts/ATTEMPTS.puml).

The map `attempts` holds, per 試行ID, an instance: its local state, which
only the builtin @at reads, and its local variables, which are all an atom,
a ghost or an invariant sees of it. The fairness assumption of the diagram
becomes the finite set of IDs: a run cannot register fresh attempts forever.
"""
from csdfbmc.values import record

IDS = ["t1", "t2", "t3"]

TYPES = {
    "試行ID": IDS,
    # A transaction to send carries its fee; a replacement also names the
    # attempt it replaces.
    "未署名Tx": record(手数料=[1]),
    "置換Tx": record(置換元=IDS, 手数料=[2, 3]),
}

# The attempts are interchangeable: no clause names one or orders them, so
# --symmetry may store one state per renaming of their IDs.
SYMMETRIC = ["試行ID"]

INVARIANTS = {
    # A replacement pays more than what it replaces, so fees never go down
    # along a chain of replacements; every attempt's fee is at least the base.
    "手数料は 1 以上": lambda attempts: all(r.手数料 >= 1 for r in attempts.values()),
}
