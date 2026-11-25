"""The checks module of RECEIPTS.puml here, and the constants its clauses
name: a quantity is received in whole boxes of BOX, and each receipt ID
has its day of receipt in RECEIVED_ON."""
from csdfbmc.values import sample

BOX = 6
RECEIPT_IDS = sample(["i1", "i2", "i3"], beyond=["i4"])
RECEIVED_ON = {"i1": "d1", "i2": "d1", "i3": "d2", "i4": "d2"}

TYPES = {
    "ReceiptId": RECEIPT_IDS,
    "Date": ["d1", "d2"],
    "Quantity": [3, 6, 12],
}
