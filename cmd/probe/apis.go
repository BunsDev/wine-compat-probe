//go:build windows

package main

import (
	"strings"
	"syscall"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

const secAPIs = `apis`

type apiCheck struct {
	dll  string
	proc string
}

// apiChecks lists exports whose presence tends to differ between Windows
// releases and Wine/Proton builds. Presence only; nothing here is invoked.
var apiChecks = []apiCheck{
	{`kernel32.dll`, `GetTickCount64`},
	{`kernel32.dll`, `QueryUnbiasedInterruptTime`},
	{`kernel32.dll`, `GetSystemCpuSetInformation`},
	{`kernel32.dll`, `SetThreadDescription`},
	{`kernel32.dll`, `GetThreadDescription`},
	{`kernel32.dll`, `CreateFile2`},
	{`kernel32.dll`, `GetFinalPathNameByHandleW`},
	{`kernel32.dll`, `PrefetchVirtualMemory`},
	{`kernel32.dll`, `SetProcessMitigationPolicy`},
	{`kernel32.dll`, `GetProcessMitigationPolicy`},
	{`kernel32.dll`, `VirtualAlloc2`},
	{`kernel32.dll`, `MapViewOfFile3`},
	{`kernel32.dll`, `IsWow64Process2`},
	{`kernel32.dll`, `GetSystemTimePreciseAsFileTime`},
	{`kernel32.dll`, `WaitOnAddress`},
	{`kernel32.dll`, `GetLogicalProcessorInformationEx`},
	{`kernelbase.dll`, `PathCchCanonicalizeEx`},
	{`ntdll.dll`, `RtlGetVersion`},
	{`ntdll.dll`, `NtQuerySystemInformation`},
	{`ntdll.dll`, `NtQueryInformationProcess`},
	{`ntdll.dll`, `RtlGetDeviceFamilyInfoEnum`},
	{`ntdll.dll`, `wine_get_version`},
	{`user32.dll`, `SetProcessDpiAwarenessContext`},
	{`user32.dll`, `GetDpiForWindow`},
	{`user32.dll`, `EnumDisplayMonitors`},
	{`user32.dll`, `GetRawInputDeviceList`},
	{`shcore.dll`, `GetDpiForMonitor`},
	{`shcore.dll`, `SetProcessDpiAwareness`},
	{`dxgi.dll`, `CreateDXGIFactory1`},
	{`dxgi.dll`, `CreateDXGIFactory2`},
	{`dxgi.dll`, `DXGIGetDebugInterface1`},
	{`d3d11.dll`, `D3D11CreateDevice`},
	{`d3d11.dll`, `D3D11On12CreateDevice`},
	{`d3d12.dll`, `D3D12CreateDevice`},
	{`d3d12.dll`, `D3D12GetDebugInterface`},
	{`d3d12.dll`, `D3D12EnableExperimentalFeatures`},
	{`d3d12core.dll`, `D3D12GetInterface`},
	{`d3d9.dll`, `Direct3DCreate9Ex`},
	{`vulkan-1.dll`, `vkCreateInstance`},
	{`vulkan-1.dll`, `vkEnumerateInstanceVersion`},
	{`opengl32.dll`, `wglGetProcAddress`},
	{`xinput1_4.dll`, `XInputGetState`},
	{`xinput1_3.dll`, `XInputGetState`},
	{`dinput8.dll`, `DirectInput8Create`},
	{`xaudio2_9.dll`, `XAudio2Create`},
	{`mmdevapi.dll`, `ActivateAudioInterfaceAsync`},
	{`winmm.dll`, `timeBeginPeriod`},
	{`winhttp.dll`, `WinHttpOpen`},
	{`winhttp.dll`, `WinHttpWebSocketCompleteUpgrade`},
	{`ws2_32.dll`, `WSAStartup`},
	{`ws2_32.dll`, `GetAddrInfoExW`},
	{`iphlpapi.dll`, `GetAdaptersAddresses`},
	{`iphlpapi.dll`, `IcmpSendEcho2Ex`},
	{`bcrypt.dll`, `BCryptGenRandom`},
	{`bcrypt.dll`, `BCryptOpenAlgorithmProvider`},
	{`crypt32.dll`, `CertOpenStore`},
	{`advapi32.dll`, `RegGetValueW`},
	{`advapi32.dll`, `SystemFunction036`},
	{`ole32.dll`, `CoCreateInstance`},
	{`combase.dll`, `RoInitialize`},
	{`combase.dll`, `RoGetActivationFactory`},
	{`shell32.dll`, `SHGetKnownFolderPath`},
	{`setupapi.dll`, `SetupDiGetClassDevsW`},
	{`hid.dll`, `HidD_GetAttributes`},
	{`powrprof.dll`, `PowerGetActiveScheme`},
	{`api-ms-win-core-synch-l1-2-0.dll`, `WaitOnAddress`},
	{`api-ms-win-core-winrt-l1-1-0.dll`, `RoInitialize`},
	{`ucrtbase.dll`, `_initialize_onexit_table`},
	{`vcruntime140.dll`, `memcpy`},
	{`vcruntime140_1.dll`, `__CxxFrameHandler4`},
	{`msvcp140.dll`, `_Xtime_get_ticks`},
}

func probeAPIs(doc *schema.Doc) {
	cache := map[string]*syscall.LazyDLL{}
	present, missing, noDLL := 0, 0, 0
	for _, c := range apiChecks {
		dll, ok := cache[c.dll]
		if !ok {
			dll = syscall.NewLazyDLL(c.dll)
			cache[c.dll] = dll
		}
		key := strings.TrimSuffix(c.dll, `.dll`) + `!` + c.proc
		if err := dll.Load(); err != nil {
			noDLL++
			doc.Set(secAPIs, key, schema.Unsupported(`dll not loadable: `+errText(err)))
			continue
		}
		if err := dll.NewProc(c.proc).Find(); err != nil {
			missing++
			doc.Set(secAPIs, key, schema.Unsupported(`export missing`))
			continue
		}
		present++
		doc.Set(secAPIs, key, schema.OK(true))
	}
	counts := schema.Node{}
	counts[`present`] = present
	counts[`missing_export`] = missing
	counts[`missing_dll`] = noDLL
	counts[`checked`] = len(apiChecks)
	doc.Set(secAPIs, `_counts`, schema.OK(counts))
}
