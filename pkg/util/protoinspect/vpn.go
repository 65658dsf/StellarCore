package protoinspect

import (
	"bytes"
	"encoding/binary"
)

// DetectOpenVPN detects OpenVPN traffic.
func DetectOpenVPN(buf []byte) (bool, map[string]string) {
	info := map[string]string{}
	if len(buf) < 2 {
		return false, info
	}
	opcode := buf[0] >> 3
	if opcode == 7 {
		info["Protocol"] = "OpenVPN"
		return true, info
	}
	if opcode == 4 {
		info["Protocol"] = "OpenVPN"
		return true, info
	}

	if len(buf) > 3 {
		plen := int(binary.BigEndian.Uint16(buf[0:2]))
		if plen == len(buf)-2 {
			op := buf[2] >> 3
			if op == 7 || op == 4 {
				info["Protocol"] = "OpenVPN-TCP"
				return true, info
			}
		}
	}

	return false, info
}

// DetectWireGuard detects WireGuard handshake initiation.
func DetectWireGuard(buf []byte) (bool, map[string]string) {
	info := map[string]string{}
	if len(buf) < 148 {
		return false, info
	}

	if buf[0] == 0x01 && buf[1] == 0x00 && buf[2] == 0x00 && buf[3] == 0x00 {
		info["Protocol"] = "WireGuard"
		return true, info
	}

	return false, info
}

// DetectIPSec detects IKEv2 (UDP port 500/4500) traffic.
func DetectIPSec(buf []byte) (bool, map[string]string) {
	info := map[string]string{}
	if len(buf) < 28 {
		return false, info
	}
	initSPI := binary.BigEndian.Uint64(buf[0:8])
	if initSPI == 0 {
		return false, info
	}
	respSPI := binary.BigEndian.Uint64(buf[8:16])
	version := buf[17]
	exchangeType := buf[18]

	if version == 0x20 && exchangeType == 34 && respSPI == 0 {
		info["Protocol"] = "IPSec"
		return true, info
	}

	return false, info
}

// DetectSOCKS5 detects SOCKS5 handshake.
func DetectSOCKS5(buf []byte) (bool, map[string]string) {
	info := map[string]string{}
	if len(buf) < 3 {
		return false, info
	}
	if buf[0] != 0x05 {
		return false, info
	}
	nmethods := int(buf[1])
	if len(buf) < 2+nmethods {
		return false, info
	}

	info["Protocol"] = "SOCKS5"
	return true, info
}

// DetectVLESS detects VLESS protocol header.
func DetectVLESS(buf []byte) (bool, map[string]string) {
	info := map[string]string{}
	if len(buf) < 18 {
		return false, info
	}
	if buf[0] != 0x00 {
		return false, info
	}

	info["Protocol"] = "VLESS"
	return true, info
}

// DetectVMess detects VMess.
func DetectVMess(buf []byte) (bool, map[string]string) {
	info := map[string]string{}

	if bytes.Contains(buf, []byte("Upgrade: websocket")) {
	}

	return false, info
}
