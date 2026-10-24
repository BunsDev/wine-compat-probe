# Wine Compatibility Probe

One small Windows binary that reports what the platform underneath it looks like, as sorted JSON, so the same `probe.exe` can run on Windows, Wine, and Steam Proton and the outputs can be diffed line by line.

Hackathon prototype. Scope: OS version, Windows API availability, graphics capabilities (display modes, DXGI adapters, D3D11, D3D12, Vulkan), file-system behaviour, and basic networking.

**Hard boundary:** this project never tests, modifies, or bypasses Easy Anti-Cheat or any other anti-cheat system. It loads only public system DLLs, calls documented APIs, and writes a JSON file. See [docs/LIMITATIONS.md](docs/LIMITATIONS.md).

## What is in the box

| Path | Purpose |
| --- | --- |
| `cmd/probe` | The Windows application. Go, standard library only, cross-compiles from any OS. |
| `cmd/compare` | Turns N result files into a Markdown report: status matrix plus every differing value. |
| `cmd/publish` | Commits files through the GitHub GraphQL API so CI commits come out signed by GitHub. |
| `.github/workflows/e2e.yml` | Build once, run on Windows, Wine (Ubuntu), and Proton (GE-Proton standalone), compare, optionally publish. |
| `.githooks/pre-commit` | Blocks secrets at commit time (gitleaks, with a built-in pattern fallback). |
| `docs/TEST-PLAN.md` | Repeatable test plan. |
| `docs/COMPARISON-REPORT.md` | Windows vs Wine vs Proton report (generated). |
| `docs/FIX-VERSION-DETECTION.md` | The demonstrated compatibility fix, with before/after results in `docs/FIX-BEFORE-AFTER.md`. |
| `docs/SETUP.md`, `docs/LIMITATIONS.md`, `docs/SCHEMA.md` | Setup, known limitations, output schema. |
| `AGENTS.md` | How an agent runs the whole loop: build, test, compare, commit. |

## Quick start

```sh
make hooks        # install the pre-commit secret check (once per clone)
make vet test     # static checks and unit tests; works on macOS, Linux, Windows
make build        # dist/probe.exe for windows/amd64, no C toolchain needed
```

Run the same binary on each target and collect the JSON:

```sh
# Windows (PowerShell)
.\dist\probe.exe -out results\windows.json -text

# Wine (Linux or macOS). Z: is the Wine drive mapped to /.
sh scripts/run-wine.sh

# Proton (Linux). Point PROTON at a proton script; no Steam client needed.
PROTON=/path/to/GE-Proton11-11/proton sh scripts/run-proton.sh
```

Compare:

```sh
make compare      # docs/COMPARISON-REPORT.md  (windows vs wine vs proton)
make compare-fix  # docs/FIX-BEFORE-AFTER.md   (workarounds off vs on, per environment)
```

Or let GitHub Actions do all of it:

```sh
make e2e          # dispatch the workflow, wait, download reports and raw results
make e2e-publish  # same, and commit results + reports back to main (GitHub-signed)
```

## probe.exe flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-out FILE` | stdout | Write the JSON document here. |
| `-workarounds all,none,LIST` | `all` | Compatibility workarounds to apply: `rtl-version`, `d3d-warp-fallback`, `tcp-reachability`. |
| `-no-network` | off | Mark network probes as skipped instead of running them. |
| `-text` | off | Also print one line per probe to stderr. |

Exit code 0 means the report was written; probe failures are data, not errors. Exit 2 is a flag error, exit 1 means the report could not be written.

## Output, in one paragraph

A document has `meta` (schema, tool version, timestamp, workarounds, args), `summary` (status counts), and `sections` (`runtime`, `os`, `apis`, `graphics`, `files`, `network`). Every probe entry carries a `status` of `ok`, `fail`, `skipped`, or `unsupported`, plus `value`, `error`, `note`, `source`, and `duration_ms` as applicable. Keys are emitted sorted, so two runs diff cleanly; `compare` ignores the volatile leaves (`timestamp`, `duration_ms`, `rtt_ms`, `total_ms`, `args`). Details in [docs/SCHEMA.md](docs/SCHEMA.md).

## Privacy

The probe records no hostnames, user names, serial numbers, MAC addresses, or IP addresses. Temp paths are reported with the account segment replaced by `[user]`; environment variables are reported as present or absent, never by value.

## License

MIT. Copyright (c) 2026 Val Alexander.
