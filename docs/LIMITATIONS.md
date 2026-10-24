# Known limitations

## Scope

- **No anti-cheat interaction.** The probe does not load, inspect, modify, or bypass Easy Anti-Cheat, BattlEye, Vanguard, or any other anti-cheat driver, service, or DLL. It is not a tool for evaluating whether protected games run; it measures the platform surface a plain application sees.
- Prototype quality. The probe set is a sample of what matters for compatibility, not an exhaustive survey. The `apis` section checks 70 exports; Windows has tens of thousands.

## What CI can and cannot tell you

- The GitHub-hosted Windows runner has no GPU. `graphics.d3d11_device` fails with the hardware driver and succeeds only through the WARP fallback; `dxgi_adapters` lists the Microsoft Basic Render Driver; `vulkan` is usually unsupported there.
- Wine and Proton run under Xvfb with Mesa software rendering (llvmpipe for OpenGL, lavapipe for Vulkan). Graphics results reflect that stack, not a real GPU or driver. Treat graphics rows as environmental unless both sides had comparable hardware.
- The Proton run uses GE-Proton standalone, launched through its `proton` script with the two `STEAM_COMPAT_*` variables set. There is no Steam client, no Steam Linux Runtime (pressure-vessel) container, and no Steam overlay. Results can differ from Steam-launched Proton, especially around DXVK/VKD3D defaults and the sandboxed filesystem view. The job is `continue-on-error`, so a Proton bootstrap failure does not block the Windows and Wine comparison.
- Network probes depend on runner egress. ICMP to the public internet is frequently dropped by cloud networks, so `network.icmp_echo` can fail everywhere; that is what the `tcp-reachability` fallback is for.
- `apt` package versions move. The Wine version is printed in the job log (`wine --version`) and recorded in `runtime.wine_version`; GE-Proton is pinned by tag in the workflow `env`.

## Binary and build

- Only `windows/amd64` is built. No 32-bit or ARM64 Windows binary, so WoW64 and ARM64 emulation paths are not probed directly (though `os.wow64_machines` reports the native machine).
- The binary has no application manifest. That is deliberate: it is what exposes the `GetVersionExW` version lie the demonstrated fix addresses. It also means the process is not declared DPI-aware or long-path-aware; `files.long_path` relies on the Go runtime adding the `\\?\` prefix.
- `go vet` for the Windows target runs with `-unsafeptr=false` because Win32 returns raw pointers as `uintptr`. The remaining vet checks are on.
- The DXGI and Vulkan probes call COM and C ABIs by hand through `syscall`. Structure layouts are for 64-bit only.

## Repeatability caveats

- `graphics.current_display_mode`, `dxgi_adapters`, `vulkan`, `native_system_info`, and `network.adapters` describe the machine and will differ between any two machines; `compare` reports them, the test plan classifies them as environmental.
- `network.dns_resolve` records address counts, which can change with DNS answers.
- Wine results depend on the prefix configuration (reported Windows version, DLL overrides). CI uses a fresh default prefix; a desktop prefix may differ.

## Privacy

Reported: version numbers, flags, counts, path casing, drive letters, GPU model names, adapter counts. Not reported: hostnames, user names, volume serials, MAC or IP addresses, environment variable values (only presence), or any file contents. The temp directory path has the account segment replaced with `[user]`.

## Repository mechanics

- All commits so far were created through the GitHub API from a sandboxed session with no local write access. They are signed by GitHub (shown as Verified) rather than by a personal key, and the files carry mode `100644`, so `make hooks` sets the executable bit on the hook and scripts are run via `sh`.
- The very first commit (`chore: bootstrap repository`) was created through the REST Contents API, which GitHub does not sign, before the signed GraphQL path was confirmed. It adds only a placeholder README. Rewriting history to replace it requires a force-push and is left to the repository owner.
