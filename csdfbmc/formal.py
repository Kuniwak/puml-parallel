"""What a formal expression may use besides the names of the step: the
helpers over the maps of an expansion and a few pure builtins."""
from __future__ import annotations

from csdfbmc.values import FrozenMap

# The key under which an instance's record holds its local state's name; the
# local variables are the other keys.
RECORD_STATE = "state"

# What a formal expression may call besides the names of the step.
FORMAL_GLOBALS = {
    "__builtins__": {},
    "len": len, "all": all, "any": any, "sum": sum, "min": min, "max": max, "abs": abs,
    "sorted": sorted, "set": set, "frozenset": frozenset, "tuple": tuple, "range": range,
    "FrozenMap": FrozenMap,
    "at": lambda m, i, state: i in m and m[i][RECORD_STATE] == state,
    "exists": lambda m, pred: any(pred(i, r) for i, r in m.items()),
    "count": lambda m, pred: sum(1 for i, r in m.items() if pred(i, r)),
}
