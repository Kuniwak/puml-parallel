"""The domains of examples/bmc/vending_stock_formal.puml, and the constants
and helpers its Python labels name. csdfbmc reads TYPES from it; the
reference glue, given it as an argument, lets every label use the other
names. Change is paid greedily, 100-yen coins first, which is exact for
these two coins."""
from csdfbmc.values import record

PRICE = {"cola": 100, "tea": 150}
CAPACITY = 2
FULL = {"cola": CAPACITY, "tea": CAPACITY}
KEY = {50: "yen50", 100: "yen100"}
MAX_COINS = 4
INITIAL_CASH = {"yen50": 1, "yen100": 0}
LIMIT = 200


def plus(m, key, n):
    return {k: v + n if k == key else v for k, v in m.items()}


def hundreds_paid(cash, amount):
    return min(cash["yen100"], amount // 100)


def can_pay(cash, amount):
    return amount - 100 * hundreds_paid(cash, amount) <= 50 * cash["yen50"]


def pay(cash, amount):
    return {"yen100": cash["yen100"] - hundreds_paid(cash, amount),
            "yen50": cash["yen50"] - (amount - 100 * hundreds_paid(cash, amount)) // 50}


TYPES = {
    "Product": list(PRICE),
    "Stock": record(cola=range(CAPACITY + 1), tea=range(CAPACITY + 1)),
    "Coin": list(KEY),
    "Cash": record(yen50=range(MAX_COINS + 1), yen100=range(MAX_COINS + 1)),
    "Count": [1, 2],
    "Amount": range(0, 100 * MAX_COINS + 1, 50),
}
