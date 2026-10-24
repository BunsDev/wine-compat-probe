#!/bin/sh
# Run dist/probe.exe through a Proton install (Steam's or GE-Proton) without the Steam client.
# Usage: PROTON=/path/to/proton sh scripts/run-proton.sh
# Proton needs STEAM_COMPAT_DATA_PATH (prefix location) and STEAM_COMPAT_CLIENT_INSTALL_PATH to exist.
set -eu
cd "$(dirname "$0")/.."
PROTON=${PROTON:-}
if [ -z "${PROTON}" ]; then
  echo "set PROTON to a proton script, for example GE-Proton11-11/proton or the proton file inside the Steam Proton install directory" >&2
  exit 2
fi
export STEAM_COMPAT_DATA_PATH=${STEAM_COMPAT_DATA_PATH:-$(pwd)/.proton-data}
export STEAM_COMPAT_CLIENT_INSTALL_PATH=${STEAM_COMPAT_CLIENT_INSTALL_PATH:-$(pwd)/.steam}
export WINEDEBUG=${WINEDEBUG:--all}
mkdir -p "${STEAM_COMPAT_DATA_PATH}" "${STEAM_COMPAT_CLIENT_INSTALL_PATH}" results
OUT="Z:$(pwd)/results"
python3 "${PROTON}" run dist/probe.exe -workarounds none -out "${OUT}/proton-before.json" -text
python3 "${PROTON}" run dist/probe.exe -workarounds all -out "${OUT}/proton.json" -text
echo "wrote results/proton-before.json and results/proton.json"
