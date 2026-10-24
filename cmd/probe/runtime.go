//go:build windows

package main

import (
	"os"
	"unsafe"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

const secRuntime = `runtime`

// Environment variables whose presence (never their value) hints at Proton or Wine.
var runtimeEnvHints = []string{
	`STEAM_COMPAT_DATA_PATH`,
	`STEAM_COMPAT_CLIENT_INSTALL_PATH`,
	`STEAM_COMPAT_TOOL_PATHS`,
	`SteamGameId`,
	`SteamAppId`,
	`PROTON_LOG`,
	`WINEPREFIX`,
	`WINEDLLOVERRIDES`,
	`WINELOADER`,
}

func probeRuntime(doc *schema.Doc) {
	wineVer := ntdll.NewProc(`wine_get_version`)
	isWine := wineVer.Find() == nil
	timed(doc, secRuntime, `wine_version`, func() schema.Node {
		if !isWine {
			return schema.Unsupported(`ntdll does not export wine_get_version`)
		}
		r, _, _ := wineVer.Call()
		return schema.OK(cstr(r))
	})
	timed(doc, secRuntime, `wine_build_id`, func() schema.Node {
		p := ntdll.NewProc(`wine_get_build_id`)
		if p.Find() != nil {
			return schema.Unsupported(`ntdll does not export wine_get_build_id`)
		}
		r, _, _ := p.Call()
		return schema.OK(cstr(r))
	})
	timed(doc, secRuntime, `wine_host`, func() schema.Node {
		p := ntdll.NewProc(`wine_get_host_version`)
		if p.Find() != nil {
			return schema.Unsupported(`ntdll does not export wine_get_host_version`)
		}
		var sysname, release uintptr
		p.Call(uintptr(unsafe.Pointer(&sysname)), uintptr(unsafe.Pointer(&release)))
		n := schema.Node{}
		n[`sysname`] = cstr(sysname)
		n[`release`] = cstr(release)
		return schema.OK(n)
	})
	hints := schema.Node{}
	proton := false
	for _, name := range runtimeEnvHints {
		_, set := os.LookupEnv(name)
		hints[name] = set
		if set && (name == `STEAM_COMPAT_DATA_PATH` || name == `STEAM_COMPAT_TOOL_PATHS`) {
			proton = true
		}
	}
	timed(doc, secRuntime, `env_hints`, func() schema.Node {
		return schema.OK(hints)
	})
	timed(doc, secRuntime, `classification`, func() schema.Node {
		switch {
		case isWine && proton:
			return schema.OK(`proton`)
		case isWine:
			return schema.OK(`wine`)
		default:
			return schema.OK(`windows`)
		}
	})
}
