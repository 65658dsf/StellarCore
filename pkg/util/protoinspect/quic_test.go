package protoinspect

import (
	"testing"
)

func TestDetectQUIC(t *testing.T) {
	// Long Header: 1xxx xxxx
	// Fixed Bit: x1xx xxxx
	// 0xC0 = 1100 0000
	packet := []byte{0xC0, 0x00, 0x00, 0x01}
	
	match, info := DetectQUIC(packet)
	if !match {
		t.Error("Expected QUIC detection")
	}
	if info["Protocol"] != "QUIC" {
		t.Errorf("Expected Protocol QUIC, got %v", info["Protocol"])
	}

	// Short Header (0xxx xxxx) - currently not detected as strict positive
	packetShort := []byte{0x40} 
	match, _ = DetectQUIC(packetShort)
	if match {
		t.Error("Expected no detection for Short Header (ambiguous)")
	}
}
