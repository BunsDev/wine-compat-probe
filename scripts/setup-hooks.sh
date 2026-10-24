#!/bin/sh
# Point git at the repo-local hooks so the secret check runs on every commit.
set -eu
cd "$(dirname "$0")/.."
chmod +x .githooks/pre-commit
git config core.hooksPath .githooks
echo "hooks: core.hooksPath=.githooks (pre-commit secret check active)"
if command -v gitleaks >/dev/null 2>&1; then
  echo "hooks: gitleaks found: $(gitleaks version)"
else
  echo "hooks: gitleaks not found; the hook falls back to its built-in pattern scan. Install: brew install gitleaks"
fi
