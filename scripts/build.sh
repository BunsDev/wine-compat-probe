#!/bin/sh
# Cross-compile probe.exe (windows/amd64) and build compare for the host. Needs only Go.
set -eu
cd "$(dirname "$0")/.."
mkdir -p dist
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/probe.exe ./cmd/probe
go build -trimpath -o dist/compare ./cmd/compare
ls -l dist
