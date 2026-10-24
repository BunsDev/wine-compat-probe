# AGENTS.md

Operating notes for coding agents working in this repository. Humans welcome too.

## The loop

Everything is a plain command with readable output. The canonical cycle is:

1. `make hooks` once per clone. Installs the pre-commit secret check.
2. `make vet test` after every code change. Vet runs twice: once for the host and once with `GOOS=windows`.
3. `make build` to produce `dist/probe.exe` (windows/amd64) and `dist/compare`.
4. Run the binary on the targets you have (`scripts/run-wine.sh`, `scripts/run-proton.sh`, or `probe.exe` on Windows), or run `make e2e` to let GitHub Actions do Windows, Wine, and Proton.
5. `make compare` and `make compare-fix` to regenerate `docs/COMPARISON-REPORT.md` and `docs/FIX-BEFORE-AFTER.md`.
6. Commit with a small, focused message. The pre-commit hook refuses secrets. Prefer `make e2e-publish` when results should land on `main`; it commits through `cmd/publish`, so the commit is signed by GitHub even from a runner.

Nothing here needs a Windows machine for development: the probe cross-compiles, `compare` and its tests run anywhere, and CI supplies the three runtimes.

## Reading CI without a browser

- `gh run list --workflow e2e.yml` lists runs; `gh run view RUN_ID --log` prints every job log.
- Each run job prints its JSON between `=== BEGIN results/NAME.json ===` and `=== END ... ===` markers. The compare job prints both reports between the same style of markers and writes the comparison report to the job summary.
- Artifacts: `probe-bin`, `results-windows`, `results-wine`, `results-proton`, `reports`. `gh run download RUN_ID --name reports --dir docs` fetches the reports.

## Committing without local git (agents in sandboxes)

Two tools exist for sessions that can call the GitHub API but cannot run `git` or sign:

- `cmd/publish` commits a list of files from the working tree to a branch through the GraphQL `createCommitOnBranch` mutation. GitHub signs those commits. CI uses it for results.
- `.github/workflows/materialize.yml` takes a manifest of `path=blobsha` lines (blobs uploaded with `gh api repos/OWNER/REPO/git/blobs`), fetches them into a runner checkout, and commits them with `cmd/publish` using the workflow token. Dispatch it with `gh workflow run materialize.yml -F manifest=@manifest.txt -f message=...`. Verify afterwards that `gh api repos/OWNER/REPO/contents/PATH --jq .sha` equals each uploaded blob sha.

Both paths produce commits authored by whoever owns the token (a person, or `github-actions[bot]`). Put real authorship in the commit body trailers.

## Code conventions

- Go standard library only. No cgo, no external modules, so `go.sum` stays empty and cross-compiling needs only Go.
- Windows-only files carry `//go:build windows`. Shared code (`internal/schema`, `internal/compare`) must build and test on every OS.
- JSON documents are built from `schema.Node` maps rather than tagged structs; `encoding/json` sorts map keys, which is what makes two runs diffable. Do not introduce struct tags for output types.
- Every probe goes through `timed(doc, section, name, fn)`: it records `duration_ms` and converts a panic into a `fail` entry so one bad API call cannot take the run down.
- Status vocabulary is fixed: `ok`, `fail`, `skipped`, `unsupported`. `unsupported` means the export or DLL is absent in this environment, which is information, not an error.
- Volatile fields (`timestamp`, `duration_ms`, `rtt_ms`, `total_ms`, `args`) are excluded by `compare`. If you add a field that legitimately changes run to run, add its leaf name to `volatileLeaves` in `internal/compare/compare.go`.
- Workarounds are opt-in by name through `-workarounds`. Add new ones to `allWorkarounds` in `cmd/probe/main.go`, gate the behaviour on `opts.enabled(...)`, and record `source` and `workaround` on the resulting entry so the before/after report shows the change.
- `go vet` for the Windows target runs with `-unsafeptr=false`. The probe converts raw `uintptr` values returned by Win32 into pointers on purpose; keep that disabled flag in sync between the Makefile and the workflow.

## Adding a probe

1. Pick the section file (`os.go`, `apis.go`, `graphics.go`, `dxgi_vulkan.go`, `files.go`, `network.go`).
2. Wrap the work in `timed(doc, secX, name, func() schema.Node {...})` and return `schema.OK`, `schema.Fail`, `schema.Skipped`, or `schema.Unsupported`.
3. Report values, never identifiers: no hostnames, user names, serials, MACs, IPs, or environment variable values.
4. Add the probe to the inventory table in `docs/TEST-PLAN.md` and, if its output varies by machine rather than by runtime, say so there.
5. Run `make vet test build` and open a pull request; the e2e workflow runs on it.

## Non-goals, enforced

- No anti-cheat interaction of any kind: no EAC, BattlEye, Vanguard, or similar DLLs, drivers, services, processes, or files are loaded, inspected, modified, or bypassed. Do not add probes that touch them.
- No telemetry, no uploads from the probe itself. Results leave the machine only when you commit them.

## Commit and repository rules

- `main` is protected by convention: small commits, meaningful messages, pre-commit hook installed.
- Never commit `.env`, keys, or tokens. The hook blocks the obvious cases; `make secrets-scan` runs gitleaks over the whole tree and `.github/workflows/secrets-scan.yml` runs it on every push.
- CI needs no secrets. The publish and materialize steps use the workflow token with `contents: write` on that job only.
