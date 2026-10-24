#!/bin/sh
# Run the full build/test/run/compare pipeline on GitHub Actions and wait for it.
# Requires an authenticated gh. Set PUBLISH=true to commit results back to the branch.
set -eu
cd "$(dirname "$0")/.."
PUBLISH=${PUBLISH:-false}
BRANCH=$(git rev-parse --abbrev-ref HEAD)
echo "e2e: dispatching e2e.yml on ${BRANCH} (publish=${PUBLISH})"
gh workflow run e2e.yml --ref "${BRANCH}" -f publish="${PUBLISH}"
sleep 8
RUN_ID=$(gh run list --workflow e2e.yml --branch "${BRANCH}" --limit 1 --json databaseId --jq '.[0].databaseId')
echo "e2e: watching run ${RUN_ID}"
gh run watch "${RUN_ID}" --exit-status || echo "e2e: run finished with failures; reports may still exist"
echo "e2e: downloading reports and results"
mkdir -p results docs
gh run download "${RUN_ID}" --name reports --dir docs || true
gh run download "${RUN_ID}" --pattern 'results-*' --dir results || true
echo "e2e: done. Read docs/COMPARISON-REPORT.md and docs/FIX-BEFORE-AFTER.md; raw JSON is under results/results-*/"
