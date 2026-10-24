# Output schema (`wine-compat-probe/1`)

The probe writes one JSON object. Keys are emitted in sorted order at every level, so the file is stable and diffable. There are no struct tags in the code; the shape below is produced from maps.

## Top level

| Key | Type | Content |
| --- | --- | --- |
| `meta` | object | `schema` (this identifier), `tool` (for example `probe/0.1.0`), `timestamp` (RFC 3339, UTC), `workarounds` (sorted list of enabled workaround names), `args` (command line), `total_ms`. |
| `summary` | object | `total` plus one counter per status seen: `ok`, `fail`, `skipped`, `unsupported`. |
| `sections` | object | One object per section: `runtime`, `os`, `apis`, `graphics`, `files`, `network`. Each section maps probe name to a probe entry. |

## Probe entry

| Key | Present when | Meaning |
| --- | --- | --- |
| `status` | always | `ok`, `fail`, `skipped`, or `unsupported`. |
| `value` | `ok`, sometimes `fail` | The measurement: scalar, list, or object. On `fail` it may hold partial data. |
| `error` | `fail` | Human-readable error, usually including the Win32 code as `0xXXXXXXXX` or an HRESULT. |
| `note` | `skipped`, `unsupported`, some `ok` | Why it was skipped or absent, or an interpretation hint. |
| `source` | some `ok` | Which API produced the value when more than one could (`RtlGetVersion`, `GetVersionExW`, `icmp_echo`, `tcp_fallback`, `fallback`). |
| `workaround` | when a workaround changed the result | Name of the workaround (`rtl-version`, `d3d-warp-fallback`, `tcp-reachability`). |
| `duration_ms` | always | Wall time of the probe. Volatile. |

## Paths as `compare` sees them

`compare` flattens the document into dotted paths, for example `sections.os.version.value.string` or `sections.apis.kernel32!GetTickCount64.status`. List elements use their index (`sections.graphics.dxgi_adapters.value.adapters.0.vendor`).

Rows whose last path segment is `status` form the status matrix (shown as `section.probe`). Everything else that differs between inputs lands in the Differing values table. Leaves named `timestamp`, `duration_ms`, `rtt_ms`, `total_ms`, or `args` are excluded.

## Sections and notable values

- `runtime`: `wine_version` (string from `wine_get_version`), `wine_build_id`, `wine_host` (`sysname`, `release`), `env_hints` (variable name to boolean presence), `classification` (`windows`, `wine`, `proton`).
- `os`: `rtl_get_version` and `get_version_ex` (`major`, `minor`, `build`, `platform_id`, `string`), `version` (canonical, with `source`), `registry_current_version` (`product_name`, `display_version`, `current_build`, ...), `native_system_info` (`architecture`, `processors`, `page_size`, `allocation_granularity`), `wow64_machines` (`process_machine`, `native_machine`), `system_directory`, `windows_directory`, `user_locale`, `logical_drives` (list of letters), `system_dpi`.
- `apis`: one entry per `module!Export` with `ok` (present) or `unsupported` (`export missing` or `dll not loadable`), plus `_counts` (`checked`, `present`, `missing_export`, `missing_dll`).
- `graphics`: `display_devices` (`count`, `active`, `primary_adapter`, `primary_device`), `current_display_mode` (`width`, `height`, `bits_per_pixel`, `refresh_hz`), `d3d11_device` (`driver_type`, `feature_level`, `feature_level_name`, optional `hardware_error`), `dxgi_adapters` (`count`, `adapters[]` with `description`, `vendor`, `vendor_id`, `device_id`, `dedicated_video_mb`, `shared_system_mb`, `software`), `d3d12_device`, `vulkan` (`instance_api_version`, `physical_device_count`, `devices[]` with `device_name`, `device_type`, `vendor`, `api_version`, `driver_version`).
- `files`: `temp_dir` (redacted), `write_read_roundtrip`, `case_insensitive_open`, `long_path` (`path_length`), `volume_info` (`root`, `filesystem`, `flags`, `case_sensitive_search`, `case_preserved_names`, `unicode_on_disk`, `max_component_length`), `file_lock_exclusion`, `memory_mapped_file`, `registry_hkcu_roundtrip` (`created_new_key`), `symlink_unprivileged`, `final_path_name` (`has_nt_prefix`, `drive`), `well_known_paths` (`windir`, `program_files`, booleans for x86 folder, profile, system32 and syswow64 kernel32).
- `network`: `winsock_startup` (`version`, `high_version`), `dns_resolve` (`host`, `address_count`, `ipv4`, `ipv6`), `tcp_connect_443`, `winhttp_get` (`url`, `status_code`), `icmp_echo` (`target`, `reply_status`, `rtt_ms`), `reachability` (with `source`), `adapters` (`count`), `udp_bind_loopback`. All are `skipped` with `-no-network`.

## Versioning

Bump `schema.Version` when a key is renamed or removed or a status value is added. Adding probes or value keys does not require a bump.
