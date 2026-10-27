"""The checks module of examples/bmc/attempts/ATTEMPTS.puml, for its expansion
(csdfpromote -format json examples/bmc/attempts/ATTEMPTS.puml).

The map `attempts` holds, per 試行ID, an instance: its local state, which
only the builtin @at reads, and its local variables, which are all an atom,
a ghost or an invariant sees of it. The fairness assumption of the diagram
becomes the finite set of IDs: a run cannot register fresh attempts forever.
The set is a sample of the IDs, so a state where the walk stops only because
every ID is used up is told from a deadlock.
"""
from csdfbmc.values import record, sample

# The IDs are a sample of the attempts a run may register, and t4 is one
# more: once all three are used up the walk tries t4, which lets a new
# attempt register, and reports the bound reached, not a deadlock.
IDS = sample(["t1", "t2", "t3"], beyond=["t4"])

TYPES = {
    "AttemptId": IDS,
    # A new job carries its priority; a resubmission also names the
    # attempt it replaces.
    "NewJob": record(優先度=[1]),
    "Resubmission": record(置換元=IDS, 優先度=[2, 3]),
}

# No clause tells two attempts apart by their IDs, so --symmetry may store
# a state by one of its renamings of them.
SYMMETRIC = ["AttemptId"]

INVARIANTS = {
    # A resubmission asks more than what it replaces, so priorities never go
    # down along a chain of them; every attempt's priority is at least the base.
    "優先度は 1 以上": lambda attempts: all(r.優先度 >= 1 for r in attempts.values()),
}
