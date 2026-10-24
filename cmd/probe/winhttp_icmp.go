//go:build windows

package main

import (
	"fmt"
	"net"
	"unsafe"

	"github.com/BunsDev/wine-compat-probe/internal/schema"
)

func probeWinHTTP() schema.Node {
	open := winhttp.NewProc(`WinHttpOpen`)
	if open.Find() != nil {
		return schema.Unsupported(`winhttp.dll or WinHttpOpen missing`)
	}
	connect := winhttp.NewProc(`WinHttpConnect`)
	openReq := winhttp.NewProc(`WinHttpOpenRequest`)
	send := winhttp.NewProc(`WinHttpSendRequest`)
	recv := winhttp.NewProc(`WinHttpReceiveResponse`)
	query := winhttp.NewProc(`WinHttpQueryHeaders`)
	closeH := winhttp.NewProc(`WinHttpCloseHandle`)
	session, _, err := open.Call(utf16(toolVersion), 0, 0, 0, 0)
	if session == 0 {
		return schema.Fail(`WinHttpOpen: ` + errText(err))
	}
	defer closeH.Call(session)
	conn, _, err := connect.Call(session, utf16(probeHost), 443, 0)
	if conn == 0 {
		return schema.Fail(`WinHttpConnect: ` + errText(err))
	}
	defer closeH.Call(conn)
	req, _, err := openReq.Call(conn, utf16(`GET`), utf16(`/`), 0, 0, 0, winhttpSecure)
	if req == 0 {
		return schema.Fail(`WinHttpOpenRequest: ` + errText(err))
	}
	defer closeH.Call(req)
	if r, _, err := send.Call(req, 0, 0, 0, 0, 0, 0); r == 0 {
		return schema.Fail(`WinHttpSendRequest: ` + errText(err))
	}
	if r, _, err := recv.Call(req, 0); r == 0 {
		return schema.Fail(`WinHttpReceiveResponse: ` + errText(err))
	}
	var code, size uint32 = 0, 4
	if r, _, err := query.Call(req, winhttpQueryStatusCode|winhttpQueryFlagNumber, 0, uintptr(unsafe.Pointer(&code)), uintptr(unsafe.Pointer(&size)), 0); r == 0 {
		return schema.Fail(`WinHttpQueryHeaders: ` + errText(err))
	}
	n := schema.Node{}
	n[`url`] = `https://` + probeHost + `/`
	n[`status_code`] = code
	return schema.OK(n)
}

func probeICMP() schema.Node {
	createFile := iphlpapi.NewProc(`IcmpCreateFile`)
	sendEcho := iphlpapi.NewProc(`IcmpSendEcho`)
	closeH := iphlpapi.NewProc(`IcmpCloseHandle`)
	if createFile.Find() != nil || sendEcho.Find() != nil {
		return schema.Unsupported(`iphlpapi ICMP exports missing`)
	}
	h, _, err := createFile.Call()
	if h == 0 || h == ^uintptr(0) {
		return schema.Fail(`IcmpCreateFile: ` + errText(err))
	}
	defer closeH.Call(h)
	ip := net.ParseIP(reachIP).To4()
	dest := uint32(ip[0]) | uint32(ip[1])<<8 | uint32(ip[2])<<16 | uint32(ip[3])<<24
	payload := []byte(probePayload)
	reply := make([]byte, 28+len(payload)+64)
	r, _, err := sendEcho.Call(h, uintptr(dest), uintptr(unsafe.Pointer(&payload[0])), uintptr(len(payload)), 0, uintptr(unsafe.Pointer(&reply[0])), uintptr(len(reply)), icmpTimeoutMS)
	if r == 0 {
		return schema.Fail(`IcmpSendEcho: ` + errText(err))
	}
	status := le32(reply[4:8])
	rtt := le32(reply[8:12])
	n := schema.Node{}
	n[`target`] = reachIP
	n[`reply_status`] = status
	n[`rtt_ms`] = rtt
	if status != 0 {
		return schema.Fail(fmt.Sprintf(`echo reply status %d`, status)).With(schema.KeyValue, n)
	}
	return schema.OK(n)
}
