package protoinspect

import (
	"bytes"
	"encoding/binary"
)

func DetectTCPVPN(buf []byte) DetectionResult {
	return BestMatch(
		DetectOpenVPN(buf),
		DetectWireGuard(buf),
		DetectSOCKS5(buf),
		DetectVLESS(buf),
		DetectTrojan(buf),
		DetectTLSClientHello(buf),
	)
}

func DetectUDPVPN(buf []byte) DetectionResult {
	return BestMatch(
		DetectWireGuard(buf),
		DetectOpenVPN(buf),
		DetectIPSec(buf),
		DetectVLESS(buf),
		DetectQUIC(buf),
	)
}

func DetectOpenVPN(buf []byte) DetectionResult {
	if len(buf) == 0 {
		return NoDetection()
	}

	opcode := buf[0] >> 3
	if isOpenVPNControlOpcode(opcode) {
		if len(buf) < 8 {
			return MatchDetection("openvpn", 60, true, []string{"openvpn_udp_prefix"}, nil)
		}
		return MatchDetection("openvpn", 100, false, []string{"openvpn_udp_control"}, map[string]string{
			"Transport": "udp",
		})
	}

	if len(buf) < 2 {
		return NoDetection()
	}
	payloadLen := int(binary.BigEndian.Uint16(buf[0:2]))
	if payloadLen < 8 {
		return NoDetection()
	}
	if len(buf) < 3 {
		return MatchDetection("openvpn", 60, true, []string{"openvpn_tcp_length_prefix"}, nil)
	}
	opcode = buf[2] >> 3
	if !isOpenVPNControlOpcode(opcode) {
		return NoDetection()
	}
	if len(buf) < 2+payloadLen {
		return MatchDetection("openvpn", 60, true, []string{"openvpn_tcp_truncated"}, map[string]string{
			"Transport": "tcp",
		})
	}
	return MatchDetection("openvpn", 100, false, []string{"openvpn_tcp_control"}, map[string]string{
		"Transport": "tcp",
	})
}

func DetectWireGuard(buf []byte) DetectionResult {
	if len(buf) < 4 {
		return NoDetection()
	}
	if buf[1] != 0x00 || buf[2] != 0x00 || buf[3] != 0x00 {
		return NoDetection()
	}

	expectedLen := 0
	reason := ""
	switch buf[0] {
	case 0x01:
		expectedLen = 148
		reason = "wireguard_handshake_initiation"
	case 0x02:
		expectedLen = 92
		reason = "wireguard_handshake_response"
	case 0x03:
		expectedLen = 64
		reason = "wireguard_cookie_reply"
	default:
		return NoDetection()
	}

	if len(buf) < expectedLen {
		return MatchDetection("wireguard", 60, true, []string{reason + "_truncated"}, nil)
	}
	return MatchDetection("wireguard", 100, false, []string{reason}, nil)
}

func DetectIPSec(buf []byte) DetectionResult {
	offset := 0
	if len(buf) >= 4 && bytes.Equal(buf[:4], []byte{0x00, 0x00, 0x00, 0x00}) {
		offset = 4
	}
	if len(buf) < offset+28 {
		if offset == 4 || len(buf) >= 16 {
			return MatchDetection("ikev2", 60, true, []string{"ikev2_truncated"}, nil)
		}
		return NoDetection()
	}

	header := buf[offset:]
	initSPI := binary.BigEndian.Uint64(header[0:8])
	respSPI := binary.BigEndian.Uint64(header[8:16])
	version := header[17]
	exchangeType := header[18]
	totalLen := int(binary.BigEndian.Uint32(header[24:28]))

	if initSPI == 0 || version != 0x20 || exchangeType != 34 || respSPI != 0 {
		return NoDetection()
	}
	if totalLen < 28 {
		return NoDetection()
	}
	if len(header) < totalLen {
		return MatchDetection("ikev2", 60, true, []string{"ikev2_length_truncated"}, nil)
	}

	features := map[string]string{}
	if offset == 4 {
		features["NATT"] = "true"
	}
	return MatchDetection("ikev2", 100, false, []string{"ikev2_sa_init"}, features)
}

func DetectSOCKS5(buf []byte) DetectionResult {
	if len(buf) == 0 || buf[0] != 0x05 {
		return NoDetection()
	}
	if len(buf) < 2 {
		return MatchDetection("socks5", 60, true, []string{"socks5_version_only"}, nil)
	}

	if greeting := detectSOCKS5Greeting(buf); greeting.Matched && !greeting.NeedMoreData {
		return greeting
	}
	if request := detectSOCKS5Request(buf); request.Matched {
		return request
	}
	return detectSOCKS5Greeting(buf)
}

func DetectVLESS(buf []byte) DetectionResult {
	if len(buf) == 0 || buf[0] != 0x00 {
		return NoDetection()
	}
	if len(buf) < 18 {
		return MatchDetection("vless", 60, true, []string{"vless_prefix_truncated"}, nil)
	}

	if bytes.Equal(buf[1:17], make([]byte, 16)) {
		return NoDetection()
	}

	addonsLen := int(buf[17])
	p := 18
	if len(buf) < p+addonsLen+4 {
		return MatchDetection("vless", 60, true, []string{"vless_addons_truncated"}, nil)
	}
	p += addonsLen

	command := buf[p]
	if command != 0x01 && command != 0x02 && command != 0x03 {
		return NoDetection()
	}
	p++

	port := binary.BigEndian.Uint16(buf[p : p+2])
	p += 2

	addrType := buf[p]
	p++
	addrLen, ok, needMore := parseVLESSAddressLength(buf[p:], addrType)
	if !ok {
		if needMore {
			return MatchDetection("vless", 60, true, []string{"vless_address_truncated"}, nil)
		}
		return NoDetection()
	}
	if len(buf[p:]) < addrLen {
		return MatchDetection("vless", 60, true, []string{"vless_address_truncated"}, nil)
	}

	return MatchDetection("vless", 100, false, []string{"vless_request"}, map[string]string{
		"Command": commandName(command),
		"Port":    uint16ToString(port),
	})
}

func DetectTrojan(buf []byte) DetectionResult {
	const passwordLen = 56
	if len(buf) == 0 {
		return NoDetection()
	}
	if len(buf) < passwordLen {
		if isASCIIHex(buf) {
			return MatchDetection("trojan", 60, true, []string{"trojan_password_truncated"}, nil)
		}
		return NoDetection()
	}
	if !isASCIIHex(buf[:passwordLen]) {
		return NoDetection()
	}
	if len(buf) < passwordLen+2 {
		return MatchDetection("trojan", 60, true, []string{"trojan_crlf_truncated"}, nil)
	}
	if !bytes.Equal(buf[passwordLen:passwordLen+2], []byte("\r\n")) {
		return NoDetection()
	}

	p := passwordLen + 2
	if len(buf) < p+4 {
		return MatchDetection("trojan", 60, true, []string{"trojan_request_truncated"}, nil)
	}

	command := buf[p]
	if command != 0x01 && command != 0x03 {
		return NoDetection()
	}
	p++

	addrType := buf[p]
	p++
	addrLen, ok, needMore := parseSOCKSAddressLength(buf[p:], addrType)
	if !ok {
		if needMore {
			return MatchDetection("trojan", 60, true, []string{"trojan_address_truncated"}, nil)
		}
		return NoDetection()
	}
	if len(buf[p:]) < addrLen+2 {
		return MatchDetection("trojan", 60, true, []string{"trojan_address_truncated"}, nil)
	}
	p += addrLen

	port := binary.BigEndian.Uint16(buf[p : p+2])
	p += 2
	if len(buf) < p+2 {
		return MatchDetection("trojan", 60, true, []string{"trojan_suffix_truncated"}, nil)
	}
	if !bytes.Equal(buf[p:p+2], []byte("\r\n")) {
		return NoDetection()
	}

	return MatchDetection("trojan", 100, false, []string{"trojan_request"}, map[string]string{
		"Command": commandName(command),
		"Port":    uint16ToString(port),
	})
}

func detectSOCKS5Greeting(buf []byte) DetectionResult {
	nmethods := int(buf[1])
	if nmethods <= 0 || nmethods > 16 {
		return NoDetection()
	}
	if len(buf) < 2+nmethods {
		return MatchDetection("socks5", 60, true, []string{"socks5_greeting_truncated"}, nil)
	}
	return MatchDetection("socks5", 100, false, []string{"socks5_greeting"}, map[string]string{
		"Methods": intToString(nmethods),
	})
}

func detectSOCKS5Request(buf []byte) DetectionResult {
	if len(buf) < 4 {
		return NoDetection()
	}
	command := buf[1]
	if command != 0x01 && command != 0x02 && command != 0x03 {
		return NoDetection()
	}
	if buf[2] != 0x00 {
		return NoDetection()
	}
	addrLen, ok, needMore := parseSOCKSAddressLength(buf[4:], buf[3])
	if !ok {
		if needMore {
			return MatchDetection("socks5", 60, true, []string{"socks5_request_truncated"}, nil)
		}
		return NoDetection()
	}
	if len(buf[4:]) < addrLen+2 {
		return MatchDetection("socks5", 60, true, []string{"socks5_request_truncated"}, nil)
	}

	portOffset := 4 + addrLen
	port := binary.BigEndian.Uint16(buf[portOffset : portOffset+2])
	return MatchDetection("socks5", 100, false, []string{"socks5_request"}, map[string]string{
		"Command": commandName(command),
		"Port":    uint16ToString(port),
	})
}

func parseSOCKSAddressLength(buf []byte, atyp byte) (int, bool, bool) {
	switch atyp {
	case 0x01:
		return 4, true, false
	case 0x03:
		if len(buf) < 1 {
			return 0, false, true
		}
		if buf[0] == 0 {
			return 0, false, false
		}
		return 1 + int(buf[0]), true, false
	case 0x04:
		return 16, true, false
	default:
		return 0, false, false
	}
}

func parseVLESSAddressLength(buf []byte, atyp byte) (int, bool, bool) {
	switch atyp {
	case 0x01:
		return 4, true, false
	case 0x02:
		if len(buf) < 1 {
			return 0, false, true
		}
		if buf[0] == 0 {
			return 0, false, false
		}
		return 1 + int(buf[0]), true, false
	case 0x03:
		return 16, true, false
	default:
		return 0, false, false
	}
}

func isOpenVPNControlOpcode(opcode byte) bool {
	return opcode == 4 || opcode == 7
}

func isASCIIHex(buf []byte) bool {
	if len(buf) == 0 {
		return false
	}
	for _, b := range buf {
		switch {
		case b >= '0' && b <= '9':
		case b >= 'a' && b <= 'f':
		case b >= 'A' && b <= 'F':
		default:
			return false
		}
	}
	return true
}

func commandName(command byte) string {
	switch command {
	case 0x01:
		return "connect"
	case 0x02:
		return "udp"
	case 0x03:
		return "mux"
	default:
		return "unknown"
	}
}
