"""The checks module of MODES.puml here: the types of MEMBERS.puml's, and
the reviews. The IDs are samples, each with one more beyond."""
from csdfbmc.values import sample

MEMBER_IDS = sample(["m1", "m2"], beyond=["m3"])
REVIEW_IDS = sample(["r1", "r2"], beyond=["r3"])

TYPES = {
    "MemberId": MEMBER_IDS,
    "ReviewId": REVIEW_IDS,
    "SuspendReason": ["延滞", "規約違反"],
}
