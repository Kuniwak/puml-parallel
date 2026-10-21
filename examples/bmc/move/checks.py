"""The checks module of examples/bmc/move/MOVE.puml, for its expansion
(csdfpromote -format json examples/bmc/move/MOVE.puml)."""

DOMAINS = {
    "出荷ID": ["s1", "s2"],
    "拠点ID": ["l1", "l2"],
    "from": ["l1", "l2"],
    "to": ["l1", "l2"],
    "計画数量": [1, 2],
    "数量": [1, 2],
}

INVARIANTS = {
    # A move takes out of one location what it puts into another, so the
    # stock of the open locations is what they opened with.
    "在庫は移動で増減しない": lambda locations: sum(r.在庫 for r in locations.values()) == 2 * len(locations),
}
