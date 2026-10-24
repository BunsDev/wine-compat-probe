# Test plan

A repeatable procedure for comparing one `probe.exe` build across Windows, Wine, and Steam Proton.

## 1. Objective

Detect where the three runtimes disagree on: reported OS version, availability of Windows API exports, graphics capabilities, file-system behaviour, and basic networking. Every difference is classified as either environmental (the machine differs) or behavioural (the runtime differs). Behavioural differences are the findings.

## 2. Environments

| Label | Runtime | Where it runs in CI | Notes |
| --- | --- | --- | --- |
| `windows` | Windows Server 2022/2025 (`windows-latest`) | GitHub-hosted runner | No GPU; D3D11 hardware device fails, WARP succeeds. |
| `wine` | Wine 9.x from Ubuntu 24.04 packages | `ubuntu-latest` + Xvfb + Mesa (llvmpipe, lavapipe) | Fresh 64-bit prefix created with `wineboot -u`. |
| `proton` | GE-Proton11-11 run standalone via its `proton` script | `ubuntu-latest` + Xvfb + Mesa | No Steam client and no pressure-vessel container; see LIMITATIONS. |

Optional manual environments (same binary, same procedure): a Windows 10/11 desktop with a real GPU, Wine on macOS, Steam-launched Proton on a Linux desktop or Steam Deck (add `probe.exe` as a non-Steam game, set the compatibility tool, launch with `-out Z:/tmp/proton.json`).

## 3. Preconditions

- The same `probe.exe` (same SHA-256, printed by the build job) is used in every environment.
- Network egress to `example.com:443` and ICMP to `1.1.1.1` is allowed, or run with `-no-network` everywhere.
- A display or Xvfb is available for the Wine and Proton runs, otherwise display and D3D probes report `fail`, which is a valid but less interesting result.

## 4. Procedure

1. Build: `make vet test build`. Record `sha256sum dist/probe.exe`.
2. For each environment, run twice:
   - `probe.exe -workarounds none -out results/ENV-before.json -text`
   - `probe.exe -workarounds all -out results/ENV.json -text`
3. Collect the six files under `results/`.
4. Generate the cross-environment report: `make compare` (writes `docs/COMPARISON-REPORT.md`).
5. Generate the before/after report: `make compare-fix` (writes `docs/FIX-BEFORE-AFTER.md`).
6. Review the status matrix. Any row marked **yes** in the Differs column is a candidate finding. Classify it with the table in section 6.
7. Commit results and reports (`make e2e-publish` does steps 1 to 5 on GitHub Actions and commits with a GitHub-signed commit).

The whole procedure is what `.github/workflows/e2e.yml` executes.

## 5. Probe inventory

| Section | Probe | What it checks | Typically differs by |
| --- | --- | --- | --- |
| runtime | `wine_version`, `wine_build_id`, `wine_host` | `ntdll` Wine exports; absent on Windows | runtime |
| runtime | `env_hints`, `classification` | Presence of Steam/Proton/Wine environment variables; final label windows/wine/proton | runtime |
| os | `rtl_get_version`, `get_version_ex`, `version` | Kernel-reported version vs the compatibility-shimmed version; `version` is the canonical field and depends on the `rtl-version` workaround | runtime (the demonstrated fix) |
| os | `registry_current_version` | ProductName, DisplayVersion, CurrentBuild and friends | runtime |
| os | `native_system_info`, `wow64_machines` | Architecture, processor count, native machine type | machine |
| os | `system_directory`, `windows_directory`, `logical_drives`, `user_locale`, `system_dpi` | Path casing, drive letters (Wine exposes Z:), locale | runtime |
| apis | one row per `dll!Export` plus `_counts` | Export presence for 70 well-known APIs across kernel32, ntdll, user32, dxgi, d3d11/12, vulkan-1, winhttp, iphlpapi, bcrypt, combase, and more | runtime |
| graphics | `display_devices`, `current_display_mode` | Adapter enumeration and current mode | machine and display |
| graphics | `d3d11_device`, `dxgi_adapters`, `d3d12_device`, `vulkan` | Device creation, feature level, adapter description and vendor, Vulkan physical devices | machine, driver, runtime |
| files | `write_read_roundtrip`, `case_insensitive_open`, `long_path`, `volume_info`, `file_lock_exclusion`, `memory_mapped_file`, `registry_hkcu_roundtrip`, `symlink_unprivileged`, `final_path_name`, `well_known_paths` | File semantics, case handling, long paths, volume flags, locking, mmap, registry, symlink privilege, `\\?\` path forms, system folder layout | runtime (and filesystem) |
| network | `winsock_startup`, `dns_resolve`, `tcp_connect_443`, `winhttp_get`, `icmp_echo`, `reachability`, `adapters`, `udp_bind_loopback` | Winsock init, resolver, TCP connect, WinHTTP over TLS, ICMP echo via iphlpapi (with the `tcp-reachability` fallback), adapter count, UDP bind | network policy and runtime |

## 6. Classifying differences

| Observation | Classification | Action |
| --- | --- | --- |
| Status differs (`ok` vs `fail`/`unsupported`) and the probe is marked runtime in the inventory | Behavioural | Record as a finding; consider a workaround. |
| Status differs and the probe is marked machine/display/network | Environmental | Note it; re-run on comparable hardware if it matters. |
| Only `value` differs for a machine-marked probe (GPU name, VRAM, processor count) | Environmental | Ignore. |
| `value` differs for a runtime-marked probe (version string, path casing, drive letters, filesystem flags) | Behavioural | Record as a finding. |
| A workaround changes a status from `fail` to `ok` in the before/after report | Fix verified | Keep the workaround on by default. |

## 7. Repeatability guarantees

- Keys are emitted sorted; the documents are stable across runs apart from the volatile leaves that `compare` excludes.
- The binary is built once and reused, so differences cannot come from compiler or source drift.
- Each probe is isolated by `timed(...)`, so a crash in one probe surfaces as a `fail` entry rather than a missing file.
- Both the `none` and `all` workaround runs are kept, so a fix is always shown against its baseline.

## 8. Exit criteria for a run

A run is complete when all six result files exist (Proton may be missing in CI; the report then says so), both reports are generated, and each **yes** row in the status matrix has a classification in the review notes or an issue.
