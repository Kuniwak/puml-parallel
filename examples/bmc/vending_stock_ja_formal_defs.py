"""The domains of examples/bmc/vending_stock_ja_formal.puml, the vending
machine of vending_stock_formal.puml with its state variables named in
Japanese, and its checks. The constants, the helpers and the types are
those of vending_stock_formal_defs.py; the ghosts and the invariants are
its, written with the Japanese names, and hold and break as those do."""
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


# The ghosts of vending_stock_formal_defs.py: 売上 is sold, 販売本数 sold_units,
# 補充本数 restocked_units, 回収額 collected, 釣り銭補充額 refilled and 預かり額 owed.
GHOSTS = {
    "売上": (0, {"choose": lambda 売上, product: 売上 + PRICE[product]}),
    "販売本数": (0, {"choose": lambda 販売本数: 販売本数 + 1}),
    "補充本数": (0, {"restock": lambda 補充本数, 在庫: 補充本数 + units(FULL) - units(在庫)}),
    "回収額": (0, {"collect": lambda 回収額, 金庫: 回収額 + 100 * 金庫["yen100"]}),
    "釣り銭補充額": (0, {"refillChange": lambda 釣り銭補充額, count: 釣り銭補充額 + 50 * count}),
    "預かり額": (0, {
        "insert": lambda 預かり額, coin: 預かり額 + coin,
        "cancel": lambda 預かり額: 0,
        "choose": lambda 預かり額, product: 預かり額 - PRICE[product],
        "returnChange": lambda 預かり額: 0,
    }),
}

INVARIANTS = {
    "金庫 = 初期の釣り銭 + 釣り銭補充額 + 売上 - 回収額 + 預かり額":
        lambda 金庫, 釣り銭補充額, 売上, 回収額, 預かり額:
            value(金庫) == value(INITIAL_CASH) + 釣り銭補充額 + 売上 - 回収額 + 預かり額,
    "在庫 = 満杯 + 補充本数 - 販売本数":
        lambda 在庫, 補充本数, 販売本数: units(在庫) == units(FULL) + 補充本数 - 販売本数,
    "お釣りは払える": lambda 金庫, お釣り: can_pay(金庫, お釣り),
    # Broken on purpose, as in vending_stock_formal_defs.py.
    "100 円玉も預かり額もなければ、回収額 = 売上":
        lambda 金庫, 回収額, 売上, 預かり額: 金庫["yen100"] != 0 or 預かり額 != 0 or 回収額 == 売上,
}
