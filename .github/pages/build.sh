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

for f in examples/valid/vending_machine.puml examples/valid/client_server.puml \
         examples/compose/user_vending_machine.puml \
         examples/promote/ATTEMPTS.puml examples/promote/MEMBERS.puml \
         examples/promote/MODES.puml examples/promote/RECEIPTS.puml; do
  "$bin/csdfreplweb" "$f" > "$site/$(basename "$f" .puml).html"
done

# The vending machine with stock and change, once per language: its drawing
# decorated, its clauses for people over the formal ones, and suggestions.
bmc=examples/bmc
for lang in en ja; do
  case "$lang" in
    en) formal="$bmc/vending_stock_formal" ;;
    ja) formal="$bmc/vending_stock_ja_formal" ;;
  esac
  "$bin/csdfdecorate" -lang "$lang" "$bmc/vending_stock_$lang.puml" | plantuml -pipe -tsvg > "$bin/vending_stock.svg"
  "$bin/csdfreplweb" -puml-exec "cat $bin/vending_stock.svg" \
    -prose "$bmc/vending_stock_$lang.puml" \
    -checks "${formal}_defs.py" \
    -evaluator-module "csdfbmc.python_glue ${formal}_defs.py" \
    "$formal.puml" > "$site/vending_stock_$lang.html"
done

cp "$(dirname "$0")/index.html" "$site/index.html"
