"""The domains of examples/bmc/vending_machine_formal.puml, the vending
machine of examples/valid/vending_machine.puml, and the constants its
Python labels name."""
from itertools import combinations

PRODUCTS = ["cola", "tea", "water"]
COINS = [50, 100]
INITIAL = frozenset(PRODUCTS)


def subsets(xs):
    return [frozenset(c) for n in range(len(xs) + 1) for c in combinations(xs, n)]


TYPES = {
    "Product": PRODUCTS,
    "Coin": COINS,
    "ProductSet": subsets(PRODUCTS),
    "CoinSet": subsets(COINS),
}

# What a customer may come to want: some products, at least one.
WANTS = [s for s in TYPES["ProductSet"] if s]
