package protoinspect

import (
	"encoding/binary"
	"errors"
	"strings"
)

var errNeedMoreData = errors.New("need more data")

type clientHelloInfo struct {
	SNI   string
	ALPNs []string
}

func DetectTLSClientHello(buf []byte) DetectionResult {
	if len(buf) == 0 {
		return NoDetection()
	}
	if buf[0] != 0x16 {
		return NoDetection()
	}
	if len(buf) < 5 {
		return MatchDetection("tls_candidate", 60, true, []string{"tls_record_prefix"}, nil)
	}
	if buf[1] != 0x03 {
		return NoDetection()
	}

	recordLen := int(binary.BigEndian.Uint16(buf[3:5]))
	available := len(buf) - 5
	if available < recordLen {
		return MatchDetection("tls_candidate", 60, true, []string{"tls_record_truncated"}, map[string]string{
			"RecordLength": intToString(recordLen),
		})
	}

	info, err := parseTLSClientHelloHandshake(buf[5 : 5+recordLen])
	if err != nil {
		if errors.Is(err, errNeedMoreData) {
			return MatchDetection("tls_candidate", 60, true, []string{"tls_handshake_truncated"}, nil)
		}
		return NoDetection()
	}

	features := map[string]string{}
	if info.SNI != "" {
		features["SNI"] = info.SNI
	}
	if len(info.ALPNs) > 0 {
		features["ALPN"] = strings.Join(info.ALPNs, ",")
	}
	return MatchDetection("tls_candidate", 60, true, []string{"tls_client_hello"}, features)
}

func parseTLSClientHelloHandshake(buf []byte) (clientHelloInfo, error) {
	if len(buf) < 4 {
		return clientHelloInfo{}, errNeedMoreData
	}
	if buf[0] != 0x01 {
		return clientHelloInfo{}, errors.New("not a client hello")
	}

	hsLen := int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
	if len(buf) < 4+hsLen {
		return clientHelloInfo{}, errNeedMoreData
	}

	hs := buf[4 : 4+hsLen]
	p := 0
	if !ensureTLSBounds(hs, p, 2) {
		return clientHelloInfo{}, errNeedMoreData
	}
	p += 2 // legacy_version
	if !ensureTLSBounds(hs, p, 32) {
		return clientHelloInfo{}, errNeedMoreData
	}
	p += 32 // random
	if !ensureTLSBounds(hs, p, 1) {
		return clientHelloInfo{}, errNeedMoreData
	}
	sessionIDLen := int(hs[p])
	p++
	if !ensureTLSBounds(hs, p, sessionIDLen) {
		return clientHelloInfo{}, errNeedMoreData
	}
	p += sessionIDLen

	if !ensureTLSBounds(hs, p, 2) {
		return clientHelloInfo{}, errNeedMoreData
	}
	cipherSuitesLen := int(binary.BigEndian.Uint16(hs[p : p+2]))
	p += 2
	if !ensureTLSBounds(hs, p, cipherSuitesLen) {
		return clientHelloInfo{}, errNeedMoreData
	}
	p += cipherSuitesLen

	if !ensureTLSBounds(hs, p, 1) {
		return clientHelloInfo{}, errNeedMoreData
	}
	compressionMethodsLen := int(hs[p])
	p++
	if !ensureTLSBounds(hs, p, compressionMethodsLen) {
		return clientHelloInfo{}, errNeedMoreData
	}
	p += compressionMethodsLen

	if p == len(hs) {
		return clientHelloInfo{}, nil
	}
	if !ensureTLSBounds(hs, p, 2) {
		return clientHelloInfo{}, errNeedMoreData
	}
	extensionsLen := int(binary.BigEndian.Uint16(hs[p : p+2]))
	p += 2
	if !ensureTLSBounds(hs, p, extensionsLen) {
		return clientHelloInfo{}, errNeedMoreData
	}

	info := clientHelloInfo{}
	end := p + extensionsLen
	for p+4 <= end {
		extType := binary.BigEndian.Uint16(hs[p : p+2])
		extLen := int(binary.BigEndian.Uint16(hs[p+2 : p+4]))
		p += 4
		if !ensureTLSBounds(hs, p, extLen) {
			return clientHelloInfo{}, errNeedMoreData
		}
		extBody := hs[p : p+extLen]
		switch extType {
		case 0x0000:
			info.SNI = parseSNI(extBody)
		case 0x0010:
			info.ALPNs = parseALPN(extBody)
		}
		p += extLen
	}
	return info, nil
}

func parseSNI(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	listLen := int(binary.BigEndian.Uint16(b[0:2]))
	p := 2
	if p+listLen > len(b) {
		listLen = len(b) - p
	}
	end := p + listLen
	for p+3 <= end {
		nameType := b[p]
		nameLen := int(binary.BigEndian.Uint16(b[p+1 : p+3]))
		p += 3
		if p+nameLen > end {
			break
		}
		if nameType == 0 {
			return string(b[p : p+nameLen])
		}
		p += nameLen
	}
	return ""
}

func parseALPN(b []byte) []string {
	if len(b) < 2 {
		return nil
	}
	listLen := int(binary.BigEndian.Uint16(b[0:2]))
	p := 2
	if p+listLen > len(b) {
		listLen = len(b) - p
	}
	end := p + listLen
	values := make([]string, 0, 2)
	for p < end {
		if p+1 > end {
			break
		}
		valueLen := int(b[p])
		p++
		if p+valueLen > end {
			break
		}
		values = append(values, string(b[p:p+valueLen]))
		p += valueLen
	}
	return values
}

func ensureTLSBounds(buf []byte, offset int, size int) bool {
	return offset >= 0 && size >= 0 && offset+size <= len(buf)
}

func intToString(v int) string {
	return strconvItoa(v)
}
