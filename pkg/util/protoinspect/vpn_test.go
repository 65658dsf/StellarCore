package protoinspect

import (
	"encoding/hex"
	"testing"
)

func TestDetectOpenVPN(t *testing.T) {
	// P_CONTROL_HARD_RESET_CLIENT_V2 (opcode 7)
	// 0x38 = 0011 1000 (Opcode 7, KeyID 0)
	packet, _ := hex.DecodeString("3801020304")
	match, info := DetectOpenVPN(packet)
	if !match {
		t.Error("Expected OpenVPN detection")
	}
	if info["Protocol"] != "OpenVPN" {
		t.Errorf("Expected Protocol OpenVPN, got %v", info["Protocol"])
	}

	// TCP length prefix
	// Length 4 (00 04) followed by 4 bytes (38 01 02 03)
	packetTCP, _ := hex.DecodeString("000438010203")
	match, info = DetectOpenVPN(packetTCP)
	if !match {
		t.Error("Expected OpenVPN-TCP detection")
	}
	if info["Protocol"] != "OpenVPN-TCP" {
		t.Errorf("Expected Protocol OpenVPN-TCP, got %v", info["Protocol"])
	}
}

func TestDetectWireGuard(t *testing.T) {
	// Type 1, Reserved 000000
	// 148 bytes total
	buf := make([]byte, 148)
	buf[0] = 0x01
	
	match, info := DetectWireGuard(buf)
	if !match {
		t.Error("Expected WireGuard detection")
	}
	if info["Protocol"] != "WireGuard" {
		t.Errorf("Expected Protocol WireGuard, got %v", info["Protocol"])
	}

	// Short packet
	match, _ = DetectWireGuard(buf[:10])
	if match {
		t.Error("Expected no detection for short packet")
	}
}

func TestDetectSOCKS5(t *testing.T) {
	// Ver 5, 1 Method, Method 00
	packet, _ := hex.DecodeString("050100")
	match, info := DetectSOCKS5(packet)
	if !match {
		t.Error("Expected SOCKS5 detection")
	}
	if info["Protocol"] != "SOCKS5" {
		t.Errorf("Expected Protocol SOCKS5, got %v", info["Protocol"])
	}
}

func TestDetectVLESS(t *testing.T) {
	// Ver 00, UUID (16 bytes), AddonsLen
	packet := make([]byte, 18)
	packet[0] = 0x00
	// UUID bytes...
	packet[17] = 0x00 // Addons len

	match, info := DetectVLESS(packet)
	if !match {
		t.Error("Expected VLESS detection")
	}
	if info["Protocol"] != "VLESS" {
		t.Errorf("Expected Protocol VLESS, got %v", info["Protocol"])
	}
}
