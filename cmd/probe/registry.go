//go:build windows

package main

import (
	"strings"
	"syscall"
	"unsafe"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

const regProbeKey = `Software\WineCompatProbe`

func probeRegistryRoundtrip() schema.Node {
	create := advapi32.NewProc(`RegCreateKeyExW`)
	set := advapi32.NewProc(`RegSetValueExW`)
	del := advapi32.NewProc(`RegDeleteKeyW`)
	var h syscall.Handle
	var disposition uint32
	r, _, _ := create.Call(uintptr(syscall.HKEY_CURRENT_USER), utf16(regProbeKey), 0, 0, 0, keyAllAccess, 0, uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(&disposition)))
	if r != 0 {
		return schema.Fail(`RegCreateKeyExW: ` + errText(syscall.Errno(r)))
	}
	defer del.Call(uintptr(syscall.HKEY_CURRENT_USER), utf16(regProbeKey))
	defer syscall.RegCloseKey(h)
	value, err := syscall.UTF16FromString(probePayload)
	if err != nil {
		return schema.Fail(err.Error())
	}
	r, _, _ = set.Call(uintptr(h), utf16(`probe`), 0, regSZ, uintptr(unsafe.Pointer(&value[0])), uintptr(len(value)*2))
	if r != 0 {
		return schema.Fail(`RegSetValueExW: ` + errText(syscall.Errno(r)))
	}
	got, err := regString(syscall.HKEY_CURRENT_USER, regProbeKey, `probe`)
	if err != nil {
		return schema.Fail(`read back: ` + errText(err))
	}
	if got != probePayload {
		return schema.Fail(`read back mismatch: ` + got)
	}
	n := schema.Node{}
	n[`created_new_key`] = disposition == 1
	return schema.OK(n)
}

func openShared(path string) (syscall.Handle, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return syscall.CreateFile(p, syscall.GENERIC_READ|syscall.GENERIC_WRITE, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
}

// redactUser replaces the account name in C:\Users\NAME\... paths.
func redactUser(p string) string {
	parts := strings.Split(p, string(rune(backslash)))
	if len(parts) > 2 && strings.EqualFold(parts[1], `users`) {
		parts[2] = `[user]`
	}
	return strings.Join(parts, string(rune(backslash)))
}

func driveOf(p string) string {
	if len(p) >= 2 && p[1] == colon {
		return strings.ToUpper(p[:1])
	}
	return ``
}
