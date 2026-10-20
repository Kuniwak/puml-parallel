"""The twins of the local clauses of examples/promote/local/ATTEMPT.puml.

A local twin reads the event's parameters and the instance's own variables,
and a local post twin returns the instance's variables after the step.
"""
DOMAINS = {}

CLAUSES = {
    "手数料' = 未署名Tx の手数料": lambda 未署名Tx: [{"手数料": 未署名Tx.手数料}],
    "手数料' = 置換Tx の手数料": lambda 置換Tx: [{"手数料": 置換Tx.手数料}],
    "手数料' = 手数料": lambda 手数料: [{"手数料": 手数料}],
}
