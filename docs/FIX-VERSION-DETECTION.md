# Fix: OS version detection (`rtl-version` workaround)

## The problem

Since Windows 8.1, `GetVersionExW` (and `GetVersion`) lie unless the executable carries an application manifest with a `supportedOS` entry for the running Windows release. Without that manifest an application sees `6.2.9200` (Windows 8) on Windows 8.1, 10, and 11. Go binaries built with plain `go build` have no manifest, and so do many small tools and game launchers.

Wine does not implement that compatibility shim. `GetVersionExW` under Wine returns whatever version the prefix is configured to report (Wine 9 defaults to Windows 10, build 19043 in most builds; Proton configures its own). The same binary therefore reports a *newer* Windows under Wine than on real Windows, and any feature gate written as `if major >= 10` behaves differently in the two environments.

## The fix

Ask the kernel instead of the shim: `RtlGetVersion` from `ntdll.dll` returns the true version on every Windows release and is implemented by Wine and Proton. Alternatively, embed a manifest with the Windows 10 `supportedOS` GUID; that fixes Windows but changes nothing under Wine, so the two approaches are not equivalent.

In the probe, the canonical `os.version` entry is sourced from `RtlGetVersion` when the `rtl-version` workaround is enabled (the default) and from `GetVersionExW` when it is disabled. Both raw readings are always recorded as `os.rtl_get_version` and `os.get_version_ex`, so the lie is visible in every run.

## Demonstrating it

```sh
probe.exe -workarounds none -out results/ENV-before.json
probe.exe -workarounds all  -out results/ENV.json
make compare-fix     # docs/FIX-BEFORE-AFTER.md
```

`compare-fix` compares the before and after files for each environment. Look at these rows in the Differing values table:

| Path | Before (`-workarounds none`) | After (`-workarounds all`) |
| --- | --- | --- |
| `sections.os.version.source` | `GetVersionExW` | `RtlGetVersion` |
| `sections.os.version.value.string` (Windows) | `6.2.9200` | the real version, for example `10.0.20348` on Windows Server 2022 or `10.0.26100` on Windows Server 2025 |
| `sections.os.version.value.string` (Wine, Proton) | configured version, for example `10.0.19043` | the same configured version |
| `sections.os.version.workaround` | absent | `rtl-version` |

Before the fix, the Windows vs Wine comparison shows a version mismatch that is an artefact of the shim; after the fix, both environments report the version their kernel claims and the comparison report is about real differences again.

CI runs this on every push and commits the results when dispatched with `publish=true`. The committed `docs/FIX-BEFORE-AFTER.md` is the evidence; the `results/*-before.json` and `results/*.json` files are the raw data behind it.

## Two more workarounds shipped alongside

They use the same mechanism and show up in the same before/after report.

- `d3d-warp-fallback`: when `D3D11CreateDevice` with `D3D_DRIVER_TYPE_HARDWARE` fails (no GPU, headless runner, missing driver), retry with `D3D_DRIVER_TYPE_WARP`. On the GPU-less Windows runner this turns `graphics.d3d11_device` from `fail` into `ok` with `driver_type: warp`. Wine has no WARP, so the fallback is recorded as attempted and failed there, which is itself a useful difference.
- `tcp-reachability`: `IcmpSendEcho` fails under Wine without raw-socket permission and on networks that drop ICMP. The fallback opens a TCP connection to `1.1.1.1:443` and records `source: tcp_fallback` on `network.reachability`.

## Not in scope

The fix changes how the probe reads the version. It does not patch Wine, Proton, or Windows, and it does not interact with any anti-cheat component.
