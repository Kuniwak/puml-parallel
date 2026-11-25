"""The checks module of MEMBERS.puml here. The IDs are a sample of the
members, m1 to m3, and m4 is one more."""
from csdfbmc.values import sample

MEMBER_IDS = sample(["m1", "m2", "m3"], beyond=["m4"])

TYPES = {
    "MemberId": MEMBER_IDS,
    "SuspendReason": ["延滞", "規約違反"],
}

SYMMETRIC = ["MemberId"]
