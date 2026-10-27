"""The checks module of examples/bmc/printer_retry_formal.puml, and the
constants its Python atoms name: csdfbmc reads TYPES, GHOSTS and INVARIANTS
from it, and the reference glue, given it as an argument, lets every atom use
the rest.

The fairness assumption of the diagram becomes the bound RETRIES on the retry
count: a run cannot jam forever. The refills are bounded by the diagram's own
補充回数, so every total stays finite without a domain for it.
"""
LIMIT = 3          # 印刷上限
BASE_WASTE = 1     # 印刷 waste before any retry; the check at 印刷 time uses it.
                   # Try waste(RETRIES) in its place in printer_retry_formal.puml.
RETRIES = 2        # the assume note, as a bound
REFILLS = 2        # how many refills the scenario allows
INITIAL = 4


def waste(再試行回数):
    """Each retry jams one more sheet."""
    return BASE_WASTE + 再試行回数


TYPES = {
    "JobSize": range(1, 4),
    "RefillSize": [1, 2],
}

# Running totals the diagram does not keep.
GHOSTS = {
    "総補充": (0, {"補充": lambda 総補充, 補充数: 総補充 + 補充数}),
    "総印刷": (0, {"完了": lambda 総印刷, 印刷枚数: 総印刷 + 印刷枚数}),
    "総損紙": (0, {"完了": lambda 総損紙, 再試行回数: 総損紙 + waste(再試行回数)}),
    "回収済み損紙": (0, {"損紙回収": lambda 回収済み損紙, 未回収損紙: 回収済み損紙 + 未回収損紙}),
}

INVARIANTS = {
    # A job takes its sheets plus the jammed ones, a refill adds its sheets as is.
    "用紙 = 初期用紙 + 総補充 − 総印刷 − 総損紙 + 回収済み損紙": lambda 用紙, 総補充, 総印刷, 総損紙, 回収済み損紙: 用紙 == INITIAL + 総補充 - 総印刷 - 総損紙 + 回収済み損紙,
    # Once every jam is cleared and re-fed, the tray is refills minus printed sheets.
    "損紙を回収し終えたら 用紙 = 初期用紙 + 総補充 − 総印刷": lambda 用紙, 未回収損紙, 総補充, 総印刷: 未回収損紙 != 0 or 用紙 == INITIAL + 総補充 - 総印刷,
    # The job in progress is within the limit.
    "印刷中の印刷枚数 ≤ 印刷上限": lambda 印刷枚数: 印刷枚数 <= LIMIT,
}
