# Setup

## Prerequisites

| Task | Needs |
| --- | --- |
| Build, test, compare (any OS) | Go 1.22 or newer. Nothing else: no cgo, no C toolchain, no external modules. |
| Run on Windows | Windows 10 or newer, x64. SmartScreen may warn about an unsigned binary; choose Run anyway or unblock the file. |
| Run on Wine | Wine 8 or newer (9.x tested). Linux: `sudo apt install wine wine64`. macOS: `brew install --cask wine-stable`. |
| Run on Proton | Linux with Python 3 and either a Steam Proton install or a GE-Proton tarball. The Steam client is not required. |
| CI | A GitHub repository with Actions enabled. No secrets. |
| Secret check | `gitleaks` (optional but recommended): `brew install gitleaks` or download a release. The hook falls back to a pattern scan without it. |

## Clone and prepare

```sh
git clone git@github.com:BunsDev/wine-compat-probe.git
cd wine-compat-probe
make hooks        # core.hooksPath=.githooks, pre-commit secret check
make vet test     # confirms the toolchain works
make build        # dist/probe.exe, dist/compare
```

Commits made through the GitHub API do not carry the executable bit, so `make hooks` also runs `chmod +x .githooks/pre-commit`. The scripts are invoked as `sh scripts/NAME.sh` for the same reason.

## Running on Windows

```powershell
New-Item -ItemType Directory -Force results | Out-Null
.\dist\probe.exe -workarounds none -out results\windows-before.json -text
.\dist\probe.exe -workarounds all  -out results\windows.json -text
```

## Running on Wine

```sh
export WINEPREFIX=$HOME/.wine-probe   # optional: a fresh prefix
wineboot -u
sh scripts/run-wine.sh                # writes results/wine-before.json and results/wine.json
```

The script passes output paths as `Z:/absolute/path`, because Wine maps the `Z:` drive to `/`. Headless machines need `xvfb-run -a` in front of the `wine` call, or the display and D3D probes will fail; set `WINE="xvfb-run -a wine"` is not supported by the script, so wrap the script itself: `xvfb-run -a sh scripts/run-wine.sh`.

## Running on Proton

Standalone (no Steam client):

```sh
curl -fsSL -o proton.tar.gz https://github.com/GloriousEggroll/proton-ge-custom/releases/download/GE-Proton11-11/GE-Proton11-11-x86_64.tar.gz
mkdir -p proton && tar -xzf proton.tar.gz -C proton
PROTON=proton/GE-Proton11-11/proton sh scripts/run-proton.sh
```

The script sets `STEAM_COMPAT_DATA_PATH` (prefix location) and `STEAM_COMPAT_CLIENT_INSTALL_PATH`, both of which Proton requires to exist. Override them to reuse a prefix.

Through the Steam client: add `dist/probe.exe` as a non-Steam game, set a Proton version under Compatibility, and put `-out Z:/tmp/proton.json -text` in Launch Options. Launch it once; the JSON appears in `/tmp/proton.json`.

## Comparing

```sh
make compare       # docs/COMPARISON-REPORT.md
make compare-fix   # docs/FIX-BEFORE-AFTER.md
go run ./cmd/compare -strict a=results/windows.json b=results/wine.json   # exit 3 if any status differs
```

## GitHub Actions

`.github/workflows/e2e.yml` runs on pushes to `main`, on pull requests, and on manual dispatch. Jobs: secrets scan, build (vet, test, cross-compile), run on Windows, run on Wine, run on Proton (`continue-on-error`), compare. Artifacts hold the binary, raw results, and reports. The compare job also writes the comparison report to the job summary.

```sh
make e2e           # dispatch, wait, download reports and results
make e2e-publish   # same, plus commit results and reports to the branch
```

The publish step uses the default workflow token with `contents: write` on that job only, and commits through the GraphQL API so the commit is signed by GitHub. Pushes made by that commit do not retrigger the workflow because the results and report paths are in `paths-ignore`.

## Local secret scanning

```sh
make secrets-scan  # gitleaks over the whole tree
```
