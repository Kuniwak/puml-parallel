#!/bin/sh
# Writes the examples' web animator pages and their index into the
# directory given, from the root of the work tree. Needs go, plantuml and
# Graphviz on the PATH. index.html, next to this script, links to every
# page written here, each once in English and once in Japanese.
set -eu

site="$1"
bin="$(mktemp -d)"
trap 'rm -rf "$bin"' EXIT
go build -o "$bin/csdfreplweb" ./tools/csdfreplweb
go build -o "$bin/csdfdecorate" ./tools/csdfdecorate
go build -o "$bin/csdfpromote" ./tools/csdfpromote
go build -o "$bin/csdfparse" ./tools/csdfparse
go build -o "$bin/csdfbmc" ./tools/csdfbmc
mkdir -p "$site"

# shapes CHECKS writes into $bin/shapes.json what the clauses of the plain
# diagram on standard input and the TYPES of the checks CHECKS say of the
# values of its types.
shapes() {
  "$bin/csdfparse" > "$bin/diagram.json"
  "$bin/csdfbmc" shapes "$bin/diagram.json" "$1" > "$bin/shapes.json"
}

# checked PAGE DIAGRAM CHECKS [PROSE] writes PAGE.html of the promoted
# DIAGRAM, whose clauses are in Python, suggesting values with the checks
# CHECKS, and showing PROSE's clauses for people over the formal ones.
checked() {
  "$bin/csdfpromote" "$2" | shapes "$3"
  "$bin/csdfreplweb" ${4:+-prose "$4"} -shapes "$bin/shapes.json" -checks "$3" -evaluator-module "csdfbmc.python_glue $3" "$2" > "$site/$1.html"
}

checked ATTEMPTS examples/bmc/attempts/ATTEMPTS.puml examples/bmc/attempts/checks.py examples/promote/ATTEMPTS.puml
checked MEMBERS examples/bmc/promote/MEMBERS.puml examples/bmc/promote/members_checks.py examples/promote/MEMBERS.puml
checked MODES examples/bmc/promote/MODES.puml examples/bmc/promote/modes_checks.py examples/promote/MODES.puml
checked RECEIPTS examples/bmc/promote/RECEIPTS.puml examples/bmc/promote/receipts_checks.py examples/promote/RECEIPTS.puml
checked OPENCLOSE examples/promote/OPENCLOSE.puml examples/bmc/openclose_checks.py

# twin PAGE LANG PROSE FORMAL DEFS writes PAGE.html of the diagram FORMAL,
# whose clauses are in Python: its drawing is PROSE decorated in LANG, its
# clauses for people are PROSE's over the formal ones, and the page suggests
# values with the checks DEFS.
twin() {
  "$bin/csdfdecorate" -lang "$2" "$3" | plantuml -pipe -tsvg > "$bin/drawing.svg"
  shapes "$5" < "$4"
  "$bin/csdfreplweb" -puml-exec "cat $bin/drawing.svg" -prose "$3" \
    -shapes "$bin/shapes.json" -checks "$5" -evaluator-module "csdfbmc.python_glue $5" \
    "$4" > "$site/$1.html"
}

bmc=examples/bmc
twin vending_stock_en en "$bmc/vending_stock_en.puml" "$bmc/vending_stock_formal.puml" "$bmc/vending_stock_formal_defs.py"
twin vending_machine en examples/valid/vending_machine.puml "$bmc/vending_machine_formal.puml" "$bmc/vending_machine_formal_defs.py"
twin user_vending_machine en examples/compose/user_vending_machine.puml "$bmc/user_vending_machine_formal.puml" "$bmc/vending_machine_formal_defs.py"
twin client_server en examples/valid/client_server.puml "$bmc/client_server_formal.puml" "$bmc/client_server_formal_defs.py"
twin vending_stock_ja ja "$bmc/vending_stock_ja.puml" "$bmc/vending_stock_ja_formal.puml" "$bmc/vending_stock_ja_formal_defs.py"

cp "$(dirname "$0")/index.html" "$site/index.html"
