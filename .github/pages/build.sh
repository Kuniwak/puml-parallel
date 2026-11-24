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
mkdir -p "$site"

for f in examples/promote/ATTEMPTS.puml examples/promote/MEMBERS.puml \
         examples/promote/MODES.puml examples/promote/RECEIPTS.puml; do
  "$bin/csdfreplweb" "$f" > "$site/$(basename "$f" .puml).html"
done

# checked PAGE DIAGRAM CHECKS writes PAGE.html of the promoted DIAGRAM,
# whose clauses are in Python, suggesting values with the checks CHECKS.
checked() {
  "$bin/csdfreplweb" -checks "$3" -evaluator-module "csdfbmc.python_glue $3" "$2" > "$site/$1.html"
}

checked OPENCLOSE examples/promote/OPENCLOSE.puml examples/bmc/openclose_checks.py

# twin PAGE LANG PROSE FORMAL DEFS writes PAGE.html of the diagram FORMAL,
# whose clauses are in Python: its drawing is PROSE decorated in LANG, its
# clauses for people are PROSE's over the formal ones, and the page suggests
# values with the checks DEFS.
twin() {
  "$bin/csdfdecorate" -lang "$2" "$3" | plantuml -pipe -tsvg > "$bin/drawing.svg"
  "$bin/csdfreplweb" -puml-exec "cat $bin/drawing.svg" -prose "$3" \
    -checks "$5" -evaluator-module "csdfbmc.python_glue $5" \
    "$4" > "$site/$1.html"
}

bmc=examples/bmc
twin vending_stock_en en "$bmc/vending_stock_en.puml" "$bmc/vending_stock_formal.puml" "$bmc/vending_stock_formal_defs.py"
twin vending_machine en examples/valid/vending_machine.puml "$bmc/vending_machine_formal.puml" "$bmc/vending_machine_formal_defs.py"
twin user_vending_machine en examples/compose/user_vending_machine.puml "$bmc/user_vending_machine_formal.puml" "$bmc/vending_machine_formal_defs.py"
twin client_server en examples/valid/client_server.puml "$bmc/client_server_formal.puml" "$bmc/client_server_formal_defs.py"
twin vending_stock_ja ja "$bmc/vending_stock_ja.puml" "$bmc/vending_stock_ja_formal.puml" "$bmc/vending_stock_ja_formal_defs.py"

cp "$(dirname "$0")/index.html" "$site/index.html"
