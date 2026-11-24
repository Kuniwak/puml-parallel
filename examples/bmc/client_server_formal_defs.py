"""The domains of examples/bmc/client_server_formal.puml, a client that
initializes one MCP server and calls its tools, and the constants its
Python labels name."""
from itertools import combinations

SERVER = "weather"
TOOLS = frozenset({"forecast", "alerts"})
INITIAL = "initial"
STATES = ["initial", "updated"]

TYPES = {
    "Server": [SERVER],
    "ToolSet": [frozenset(c) for n in range(len(TOOLS) + 1) for c in combinations(sorted(TOOLS), n)],
    "ServerState": STATES,
}
