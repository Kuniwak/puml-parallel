"""The domains of examples/bmc/printer_formal.puml, and the constants its
Python labels name. csdfbmc reads TYPES from it; the reference glue,
given it as an argument, lets every label use the other names."""
LIMIT = 3
WASTE = 1
INITIAL = 3
CAP = 6            # bounds the scenario: refills stop at this many sheets

TYPES = {
    "JobSize": range(1, 4),
    "RefillSize": [1, 2],
}
