"""The domains of examples/bmc/vending_stock_formal.puml, and the constants
and helpers its Python labels name. csdfbmc reads TYPES from it; the
reference glue, given it as an argument, lets every label use the other
names. It is also the checks module: GHOSTS total the events and
INVARIANTS relate the totals to the coins in the machine. Change is paid greedily, 100-yen coins first, which is exact for
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


def value(cash):
    return 50 * cash["yen50"] + 100 * cash["yen100"]


def units(stock):
    return sum(stock.values())


# Running totals the diagram does not keep. "owed" is the customer's money
# in the machine: the credit while paying, then the change until it is paid.
GHOSTS = {
    "sold": (0, {"choose": lambda sold, product: sold + PRICE[product]}),
    "sold_units": (0, {"choose": lambda sold_units: sold_units + 1}),
    "restocked_units": (0, {"restock": lambda restocked_units, stock: restocked_units + units(FULL) - units(stock)}),
    "collected": (0, {"collect": lambda collected, cash: collected + 100 * cash["yen100"]}),
    "refilled": (0, {"refillChange": lambda refilled, count: refilled + 50 * count}),
    "owed": (0, {
        "insert": lambda owed, coin: owed + coin,
        "cancel": lambda owed: 0,
        "choose": lambda owed, product: owed - PRICE[product],
        "returnChange": lambda owed: 0,
    }),
}

INVARIANTS = {
    # No coin appears or disappears: what is in the machine is the initial
    # change, the refills and the sales, less what was collected, plus what
    # the customer has put in and not yet got back.
    "cash = initial + refilled + sold - collected + owed":
        lambda cash, refilled, sold, collected, owed: value(cash) == value(INITIAL_CASH) + refilled + sold - collected + owed,
    # Every product that left was sold, and every one sold was stocked.
    "stock = full + restocked - sold":
        lambda stock, restocked_units, sold_units: units(stock) == units(FULL) + restocked_units - sold_units,
    # The machine never promises change it cannot pay.
    "the change owed can be paid": lambda cash, change: can_pay(cash, change),
    # Broken on purpose: one may expect collecting the 100-yen coins to
    # collect the sales, but a product paid in 50-yen coins leaves its price
    # among the change, and change paid in 100-yen coins takes from the
    # sales. The first invariant is the property that holds.
    "collected = sold, once no 100-yen coin and no customer's money is left":
        lambda cash, collected, sold, owed: cash["yen100"] != 0 or owed != 0 or collected == sold,
}
