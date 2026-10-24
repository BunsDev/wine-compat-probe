#!/bin/sh
# Run dist/probe.exe under a local Wine and write results/wine-before.json and results/wine.json.
# Override the binary with WINE=/path/to/wine. Output paths use the Z: drive, which Wine maps to /.
set -eu
cd "$(dirname "$0")/.."
WINE=${WINE:-wine}
export WINEDEBUG=${WINEDEBUG:--all}
mkdir -p results
OUT="Z:$(pwd)/results"
"${WINE}" dist/probe.exe -workarounds none -out "${OUT}/wine-before.json" -text
"${WINE}" dist/probe.exe -workarounds all -out "${OUT}/wine.json" -text
echo "wrote results/wine-before.json and results/wine.json"
