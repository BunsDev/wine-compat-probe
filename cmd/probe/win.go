//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// Lazily loaded system libraries. Loading never happens until a probe asks.
var (
	ntdll    = syscall.NewLazyDLL(`ntdll.dll`)
	kernel32 = syscall.NewLazyDLL(`kernel32.dll`)
	advapi32 = syscall.NewLazyDLL(`advapi32.dll`)
	user32   = syscall.NewLazyDLL(`user32.dll`)
	dxgi     = syscall.NewLazyDLL(`dxgi.dll`)
	d3d11    = syscall.NewLazyDLL(`d3d11.dll`)
	d3d12    = syscall.NewLazyDLL(`d3d12.dll`)
	vulkan   = syscall.NewLazyDLL(`vulkan-1.dll`)
	winhttp  = syscall.NewLazyDLL(`winhttp.dll`)
	iphlpapi = syscall.NewLazyDLL(`iphlpapi.dll`)
	ws2_32   = syscall.NewLazyDLL(`ws2_32.dll`)
)

// guid mirrors the Win32 GUID layout.
type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

// utf16 converts a Go string to a NUL-terminated UTF-16 pointer as uintptr.
func utf16(s string) uintptr {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return 0
	}
	return uintptr(unsafe.Pointer(p))
}

// wstr converts a NUL-terminated UTF-16 buffer to a Go string.
func wstr(buf []uint16) string {
	return syscall.UTF16ToString(buf)
}

// cstr reads a NUL-terminated byte string from a raw pointer.
func cstr(p uintptr) string {
	if p == 0 {
		return ``
	}
	var out []byte
	for i := uintptr(0); i < 4096; i++ {
		c := *(*byte)(unsafe.Pointer(p + i))
		if c == 0 {
			break
		}
		out = append(out, c)
	}
	return string(out)
}

// errText formats a Win32 error returned by syscall.
func errText(e error) string {
	if e == nil {
		return ``
	}
	if en, ok := e.(syscall.Errno); ok {
		return fmt.Sprintf(`0x%08X %s`, uint32(en), en.Error())
	}
	return e.Error()
}

// hresultText formats an HRESULT.
func hresultText(hr uintptr) string {
	return fmt.Sprintf(`0x%08X`, uint32(hr))
}

// failed reports whether an HRESULT indicates failure.
func failed(hr uintptr) bool {
	return int32(uint32(hr)) < 0
}

// vcall invokes method index idx on a COM object pointer.
func vcall(obj uintptr, idx int, args ...uintptr) (uintptr, error) {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(idx)*unsafe.Sizeof(uintptr(0))))
	all := append([]uintptr{obj}, args...)
	r, _, errno := syscall.SyscallN(fn, all...)
	if errno != 0 {
		return r, errno
	}
	return r, nil
}

// release calls IUnknown::Release (vtable slot 2) when obj is non-nil.
func release(obj uintptr) {
	if obj != 0 {
		vcall(obj, 2)
	}
}
