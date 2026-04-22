package protoinspect

// DetectQUIC detects QUIC packets (Long Header).
func DetectQUIC(buf []byte) (bool, map[string]string) {
	info := map[string]string{}
	if len(buf) < 1 {
		return false, info
	}

	firstByte := buf[0]
	if (firstByte & 0x80) != 0 {
		if (firstByte & 0x40) != 0 {
			info["Protocol"] = "QUIC"
			info["Type"] = "LongHeader"
			return true, info
		}
	}
	return false, info
}
