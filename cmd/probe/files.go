//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

const secFiles = `files`

const (
	ntPrefix              = `\\?\`
	lockFailImmediately   = 0x1
	lockExclusive         = 0x2
	pageReadWrite         = 0x4
	fileMapAllAccess      = 0xf001f
	symlinkUnprivileged   = 0x2
	keyAllAccess          = 0xf003f
	regSZ                 = 1
	fsCaseSensitiveSearch = 0x1
	fsCasePreservedNames  = 0x2
	fsUnicodeOnDisk       = 0x4
	backslash             = 0x5c
	colon                 = 0x3a
)

const probePayload = `wine-compat-probe payload`

func probeFiles(doc *schema.Doc) {
	tmp, err := os.MkdirTemp(``, `wcp-`)
	if err != nil {
		doc.Set(secFiles, `temp_dir`, schema.Fail(`MkdirTemp: `+err.Error()))
		return
	}
	defer os.RemoveAll(tmp)
	doc.Set(secFiles, `temp_dir`, schema.OK(redactUser(filepath.Dir(tmp))))

	timed(doc, secFiles, `write_read_roundtrip`, func() schema.Node {
		p := filepath.Join(tmp, `roundtrip.txt`)
		if err := os.WriteFile(p, []byte(probePayload), 0o644); err != nil {
			return schema.Fail(err.Error())
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return schema.Fail(err.Error())
		}
		if string(b) != probePayload {
			return schema.Fail(`content mismatch after write`)
		}
		if err := os.Remove(p); err != nil {
			return schema.Fail(`remove: ` + err.Error())
		}
		return schema.OK(len(b))
	})
	timed(doc, secFiles, `case_insensitive_open`, func() schema.Node {
		p := filepath.Join(tmp, `Probe-Case.TXT`)
		if err := os.WriteFile(p, []byte(probePayload), 0o644); err != nil {
			return schema.Fail(err.Error())
		}
		if _, err := os.Stat(filepath.Join(tmp, `probe-case.txt`)); err != nil {
			return schema.Fail(`lowercase lookup failed: ` + err.Error())
		}
		return schema.OK(true).With(schema.KeyNote, `created Probe-Case.TXT, found probe-case.txt`)
	})
	timed(doc, secFiles, `long_path`, func() schema.Node {
		seg := strings.Repeat(`a`, 120)
		deep := filepath.Join(tmp, seg, seg, seg)
		if err := os.MkdirAll(deep, 0o755); err != nil {
			return schema.Fail(fmt.Sprintf(`MkdirAll (%d chars): %s`, len(deep), err.Error()))
		}
		p := filepath.Join(deep, `file.txt`)
		if err := os.WriteFile(p, []byte(probePayload), 0o644); err != nil {
			return schema.Fail(err.Error())
		}
		n := schema.Node{}
		n[`path_length`] = len(p)
		return schema.OK(n)
	})
	timed(doc, secFiles, `volume_info`, func() schema.Node {
		sysDir, err := systemDirectory()
		if err != nil || len(sysDir) < 3 {
			return schema.Fail(`cannot determine system drive`)
		}
		root := sysDir[:3]
		p := kernel32.NewProc(`GetVolumeInformationW`)
		fsName := make([]uint16, 64)
		var serial, maxLen, flags uint32
		r, _, callErr := p.Call(utf16(root), 0, 0, uintptr(unsafe.Pointer(&serial)), uintptr(unsafe.Pointer(&maxLen)), uintptr(unsafe.Pointer(&flags)), uintptr(unsafe.Pointer(&fsName[0])), uintptr(len(fsName)))
		if r == 0 {
			return schema.Fail(errText(callErr))
		}
		n := schema.Node{}
		n[`root`] = root
		n[`filesystem`] = wstr(fsName)
		n[`max_component_length`] = maxLen
		n[`flags`] = fmt.Sprintf(`0x%08X`, flags)
		n[`case_sensitive_search`] = flags&fsCaseSensitiveSearch != 0
		n[`case_preserved_names`] = flags&fsCasePreservedNames != 0
		n[`unicode_on_disk`] = flags&fsUnicodeOnDisk != 0
		return schema.OK(n)
	})
	timed(doc, secFiles, `file_lock_exclusion`, func() schema.Node {
		p := filepath.Join(tmp, `lock.txt`)
		if err := os.WriteFile(p, []byte(probePayload), 0o644); err != nil {
			return schema.Fail(err.Error())
		}
		h1, err := openShared(p)
		if err != nil {
			return schema.Fail(`open 1: ` + errText(err))
		}
		defer syscall.CloseHandle(h1)
		h2, err := openShared(p)
		if err != nil {
			return schema.Fail(`open 2: ` + errText(err))
		}
		defer syscall.CloseHandle(h2)
		lock := kernel32.NewProc(`LockFileEx`)
		unlock := kernel32.NewProc(`UnlockFileEx`)
		var ov1, ov2 syscall.Overlapped
		r, _, callErr := lock.Call(uintptr(h1), lockExclusive|lockFailImmediately, 0, 1, 0, uintptr(unsafe.Pointer(&ov1)))
		if r == 0 {
			return schema.Fail(`first lock failed: ` + errText(callErr))
		}
		defer unlock.Call(uintptr(h1), 0, 1, 0, uintptr(unsafe.Pointer(&ov1)))
		r2, _, callErr2 := lock.Call(uintptr(h2), lockExclusive|lockFailImmediately, 0, 1, 0, uintptr(unsafe.Pointer(&ov2)))
		if r2 != 0 {
			unlock.Call(uintptr(h2), 0, 1, 0, uintptr(unsafe.Pointer(&ov2)))
			return schema.Fail(`second exclusive lock unexpectedly succeeded`)
		}
		return schema.OK(true).With(schema.KeyNote, `second lock rejected with `+errText(callErr2))
	})
	timed(doc, secFiles, `memory_mapped_file`, func() schema.Node {
		create := kernel32.NewProc(`CreateFileMappingW`)
		mapView := kernel32.NewProc(`MapViewOfFile`)
		unmap := kernel32.NewProc(`UnmapViewOfFile`)
		h, _, callErr := create.Call(^uintptr(0), 0, pageReadWrite, 0, 4096, 0)
		if h == 0 {
			return schema.Fail(`CreateFileMappingW: ` + errText(callErr))
		}
		defer syscall.CloseHandle(syscall.Handle(h))
		view, _, callErr := mapView.Call(h, fileMapAllAccess, 0, 0, 4096)
		if view == 0 {
			return schema.Fail(`MapViewOfFile: ` + errText(callErr))
		}
		defer unmap.Call(view)
		b := unsafe.Slice((*byte)(unsafe.Pointer(view)), 4096)
		copy(b, probePayload)
		if string(b[:len(probePayload)]) != probePayload {
			return schema.Fail(`mapped view readback mismatch`)
		}
		return schema.OK(true)
	})
	timed(doc, secFiles, `registry_hkcu_roundtrip`, func() schema.Node {
		return probeRegistryRoundtrip()
	})
	timed(doc, secFiles, `symlink_unprivileged`, func() schema.Node {
		p := kernel32.NewProc(`CreateSymbolicLinkW`)
		if p.Find() != nil {
			return schema.Unsupported(`CreateSymbolicLinkW missing`)
		}
		target := filepath.Join(tmp, `target.txt`)
		if err := os.WriteFile(target, []byte(probePayload), 0o644); err != nil {
			return schema.Fail(err.Error())
		}
		link := filepath.Join(tmp, `link.txt`)
		r, _, callErr := p.Call(utf16(link), utf16(target), symlinkUnprivileged)
		if r == 0 {
			return schema.Fail(errText(callErr)).With(schema.KeyNote, `expected on Windows without Developer Mode or SeCreateSymbolicLinkPrivilege`)
		}
		return schema.OK(true)
	})
	timed(doc, secFiles, `final_path_name`, func() schema.Node {
		p := kernel32.NewProc(`GetFinalPathNameByHandleW`)
		if p.Find() != nil {
			return schema.Unsupported(`GetFinalPathNameByHandleW missing`)
		}
		target := filepath.Join(tmp, `final.txt`)
		if err := os.WriteFile(target, []byte(probePayload), 0o644); err != nil {
			return schema.Fail(err.Error())
		}
		h, err := openShared(target)
		if err != nil {
			return schema.Fail(errText(err))
		}
		defer syscall.CloseHandle(h)
		buf := make([]uint16, 1024)
		r, _, callErr := p.Call(uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
		if r == 0 {
			return schema.Fail(errText(callErr))
		}
		s := wstr(buf)
		n := schema.Node{}
		n[`has_nt_prefix`] = strings.HasPrefix(s, ntPrefix)
		n[`drive`] = driveOf(strings.TrimPrefix(s, ntPrefix))
		return schema.OK(n)
	})
	timed(doc, secFiles, `well_known_paths`, func() schema.Node {
		n := schema.Node{}
		windir := os.Getenv(`WINDIR`)
		n[`windir`] = windir
		n[`program_files`] = os.Getenv(`ProgramFiles`)
		n[`program_files_x86_set`] = os.Getenv(`ProgramFiles(x86)`) != ``
		n[`userprofile_set`] = os.Getenv(`USERPROFILE`) != ``
		_, err := os.Stat(filepath.Join(windir, `system32`, `kernel32.dll`))
		n[`system32_kernel32_exists`] = err == nil
		_, err = os.Stat(filepath.Join(windir, `syswow64`, `kernel32.dll`))
		n[`syswow64_kernel32_exists`] = err == nil
		return schema.OK(n)
	})
}
