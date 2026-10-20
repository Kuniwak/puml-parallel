"""The domains of examples/bmc/withdraw_formal.puml, and the constants its
Python labels name. csdfbmc reads DOMAINS from it; the reference glue,
given it as an argument, lets every label use the other names."""
LIMIT = 3
FEE = 1
INITIAL = 3
CAP = 6            # bounds the scenario: deposits stop at this balance

DOMAINS = {
    "数量": range(1, 4),
    "額": [1, 2],
}
