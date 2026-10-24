//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

const (
	dxgiErrorNotFound    = 0x887a0002
	dxgiAdapterSoftware  = 0x2
	vkStructInstanceInfo = 1
)

// dxgiAdapterDesc1 mirrors DXGI_ADAPTER_DESC1.
type dxgiAdapterDesc1 struct {
	description           [128]uint16
	vendorID              uint32
	deviceID              uint32
	subSysID              uint32
	revision              uint32
	dedicatedVideoMemory  uintptr
	dedicatedSystemMemory uintptr
	sharedSystemMemory    uintptr
	adapterLUID           [2]uint32
	flags                 uint32
}

// vkInstanceCreateInfo mirrors VkInstanceCreateInfo on 64-bit targets.
type vkInstanceCreateInfo struct {
	sType                   uint32
	pNext                   uintptr
	flags                   uint32
	pApplicationInfo        uintptr
	enabledLayerCount       uint32
	ppEnabledLayerNames     uintptr
	enabledExtensionCount   uint32
	ppEnabledExtensionNames uintptr
}

var iidDXGIFactory1 = guid{0x770aae78, 0xf26f, 0x4dba, [8]byte{0xa8, 0x29, 0x25, 0x3c, 0x83, 0xd1, 0xb3, 0x87}}

var vendorNames = map[uint32]string{
	0x10de:  `nvidia`,
	0x1002:  `amd`,
	0x8086:  `intel`,
	0x1414:  `microsoft`,
	0x106b:  `apple`,
	0x10005: `mesa`,
	0x1ae0:  `google`,
	0x15ad:  `vmware`,
	0x1af4:  `virtio`,
}

var vkDeviceTypeNames = map[uint32]string{
	0: `other`,
	1: `integrated_gpu`,
	2: `discrete_gpu`,
	3: `virtual_gpu`,
	4: `cpu`,
}

func probeDXGI() schema.Node {
	p := dxgi.NewProc(`CreateDXGIFactory1`)
	if p.Find() != nil {
		return schema.Unsupported(`dxgi.dll or CreateDXGIFactory1 missing`)
	}
	var factory uintptr
	hr, _, _ := p.Call(uintptr(unsafe.Pointer(&iidDXGIFactory1)), uintptr(unsafe.Pointer(&factory)))
	if failed(hr) || factory == 0 {
		return schema.Fail(`CreateDXGIFactory1 HRESULT ` + hresultText(hr))
	}
	defer release(factory)
	var adapters []schema.Node
	for i := 0; i < 16; i++ {
		var adapter uintptr
		hr, _ := vcall(factory, 12, uintptr(i), uintptr(unsafe.Pointer(&adapter)))
		if uint32(hr) == dxgiErrorNotFound || adapter == 0 {
			break
		}
		if failed(hr) {
			return schema.Fail(`EnumAdapters1 HRESULT ` + hresultText(hr))
		}
		var desc dxgiAdapterDesc1
		hr, _ = vcall(adapter, 10, uintptr(unsafe.Pointer(&desc)))
		release(adapter)
		if failed(hr) {
			return schema.Fail(`GetDesc1 HRESULT ` + hresultText(hr))
		}
		a := schema.Node{}
		a[`description`] = wstr(desc.description[:])
		a[`vendor_id`] = fmt.Sprintf(`0x%04X`, desc.vendorID)
		a[`vendor`] = vendorNames[desc.vendorID]
		a[`device_id`] = fmt.Sprintf(`0x%04X`, desc.deviceID)
		a[`dedicated_video_mb`] = uint64(desc.dedicatedVideoMemory) / (1024 * 1024)
		a[`shared_system_mb`] = uint64(desc.sharedSystemMemory) / (1024 * 1024)
		a[`software`] = desc.flags&dxgiAdapterSoftware != 0
		adapters = append(adapters, a)
	}
	n := schema.Node{}
	n[`count`] = len(adapters)
	n[`adapters`] = adapters
	if len(adapters) == 0 {
		return schema.Fail(`factory created but no adapters enumerated`).With(schema.KeyValue, n)
	}
	return schema.OK(n)
}

func probeVulkan() schema.Node {
	create := vulkan.NewProc(`vkCreateInstance`)
	if create.Find() != nil {
		return schema.Unsupported(`vulkan-1.dll or vkCreateInstance missing`)
	}
	n := schema.Node{}
	if ev := vulkan.NewProc(`vkEnumerateInstanceVersion`); ev.Find() == nil {
		var ver uint32
		ev.Call(uintptr(unsafe.Pointer(&ver)))
		n[`instance_api_version`] = vkVersion(ver)
	}
	info := vkInstanceCreateInfo{sType: vkStructInstanceInfo}
	var instance uintptr
	res, _, _ := create.Call(uintptr(unsafe.Pointer(&info)), 0, uintptr(unsafe.Pointer(&instance)))
	if int32(res) != 0 || instance == 0 {
		return schema.Fail(fmt.Sprintf(`vkCreateInstance VkResult %d`, int32(res))).With(schema.KeyValue, n)
	}
	defer vulkan.NewProc(`vkDestroyInstance`).Call(instance, 0)
	enum := vulkan.NewProc(`vkEnumeratePhysicalDevices`)
	var count uint32
	enum.Call(instance, uintptr(unsafe.Pointer(&count)), 0)
	n[`physical_device_count`] = count
	if count == 0 {
		return schema.Fail(`instance created but no physical devices`).With(schema.KeyValue, n)
	}
	devs := make([]uintptr, count)
	enum.Call(instance, uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&devs[0])))
	getProps := vulkan.NewProc(`vkGetPhysicalDeviceProperties`)
	var list []schema.Node
	for _, d := range devs[:count] {
		var props [4096]byte
		getProps.Call(d, uintptr(unsafe.Pointer(&props[0])))
		item := schema.Node{}
		item[`api_version`] = vkVersion(le32(props[0:4]))
		item[`driver_version`] = le32(props[4:8])
		item[`vendor_id`] = fmt.Sprintf(`0x%04X`, le32(props[8:12]))
		item[`vendor`] = vendorNames[le32(props[8:12])]
		item[`device_type`] = vkDeviceTypeNames[le32(props[16:20])]
		item[`device_name`] = cstrBytes(props[20:276])
		list = append(list, item)
	}
	n[`devices`] = list
	return schema.OK(n)
}

func vkVersion(v uint32) string {
	return fmt.Sprintf(`%d.%d.%d`, v>>22, (v>>12)&0x3ff, v&0xfff)
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func cstrBytes(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
