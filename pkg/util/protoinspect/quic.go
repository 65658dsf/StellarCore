package protoinspect

// DetectQUIC detects QUIC packets (Long Header).
// Hysteria 2 and other UDP protocols often base themselves on QUIC or mimic it.
func DetectQUIC(buf []byte) (bool, map[string]string) {
	info := map[string]string{}
	if len(buf) < 1 {
		return false, info
	}

	// RFC 9000:
	// Long Header: First bit is 1.
	// Fixed Bit: Second bit is 1 (for v1).
	// Short Header: First bit is 0.
	
	firstByte := buf[0]
	
	// Check for Long Header (Common for initial handshake)
	// 0x80 = 1000 0000
	// 0x40 = 0100 0000 (Fixed bit)
	if (firstByte & 0x80) != 0 {
		// Is Long Header
		if (firstByte & 0x40) != 0 {
			// Fixed bit is 1, standard QUIC v1
			info["Protocol"] = "QUIC"
			info["Type"] = "LongHeader"
			return true, info
		}
	}

	// Short Header (Data phase)
	// 0xxx xxxx
	// It's harder to distinguish short header from random UDP without context.
	// But usually we care about the handshake (Long Header).

	return false, info
}
