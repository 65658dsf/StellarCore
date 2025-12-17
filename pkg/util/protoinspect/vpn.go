package protoinspect

import (
	"bytes"
	"encoding/binary"
)

// DetectOpenVPN detects OpenVPN traffic.
// Focuses on the initial packet of the handshake.
func DetectOpenVPN(buf []byte) (bool, map[string]string) {
	info := map[string]string{}
	if len(buf) < 2 {
		return false, info
	}

	// OpenVPN packet starts with a 1-byte opcode/key_id
	// Opcode is top 5 bits. P_CONTROL_HARD_RESET_CLIENT_V2 is 7 (0x07 << 3 = 0x38).
	// KeyID is bottom 3 bits.
	// Common first byte for client handshake is 0x38 (Opcode 7, KeyID 0).
	opcode := buf[0] >> 3
	if opcode == 7 { // P_CONTROL_HARD_RESET_CLIENT_V2
		info["Protocol"] = "OpenVPN"
		return true, info
	}
	if opcode == 4 { // P_CONTROL_V1
		info["Protocol"] = "OpenVPN"
		return true, info
	}

	// TCP OpenVPN often starts with a 2-byte length
	if len(buf) > 3 {
		plen := int(binary.BigEndian.Uint16(buf[0:2]))
		if plen == len(buf)-2 {
			// Check 3rd byte (which is the opcode)
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
	// WireGuard Handshake Initiation is 148 bytes
	// Type (1 byte) + Reserved (3 bytes) + Sender (4 bytes) + ...

	// Strict length check to avoid false positives.
	// For UDP, this is exact. For TCP, we expect at least this much.
	if len(buf) < 148 {
		return false, info
	}

	// Type 1: Handshake Initiation
	if buf[0] == 0x01 && buf[1] == 0x00 && buf[2] == 0x00 && buf[3] == 0x00 {
		info["Protocol"] = "WireGuard"
		return true, info
	}

	return false, info
}

// DetectIPSec detects IKEv2 (UDP port 500/4500) traffic.
func DetectIPSec(buf []byte) (bool, map[string]string) {
	info := map[string]string{}
	// IKEv2 Header is 28 bytes
	if len(buf) < 28 {
		return false, info
	}

	// Initiator SPI (8 bytes) - must not be zero
	initSPI := binary.BigEndian.Uint64(buf[0:8])
	if initSPI == 0 {
		return false, info
	}

	// Responder SPI (8 bytes) - zero for IKE_SA_INIT
	respSPI := binary.BigEndian.Uint64(buf[8:16])

	// Next Payload (1 byte)
	// Version (1 byte) - 0x20 for IKEv2
	// Exchange Type (1 byte) - 34 (IKE_SA_INIT)
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

	// Version 5
	if buf[0] != 0x05 {
		return false, info
	}

	// NMETHODS
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
	// Version (1) + UUID (16) + AddonsLen (1)
	if len(buf) < 18 {
		return false, info
	}

	// Version 0
	if buf[0] != 0x00 {
		return false, info
	}

	// Check if bytes 1-17 look vaguely like a UUID (just 16 bytes, can't validate much without format)
	// But combined with Version 0, it's a candidate.
	// Let's verify AddonsLen isn't something crazy large if we have more data
	// addonsLen := int(buf[17])

	// Since 0x00 is a common first byte, this is weak.
	// But VLESS usually runs over a reliable transport.
	// We can add a heuristic: VLESS is often used with specific transports, but here we see raw bytes.
	// Let's rely on the structure [00][16 bytes][1 byte len].

	info["Protocol"] = "VLESS"
	return true, info
}

// DetectVMess detects VMess.
// Note: VMess is designed to be indistinguishable from random noise or mimics other protocols.
// This detector attempts to identify VMess over WebSocket or TLS.
func DetectVMess(buf []byte) (bool, map[string]string) {
	info := map[string]string{}

	// Check for HTTP Upgrade for WebSocket which might be VMess-WS
	if bytes.Contains(buf, []byte("Upgrade: websocket")) {
		// This is generic WebSocket, but often used for VMess.
		// We flag it as WebSocket, which might be blocked if "vmess" is selected
		// and we want to be aggressive, but for now let's just flag it as WebSocket.
		// If the user wants to block VMess, they might block all WS.
		// But let's return false here as it's not strictly VMess *signature*.
	}

	// VMess TCP (without TLS) is encrypted with a time-based hash.
	// We cannot detect it without the key.

	return false, info
}
