"""The formal twins of examples/bmc/withdraw.puml.

A guard twin decides; a post twin returns the successor values of the
variables it constrains, so a deterministic clause is one assignment. Only the
event parameters need a domain.
"""
LIMIT = 3
FEE = 1
INITIAL = 3
CAP = 6            # bounds the scenario: deposits stop at this balance

DOMAINS = {
    "数量": range(1, 4),
    "額": [1, 2],
}

CLAUSES = {
    "残高 = 初期残高": lambda: [{"残高": INITIAL}],
    "数量 ≤ 移転上限": lambda 数量: 数量 <= LIMIT,
    # "The balance covers the amount." Compare with the completion guard
    # below, which also needs the fee; try 残高 >= 数量 + FEE here.
    "残高で払える": lambda 残高, 数量: 残高 >= 数量,
    "残高' = 残高": lambda 残高: [{"残高": 残高}],
    "数量' = 数量": lambda 数量: [{"数量": 数量}],
    "残高 − 数量 ≥ 手数料": lambda 残高, 数量: 残高 - 数量 >= FEE,
    "残高' = 残高 − 数量 − 手数料": lambda 残高, 数量: [{"残高": 残高 - 数量 - FEE}],
    "残高 + 額 ≤ 残高上限": lambda 残高, 額: 残高 + 額 <= CAP,
    "残高' = 残高 + 額": lambda 残高, 額: [{"残高": 残高 + 額}],
}
