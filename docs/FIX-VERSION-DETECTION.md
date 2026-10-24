# Compatibility fixes: what the before/after runs show

`probe.exe` ships three opt-in workarounds (`-workarounds all|none|LIST`). Every CI run executes the probe twice per environment, once with `none` and once with `all`, so each workaround is always shown against its own baseline. `docs/FIX-BEFORE-AFTER.md` is the generated evidence; this page interprets it.

First evidence set: e2e run 40139506110, Windows Server 2025 (build 26100, `windows-latest`, no GPU) and Wine 9.0 (Ubuntu 24.04, Xvfb, Mesa llvmpipe/lavapipe). Proton did not run in that attempt (tarball layout bug, fixed afterwards).

| Workaround | Windows: before, after | Wine: before, after | Verdict |
| --- | --- | --- | --- |
| `tcp-reachability` | `network.reachability` fail (ICMP error 0x2B02), then ok via TCP | fail (ICMP 0x5 access denied), then ok via TCP | **demonstrated fix, both runtimes** |
| `rtl-version` | `os.version` 10.0.26100, unchanged | 10.0.19043, unchanged | no observable change on these runners (see below) |
| `d3d-warp-fallback` | hardware D3D11 device succeeded, fallback not needed | hardware (wined3d) succeeded, fallback not needed | not exercised |

## The demonstrated fix: ICMP echo is not a reliable reachability check

**Problem.** `IcmpSendEcho` (iphlpapi) fails in both environments, for different reasons. Under Wine it returns `ERROR_ACCESS_DENIED` (0x5): Wine implements ICMP echo through raw or ping sockets, which an unprivileged Linux user often cannot open. On the Windows runner it fails with 0x2B02 because the cloud network drops ICMP to the public internet. Any application that gates online features on a ping is broken on both, and worse, broken differently.

**Fix.** When the ICMP probe fails and the workaround is on, open a TCP connection to `1.1.1.1:443` instead and record `source: tcp_fallback` and `workaround: tcp-reachability` on `network.reachability`. The raw `network.icmp_echo` entry is still recorded as failed, so the underlying platform difference remains visible in the comparison report.

**Evidence.** In `docs/FIX-BEFORE-AFTER.md`, `network.reachability` goes from `fail` to `ok` for both `windows-before/windows-after` and `wine-before/wine-after`, and `sections.network.reachability.source` reads `tcp_fallback`.

## The version-lie hypothesis, honestly

The expectation was that an unmanifested binary sees `6.2.9200` from `GetVersionExW` on Windows 8.1 and later, while Wine returns the configured prefix version, so the two would disagree until `RtlGetVersion` is used. On Windows Server 2025 build 26100 that did not happen: `GetVersionExW` returned `10.0.26100`, identical to `RtlGetVersion`. Possible reasons: server SKU or runner image compatibility settings, or a behaviour change in recent Windows builds. A run on a Windows 10/11 client desktop is needed to confirm either way; the test plan lists it as an optional manual environment.

The workaround stays enabled by default because `RtlGetVersion` is correct everywhere, and the probe always records both readings (`os.rtl_get_version`, `os.get_version_ex`) so the lie shows up wherever it exists. Wine reports its prefix version (10.0.19043, "Windows 10 Pro") through both APIs.

## Other behavioural differences found in the first run

Classified with the table in `docs/TEST-PLAN.md`, section 6.

- Wine: `network.udp_bind_loopback` fails with `wsaioctl: winapi error #10045` (`WSAEOPNOTSUPP`). The Go runtime issues a `WSAIoctl` on new UDP sockets that Wine 9.0 does not implement; Windows succeeds. Behavioural; candidate for a fourth workaround (retry without the ioctl, or report the limitation to the Wine project).
- Wine `graphics.dxgi_adapters` reports an "NVIDIA GeForce GTX 470" with about 16 GB of video memory on top of Mesa llvmpipe. That is wined3d's default reported adapter when the real one is unknown; Windows reports "Microsoft Basic Render Driver". Environmental in CI, but it shows how adapter-based feature gating goes wrong under Wine.
- `graphics.vulkan`: Wine succeeds through lavapipe (API 1.4), the Windows runner fails with `VK_ERROR_INCOMPATIBLE_DRIVER` (-9). Environmental (no GPU on the runner).
- Path casing: `C:\windows\system32` under Wine, `C:\Windows\system32` on Windows. Drive letters `[C Z]` versus `[C D]`. Behavioural, expected.
- API surface: Wine exposes `wine_get_version`, `wine_get_build_id`, `wine_get_host_version`; it lacks `ntdll!RtlGetDeviceFamilyInfoEnum`. Windows Server 2025 lacks `xinput1_3.dll` entirely. Both lack `MapViewOfFile3`, `VirtualAlloc2`, and `WaitOnAddress` as kernel32 exports (they live in kernelbase and the api-ms forwarders, which both resolve).
- `os.wow64_machines.process_machine` is `IMAGE_FILE_MACHINE_UNKNOWN` (0) on both, which is the documented value for a process that is not running under WoW64.

## Not in scope

These fixes change how the probe reads the platform. Nothing here patches Wine, Proton, or Windows, and nothing touches any anti-cheat component.
