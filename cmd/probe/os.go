//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

const secOS = `os`

const regCurrentVersion = `SOFTWARE\Microsoft\Windows NT\CurrentVersion`

const driveA = 0x41

// osVersionInfo mirrors OSVERSIONINFOW / RTL_OSVERSIONINFOW.
type osVersionInfo struct {
	size       uint32
	major      uint32
	minor      uint32
	build      uint32
	platformID uint32
	csdVersion [128]uint16
}

// systemInfo mirrors SYSTEM_INFO.
type systemInfo struct {
	processorArchitecture uint16
	reserved              uint16
	pageSize              uint32
	minAppAddress         uintptr
	maxAppAddress         uintptr
	activeProcessorMask   uintptr
	numberOfProcessors    uint32
	processorType         uint32
	allocationGranularity uint32
	processorLevel        uint16
	processorRevision     uint16
}

var archNames = map[uint16]string{
	0:  `x86`,
	5:  `arm`,
	6:  `ia64`,
	9:  `amd64`,
	12: `arm64`,
}

// IMAGE_FILE_MACHINE values; 0 means the process is not running under WoW64.
var machineNames = map[uint16]string{
	0x0000: `none (not wow64)`,
	0x014c: `i386`,
	0x8664: `amd64`,
	0x01c4: `armnt`,
	0xaa64: `arm64`,
}

// Registry values under CurrentVersion: value name, output key.
var currentVersionValues = [][2]string{
	{`ProductName`, `product_name`},
	{`DisplayVersion`, `display_version`},
	{`ReleaseId`, `release_id`},
	{`CurrentBuild`, `current_build`},
	{`CurrentBuildNumber`, `current_build_number`},
	{`CurrentMajorVersionNumber`, `current_major_version_number`},
	{`CurrentMinorVersionNumber`, `current_minor_version_number`},
	{`UBR`, `ubr`},
	{`EditionID`, `edition_id`},
	{`InstallationType`, `installation_type`},
}

func probeOS(doc *schema.Doc, opts *options) {
	var rtl, legacy schema.Node
	timed(doc, secOS, `rtl_get_version`, func() schema.Node {
		p := ntdll.NewProc(`RtlGetVersion`)
		if p.Find() != nil {
			return schema.Unsupported(`RtlGetVersion missing`)
		}
		var v osVersionInfo
		v.size = uint32(unsafe.Sizeof(v))
		status, _, _ := p.Call(uintptr(unsafe.Pointer(&v)))
		if status != 0 {
			return schema.Fail(`NTSTATUS ` + hresultText(status))
		}
		rtl = versionNode(v)
		return schema.OK(rtl)
	})
	timed(doc, secOS, `get_version_ex`, func() schema.Node {
		p := kernel32.NewProc(`GetVersionExW`)
		if p.Find() != nil {
			return schema.Unsupported(`GetVersionExW missing`)
		}
		var v osVersionInfo
		v.size = uint32(unsafe.Sizeof(v))
		r, _, err := p.Call(uintptr(unsafe.Pointer(&v)))
		if r == 0 {
			return schema.Fail(errText(err))
		}
		legacy = versionNode(v)
		return schema.OK(legacy).With(schema.KeyNote, `subject to the manifest-based version lie on Windows 8.1 and later`)
	})
	timed(doc, secOS, `version`, func() schema.Node {
		if opts.enabled(WorkaroundRtlVersion) && rtl != nil {
			return schema.OK(rtl).With(schema.KeySource, `RtlGetVersion`).With(`workaround`, WorkaroundRtlVersion)
		}
		if legacy != nil {
			return schema.OK(legacy).With(schema.KeySource, `GetVersionExW`)
		}
		return schema.Fail(`no version API succeeded`)
	})
	timed(doc, secOS, `registry_current_version`, func() schema.Node {
		n := schema.Node{}
		got := 0
		for _, kv := range currentVersionValues {
			v, err := regString(syscall.HKEY_LOCAL_MACHINE, regCurrentVersion, kv[0])
			if err != nil {
				continue
			}
			n[kv[1]] = v
			got++
		}
		if got == 0 {
			return schema.Fail(`no CurrentVersion values readable`)
		}
		return schema.OK(n)
	})
	timed(doc, secOS, `native_system_info`, func() schema.Node {
		p := kernel32.NewProc(`GetNativeSystemInfo`)
		if p.Find() != nil {
			return schema.Unsupported(`GetNativeSystemInfo missing`)
		}
		var si systemInfo
		p.Call(uintptr(unsafe.Pointer(&si)))
		n := schema.Node{}
		n[`architecture`] = nameOr(archNames[si.processorArchitecture], si.processorArchitecture)
		n[`processors`] = si.numberOfProcessors
		n[`page_size`] = si.pageSize
		n[`allocation_granularity`] = si.allocationGranularity
		return schema.OK(n)
	})
	timed(doc, secOS, `wow64_machines`, func() schema.Node {
		p := kernel32.NewProc(`IsWow64Process2`)
		if p.Find() != nil {
			return schema.Unsupported(`IsWow64Process2 missing (Windows before 1511, or an older Wine)`)
		}
		var process, native uint16
		h, _ := syscall.GetCurrentProcess()
		r, _, err := p.Call(uintptr(h), uintptr(unsafe.Pointer(&process)), uintptr(unsafe.Pointer(&native)))
		if r == 0 {
			return schema.Fail(errText(err))
		}
		n := schema.Node{}
		n[`process_machine`] = nameOr(machineNames[process], process)
		n[`native_machine`] = nameOr(machineNames[native], native)
		return schema.OK(n)
	})
	timed(doc, secOS, `system_directory`, func() schema.Node {
		dir, err := systemDirectory()
		if err != nil {
			return schema.Fail(errText(err))
		}
		return schema.OK(dir)
	})
	timed(doc, secOS, `windows_directory`, func() schema.Node {
		p := kernel32.NewProc(`GetWindowsDirectoryW`)
		buf := make([]uint16, 520)
		r, _, err := p.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if r == 0 {
			return schema.Fail(errText(err))
		}
		return schema.OK(wstr(buf))
	})
	timed(doc, secOS, `user_locale`, func() schema.Node {
		p := kernel32.NewProc(`GetUserDefaultLocaleName`)
		if p.Find() != nil {
			return schema.Unsupported(`GetUserDefaultLocaleName missing`)
		}
		buf := make([]uint16, 85)
		r, _, err := p.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if r == 0 {
			return schema.Fail(errText(err))
		}
		return schema.OK(wstr(buf))
	})
	timed(doc, secOS, `logical_drives`, func() schema.Node {
		p := kernel32.NewProc(`GetLogicalDrives`)
		mask, _, err := p.Call()
		if mask == 0 {
			return schema.Fail(errText(err))
		}
		var letters []string
		for i := 0; i < 26; i++ {
			if mask&(1<<uint(i)) != 0 {
				letters = append(letters, string(rune(driveA+i)))
			}
		}
		return schema.OK(letters)
	})
	timed(doc, secOS, `system_dpi`, func() schema.Node {
		p := user32.NewProc(`GetDpiForSystem`)
		if p.Find() != nil {
			return schema.Unsupported(`GetDpiForSystem missing`)
		}
		dpi, _, _ := p.Call()
		return schema.OK(dpi)
	})
}

func versionNode(v osVersionInfo) schema.Node {
	n := schema.Node{}
	n[`major`] = v.major
	n[`minor`] = v.minor
	n[`build`] = v.build
	n[`platform_id`] = v.platformID
	n[`string`] = fmt.Sprintf(`%d.%d.%d`, v.major, v.minor, v.build)
	csd := wstr(v.csdVersion[:])
	if csd != `` {
		n[`csd_version`] = csd
	}
	return n
}

func nameOr(name string, raw uint16) string {
	if name != `` {
		return name
	}
	return fmt.Sprintf(`unknown(%d)`, raw)
}

func systemDirectory() (string, error) {
	p := kernel32.NewProc(`GetSystemDirectoryW`)
	buf := make([]uint16, 520)
	r, _, err := p.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return ``, err
	}
	return wstr(buf), nil
}

// regString reads a REG_SZ, REG_EXPAND_SZ, or REG_DWORD value as text.
func regString(root syscall.Handle, path, name string) (string, error) {
	var h syscall.Handle
	pathP, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return ``, err
	}
	if err := syscall.RegOpenKeyEx(root, pathP, 0, syscall.KEY_READ, &h); err != nil {
		return ``, err
	}
	defer syscall.RegCloseKey(h)
	nameP, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return ``, err
	}
	var typ, size uint32
	if err := syscall.RegQueryValueEx(h, nameP, nil, &typ, nil, &size); err != nil {
		return ``, err
	}
	buf := make([]byte, size+2)
	if err := syscall.RegQueryValueEx(h, nameP, nil, &typ, &buf[0], &size); err != nil {
		return ``, err
	}
	switch typ {
	case syscall.REG_SZ, syscall.REG_EXPAND_SZ:
		u := unsafe.Slice((*uint16)(unsafe.Pointer(&buf[0])), len(buf)/2)
		return wstr(u), nil
	case syscall.REG_DWORD:
		return fmt.Sprint(*(*uint32)(unsafe.Pointer(&buf[0]))), nil
	}
	return fmt.Sprintf(`type %d, %d bytes`, typ, size), nil
}
