"""The checks module of examples/bmc/wallet_formal.puml, and the constants its
Python atoms name: csdfbmc reads TYPES, GHOSTS and INVARIANTS from it, and
the reference glue, given it as an argument, lets every atom use the rest.

The fairness assumption of the diagram becomes the bound RESENDS on the resend
count: a run cannot bump the fee forever. The deposits are bounded by the
diagram's own 入庫回数, so every total stays finite without a domain for it.
"""
LIMIT = 3          # 移転上限
BASE_FEE = 1       # 出庫 fee before any bump; the check at 出庫 time uses it.
                   # Try fee(RESENDS) in its place in wallet_formal.puml.
RESENDS = 2        # the assume note, as a bound
DEPOSITS = 2       # how many deposits the scenario allows
INITIAL = 4


def fee(再送回数):
    """Each resend bumps the fee by one."""
    return BASE_FEE + 再送回数


TYPES = {
    "数量": range(1, 4),
    "額": [1, 2],
}

# Running totals the diagram does not keep.
GHOSTS = {
    "総入庫": (0, {"入庫": lambda 総入庫, 額: 総入庫 + 額}),
    "総出庫": (0, {"確定": lambda 総出庫, 送信量: 総出庫 + 送信量}),
    "総fee": (0, {"確定": lambda 総fee, 再送回数: 総fee + fee(再送回数)}),
    "戻し済みfee": (0, {"fee戻し": lambda 戻し済みfee, 未戻しfee: 戻し済みfee + 未戻しfee}),
}

INVARIANTS = {
    # A withdrawal takes amount plus fee, a deposit adds the amount as is.
    "残高 = 初期残高 + 総入庫 − 総出庫 − 総fee + 戻し済みfee": lambda 残高, 総入庫, 総出庫, 総fee, 戻し済みfee: 残高 == INITIAL + 総入庫 - 総出庫 - 総fee + 戻し済みfee,
    # Once every fee is back, the total is deposits minus withdrawn amounts.
    "fee を戻し終えたら 残高 = 初期残高 + 総入庫 − 総出庫": lambda 残高, 未戻しfee, 総入庫, 総出庫: 未戻しfee != 0 or 残高 == INITIAL + 総入庫 - 総出庫,
    # The amount in flight is within the limit.
    "送信中の送信量 ≤ 移転上限": lambda 送信量: 送信量 <= LIMIT,
}
