//go:build windows

package main

import (
	"fmt"
	"net"
	"time"
	"unsafe"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

const secNetwork = `network`

const (
	probeHost              = `example.com`
	probeHTTPS             = `example.com:443`
	reachIP                = `1.1.1.1`
	reachTCP               = `1.1.1.1:443`
	netTimeout             = 5 * time.Second
	icmpTimeoutMS          = 2000
	winhttpSecure          = 0x00800000
	winhttpQueryStatusCode = 19
	winhttpQueryFlagNumber = 0x20000000
	errorBufferOverflow    = 111
	afUnspec               = 0
	wsaVersion22           = 0x202
)

var networkProbes = []string{
	`winsock_startup`,
	`dns_resolve`,
	`tcp_connect_443`,
	`winhttp_get`,
	`icmp_echo`,
	`reachability`,
	`adapters`,
	`udp_bind_loopback`,
}

func probeNetwork(doc *schema.Doc, opts *options) {
	if opts.noNetwork {
		for _, name := range networkProbes {
			doc.Set(secNetwork, name, schema.Skipped(`-no-network`))
		}
		return
	}
	timed(doc, secNetwork, `winsock_startup`, func() schema.Node {
		var data [512]byte
		p := ws2_32.NewProc(`WSAStartup`)
		r, _, _ := p.Call(wsaVersion22, uintptr(unsafe.Pointer(&data[0])))
		if r != 0 {
			return schema.Fail(fmt.Sprintf(`WSAStartup error %d`, r))
		}
		n := schema.Node{}
		n[`version`] = fmt.Sprintf(`%d.%d`, data[0], data[1])
		n[`high_version`] = fmt.Sprintf(`%d.%d`, data[2], data[3])
		return schema.OK(n)
	})
	timed(doc, secNetwork, `dns_resolve`, func() schema.Node {
		addrs, err := net.LookupHost(probeHost)
		if err != nil {
			return schema.Fail(err.Error())
		}
		v4, v6 := 0, 0
		for _, a := range addrs {
			if ip := net.ParseIP(a); ip != nil && ip.To4() != nil {
				v4++
			} else {
				v6++
			}
		}
		n := schema.Node{}
		n[`host`] = probeHost
		n[`address_count`] = len(addrs)
		n[`ipv4`] = v4
		n[`ipv6`] = v6
		return schema.OK(n)
	})
	timed(doc, secNetwork, `tcp_connect_443`, func() schema.Node {
		c, err := net.DialTimeout(`tcp`, probeHTTPS, netTimeout)
		if err != nil {
			return schema.Fail(err.Error())
		}
		c.Close()
		return schema.OK(probeHTTPS)
	})
	timed(doc, secNetwork, `winhttp_get`, func() schema.Node {
		return probeWinHTTP()
	})
	icmpOK := false
	timed(doc, secNetwork, `icmp_echo`, func() schema.Node {
		n := probeICMP()
		icmpOK = n[schema.KeyStatus] == schema.StatusOK
		return n
	})
	timed(doc, secNetwork, `reachability`, func() schema.Node {
		if icmpOK {
			return schema.OK(reachIP).With(schema.KeySource, `icmp_echo`)
		}
		if !opts.enabled(WorkaroundTCPReach) {
			return schema.Fail(`icmp_echo failed and the tcp-reachability workaround is disabled`)
		}
		c, err := net.DialTimeout(`tcp`, reachTCP, netTimeout)
		if err != nil {
			return schema.Fail(`tcp fallback: ` + err.Error()).With(schema.KeySource, `tcp_fallback`)
		}
		c.Close()
		return schema.OK(reachTCP).With(schema.KeySource, `tcp_fallback`).With(`workaround`, WorkaroundTCPReach)
	})
	timed(doc, secNetwork, `adapters`, func() schema.Node {
		p := iphlpapi.NewProc(`GetAdaptersAddresses`)
		if p.Find() != nil {
			return schema.Unsupported(`GetAdaptersAddresses missing`)
		}
		var size uint32
		r, _, _ := p.Call(afUnspec, 0, 0, 0, uintptr(unsafe.Pointer(&size)))
		if r != errorBufferOverflow || size == 0 {
			return schema.Fail(fmt.Sprintf(`size query returned %d`, r))
		}
		buf := make([]byte, size)
		r, _, _ = p.Call(afUnspec, 0, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
		if r != 0 {
			return schema.Fail(fmt.Sprintf(`GetAdaptersAddresses error %d`, r))
		}
		count := 0
		next := uintptr(unsafe.Pointer(&buf[0]))
		for next != 0 && count < 256 {
			count++
			next = *(*uintptr)(unsafe.Pointer(next + 8))
		}
		n := schema.Node{}
		n[`count`] = count
		return schema.OK(n)
	})
	timed(doc, secNetwork, `udp_bind_loopback`, func() schema.Node {
		c, err := net.ListenPacket(`udp4`, `127.0.0.1:0`)
		if err != nil {
			return schema.Fail(err.Error())
		}
		c.Close()
		return schema.OK(true)
	})
}
