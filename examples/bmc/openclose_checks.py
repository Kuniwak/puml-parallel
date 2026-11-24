"""The checks module of examples/promote/OPENCLOSE.puml, whose diagram has
no clause to write in Python. The map `users` has no bound; the IDs here are
a sample of them, u1 to u3, and u4 is one more, so a walk that has used up
the three reports the bound reached, not a deadlock."""
from csdfbmc.values import sample

IDS = sample(["u1", "u2", "u3"], beyond=["u4"])

TYPES = {
    "UserId": IDS,
}

# No clause tells two users apart by their IDs.
SYMMETRIC = ["UserId"]
