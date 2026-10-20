"""The formal twins of examples/bmc/wallet.puml.

The fairness assumption of the diagram becomes the bound RESENDS on the resend
count: a run cannot bump the fee forever. The deposits are bounded by the
diagram's own 入庫回数, so every total stays finite without a domain for it.
"""
LIMIT = 3          # 移転上限
BASE_FEE = 1       # 出庫 fee before any bump
RESENDS = 2        # the assume note, as a bound
DEPOSITS = 2       # how many deposits the scenario allows
INITIAL = 4

def fee(再送回数):
    """Each resend bumps the fee by one."""
    return BASE_FEE + 再送回数

DOMAINS = {
    "数量": range(1, 4),
    "額": [1, 2],
}

CLAUSES = {
    "残高 = 初期残高": lambda: [{"残高": INITIAL}],
    "未戻しfee = 0": lambda: [{"未戻しfee": 0}],
    "入庫回数 = 0": lambda: [{"入庫回数": 0}],
    "入庫回数 < 入庫上限": lambda 入庫回数: 入庫回数 < DEPOSITS,
    "入庫回数' = 入庫回数 + 1": lambda 入庫回数: [{"入庫回数": 入庫回数 + 1}],
    "入庫回数' = 入庫回数": lambda 入庫回数: [{"入庫回数": 入庫回数}],
    "残高' = 残高 + 額": lambda 残高, 額: [{"残高": 残高 + 額}],
    "未戻しfee' = 未戻しfee": lambda 未戻しfee: [{"未戻しfee": 未戻しfee}],
    "数量 ≤ 移転上限": lambda 数量: 数量 <= LIMIT,
    # The check at 出庫 time. Try fee(RESENDS) in place of BASE_FEE.
    "数量 + 出庫fee ≤ 残高": lambda 数量, 残高: 数量 + BASE_FEE <= 残高,
    "残高' = 残高": lambda 残高: [{"残高": 残高}],
    "数量' = 数量": lambda 数量: [{"数量": 数量}],
    "再送回数' = 0": lambda: [{"再送回数": 0}],
    "再送回数 < 再送上限": lambda 再送回数: 再送回数 < RESENDS,
    "再送回数' = 再送回数 + 1": lambda 再送回数: [{"再送回数": 再送回数 + 1}],
    "残高 − 数量 ≥ 今回の出庫fee": lambda 残高, 数量, 再送回数: 残高 - 数量 >= fee(再送回数),
    "残高' = 残高 − 数量 − 今回の出庫fee": lambda 残高, 数量, 再送回数: [{"残高": 残高 - 数量 - fee(再送回数)}],
    "未戻しfee' = 未戻しfee + 今回の出庫fee": lambda 未戻しfee, 再送回数: [{"未戻しfee": 未戻しfee + fee(再送回数)}],
    "未戻しfee > 0": lambda 未戻しfee: 未戻しfee > 0,
    "残高' = 残高 + 未戻しfee": lambda 残高, 未戻しfee: [{"残高": 残高 + 未戻しfee}],
    "未戻しfee' = 0": lambda: [{"未戻しfee": 0}],
}

# Running totals the diagram does not keep.
GHOSTS = {
    "総入庫": (0, {"入庫": lambda 総入庫, 額: 総入庫 + 額}),
    "総出庫": (0, {"確定": lambda 総出庫, 数量: 総出庫 + 数量}),
    "総fee": (0, {"確定": lambda 総fee, 再送回数: 総fee + fee(再送回数)}),
    "戻し済みfee": (0, {"fee戻し": lambda 戻し済みfee, 未戻しfee: 戻し済みfee + 未戻しfee}),
}

INVARIANTS = {
    # 4 and 5: a withdrawal takes amount plus fee, a deposit adds the amount as is.
    "残高 = 初期残高 + 総入庫 − 総出庫 − 総fee + 戻し済みfee": lambda 残高, 総入庫, 総出庫, 総fee, 戻し済みfee: 残高 == INITIAL + 総入庫 - 総出庫 - 総fee + 戻し済みfee,
    # 6: once every fee is back, the total is deposits minus withdrawn amounts.
    "fee を戻し終えたら 残高 = 初期残高 + 総入庫 − 総出庫": lambda 残高, 未戻しfee, 総入庫, 総出庫: 未戻しfee != 0 or 残高 == INITIAL + 総入庫 - 総出庫,
    # 3: the amount in flight is within the limit.
    "送信中の数量 ≤ 移転上限": lambda 数量: 数量 <= LIMIT,
}
