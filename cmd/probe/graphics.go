//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

const secGraphics = `graphics`

const (
	d3dDriverHardware    = 1
	d3dDriverWarp        = 5
	d3d11SDKVersion      = 7
	d3dFeatureLevel11    = 0xb000
	enumCurrentSettings  = 0xffffffff
	displayDeviceActive  = 0x1
	displayDevicePrimary = 0x4
)

// displayDevice mirrors DISPLAY_DEVICEW.
type displayDevice struct {
	cb           uint32
	deviceName   [32]uint16
	deviceString [128]uint16
	stateFlags   uint32
	deviceID     [128]uint16
	deviceKey    [128]uint16
}

// devMode mirrors DEVMODEW (display variant of the union).
type devMode struct {
	deviceName       [32]uint16
	specVersion      uint16
	driverVersion    uint16
	size             uint16
	driverExtra      uint16
	fields           uint32
	position         [4]uint32
	color            int16
	duplex           int16
	yResolution      int16
	ttOption         int16
	collate          int16
	formName         [32]uint16
	logPixels        uint16
	bitsPerPel       uint32
	pelsWidth        uint32
	pelsHeight       uint32
	displayFlags     uint32
	displayFrequency uint32
	icmMethod        uint32
	icmIntent        uint32
	mediaType        uint32
	ditherType       uint32
	reserved1        uint32
	reserved2        uint32
	panningWidth     uint32
	panningHeight    uint32
}

var iidD3D12Device = guid{0x189819f1, 0x1db6, 0x4b57, [8]byte{0xbe, 0x54, 0x18, 0x21, 0x33, 0x9b, 0x85, 0xf7}}

var featureLevelNames = map[uint32]string{
	0xc200: `12_2`,
	0xc100: `12_1`,
	0xc000: `12_0`,
	0xb100: `11_1`,
	0xb000: `11_0`,
	0xa100: `10_1`,
	0xa000: `10_0`,
	0x9300: `9_3`,
	0x9200: `9_2`,
	0x9100: `9_1`,
}

func probeGraphics(doc *schema.Doc, opts *options) {
	timed(doc, secGraphics, `display_devices`, func() schema.Node {
		p := user32.NewProc(`EnumDisplayDevicesW`)
		if p.Find() != nil {
			return schema.Unsupported(`EnumDisplayDevicesW missing`)
		}
		n := schema.Node{}
		count, active := 0, 0
		for i := 0; i < 16; i++ {
			var dd displayDevice
			dd.cb = uint32(unsafe.Sizeof(dd))
			r, _, _ := p.Call(0, uintptr(i), uintptr(unsafe.Pointer(&dd)), 0)
			if r == 0 {
				break
			}
			count++
			if dd.stateFlags&displayDeviceActive != 0 {
				active++
			}
			if dd.stateFlags&displayDevicePrimary != 0 {
				n[`primary_adapter`] = wstr(dd.deviceString[:])
				n[`primary_device`] = wstr(dd.deviceName[:])
			}
		}
		n[`count`] = count
		n[`active`] = active
		if count == 0 {
			return schema.Fail(`no display devices enumerated`).With(schema.KeyValue, n)
		}
		return schema.OK(n)
	})
	timed(doc, secGraphics, `current_display_mode`, func() schema.Node {
		p := user32.NewProc(`EnumDisplaySettingsW`)
		var dm devMode
		dm.size = uint16(unsafe.Sizeof(dm))
		r, _, _ := p.Call(0, enumCurrentSettings, uintptr(unsafe.Pointer(&dm)))
		if r == 0 {
			return schema.Fail(`EnumDisplaySettingsW returned FALSE`)
		}
		n := schema.Node{}
		n[`width`] = dm.pelsWidth
		n[`height`] = dm.pelsHeight
		n[`bits_per_pixel`] = dm.bitsPerPel
		n[`refresh_hz`] = dm.displayFrequency
		return schema.OK(n)
	})
	timed(doc, secGraphics, `d3d11_device`, func() schema.Node {
		p := d3d11.NewProc(`D3D11CreateDevice`)
		if p.Find() != nil {
			return schema.Unsupported(`d3d11.dll or D3D11CreateDevice missing`)
		}
		hr, level := createD3D11(p, d3dDriverHardware)
		if !failed(hr) {
			return d3d11Node(level, `hardware`)
		}
		n := schema.Fail(`hardware device: HRESULT ` + hresultText(hr))
		if !opts.enabled(WorkaroundWarpFallback) {
			return n
		}
		hr2, level2 := createD3D11(p, d3dDriverWarp)
		if failed(hr2) {
			return n.With(`warp_fallback`, `HRESULT `+hresultText(hr2))
		}
		return d3d11Node(level2, `warp`).With(schema.KeySource, `fallback`).With(`workaround`, WorkaroundWarpFallback).With(`hardware_error`, hresultText(hr))
	})
	timed(doc, secGraphics, `dxgi_adapters`, func() schema.Node {
		return probeDXGI()
	})
	timed(doc, secGraphics, `d3d12_device`, func() schema.Node {
		p := d3d12.NewProc(`D3D12CreateDevice`)
		if p.Find() != nil {
			return schema.Unsupported(`d3d12.dll or D3D12CreateDevice missing`)
		}
		var dev uintptr
		hr, _, _ := p.Call(0, d3dFeatureLevel11, uintptr(unsafe.Pointer(&iidD3D12Device)), uintptr(unsafe.Pointer(&dev)))
		if failed(hr) {
			return schema.Fail(`HRESULT ` + hresultText(hr))
		}
		release(dev)
		return schema.OK(`feature level 11_0 device created`)
	})
	timed(doc, secGraphics, `vulkan`, func() schema.Node {
		return probeVulkan()
	})
}

func createD3D11(p *syscall.LazyProc, driverType uintptr) (uintptr, uint32) {
	var dev, ctx uintptr
	var level uint32
	hr, _, _ := p.Call(0, driverType, 0, 0, 0, 0, d3d11SDKVersion, uintptr(unsafe.Pointer(&dev)), uintptr(unsafe.Pointer(&level)), uintptr(unsafe.Pointer(&ctx)))
	release(ctx)
	release(dev)
	return hr, level
}

func d3d11Node(level uint32, driver string) schema.Node {
	n := schema.Node{}
	n[`driver_type`] = driver
	n[`feature_level`] = fmt.Sprintf(`0x%X`, level)
	n[`feature_level_name`] = featureLevelNames[level]
	return schema.OK(n)
}
