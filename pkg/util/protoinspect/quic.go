package protoinspect

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"strings"

	"github.com/quic-go/quic-go/quicvarint"
	"golang.org/x/crypto/hkdf"
)

const (
	quicVersion1 = 0x00000001
	quicVersion2 = 0x6b3343cf
)

var (
	quicSaltV1 = []byte{0x38, 0x76, 0x2c, 0xf7, 0xf5, 0x59, 0x34, 0xb3, 0x4d, 0x17, 0x9a, 0xe6, 0xa4, 0xc8, 0x0c, 0xad, 0xcc, 0xbb, 0x7f, 0x0a}
	quicSaltV2 = []byte{0x0d, 0xed, 0xe3, 0xde, 0xf7, 0x00, 0xa6, 0xdb, 0x81, 0x93, 0x81, 0xbe, 0x6e, 0x26, 0x9d, 0xcb, 0xf9, 0xbd, 0x2e, 0xd9}
)

func DetectQUIC(buf []byte) DetectionResult {
	if len(buf) == 0 {
		return NoDetection()
	}
	if (buf[0]&0x80) == 0 || (buf[0]&0x40) == 0 {
		return NoDetection()
	}
	if len(buf) < 5 {
		return MatchDetection("generic_quic", 60, true, []string{"quic_long_header_prefix"}, nil)
	}

	version := binary.BigEndian.Uint32(buf[1:5])
	features := map[string]string{
		"Version": uint32ToString(version),
	}

	info, err := parseQUICInitialClientHello(buf)
	switch {
	case err == nil:
		if info.SNI != "" {
			features["SNI"] = info.SNI
		}
		if len(info.ALPNs) > 0 {
			features["ALPN"] = strings.Join(info.ALPNs, ",")
		}
		if containsExact(info.ALPNs, "tuic") {
			return MatchDetection("tuic", 100, false, []string{"quic_alpn_tuic"}, features)
		}
		if containsHysteriaHint(info.ALPNs, info.SNI) {
			return MatchDetection("hysteria2", 100, false, []string{"quic_alpn_hysteria"}, features)
		}
		if len(info.ALPNs) > 0 || info.SNI != "" {
			return MatchDetection("quic_tunnel_candidate", 60, true, []string{"quic_client_hello"}, features)
		}
		return MatchDetection("generic_quic", 60, true, []string{"quic_initial"}, features)
	case errors.Is(err, errNeedMoreData):
		return MatchDetection("generic_quic", 60, true, []string{"quic_initial_truncated"}, features)
	default:
		return MatchDetection("generic_quic", 60, true, []string{"quic_long_header"}, features)
	}
}

type quicClientHelloInfo struct {
	SNI   string
	ALPNs []string
}

func parseQUICInitialClientHello(packet []byte) (quicClientHelloInfo, error) {
	parsed, err := parseQUICLongHeader(packet)
	if err != nil {
		return quicClientHelloInfo{}, err
	}

	firstByte, pnBytes, err := removeQUICHeaderProtection(packet, parsed)
	if err != nil {
		return quicClientHelloInfo{}, err
	}
	pnLen := int(firstByte&0x03) + 1
	packetNumber := readPacketNumber(pnBytes[:pnLen])
	headerLen := parsed.headerLen + pnLen

	if len(packet) < headerLen {
		return quicClientHelloInfo{}, errNeedMoreData
	}
	totalLen := parsed.headerLen + int(parsed.length)
	if len(packet) < totalLen {
		return quicClientHelloInfo{}, errNeedMoreData
	}
	if totalLen < headerLen+16 {
		return quicClientHelloInfo{}, errNeedMoreData
	}

	aad := append([]byte(nil), packet[:headerLen]...)
	aad[0] = firstByte
	copy(aad[parsed.headerLen:headerLen], pnBytes[:pnLen])

	decrypted, err := openQUICInitialPayload(parsed, packetNumber, aad, packet[headerLen:totalLen])
	if err != nil {
		return quicClientHelloInfo{}, err
	}

	cryptoPayload, err := collectInitialCryptoFrames(decrypted)
	if err != nil {
		return quicClientHelloInfo{}, err
	}
	info, err := parseTLSClientHelloHandshake(cryptoPayload)
	if err != nil {
		return quicClientHelloInfo{}, err
	}
	return quicClientHelloInfo{
		SNI:   info.SNI,
		ALPNs: info.ALPNs,
	}, nil
}

type quicLongHeader struct {
	firstByte byte
	version   uint32
	dcid      []byte
	headerLen int
	length    uint64
}

func parseQUICLongHeader(packet []byte) (quicLongHeader, error) {
	if len(packet) < 6 {
		return quicLongHeader{}, errNeedMoreData
	}
	firstByte := packet[0]
	version := binary.BigEndian.Uint32(packet[1:5])
	if version == 0 {
		return quicLongHeader{}, errors.New("version negotiation packet")
	}
	if !isQUICInitialType(firstByte, version) {
		return quicLongHeader{}, errors.New("not an initial packet")
	}

	p := 5
	dcidLen := int(packet[p])
	p++
	if len(packet) < p+dcidLen+1 {
		return quicLongHeader{}, errNeedMoreData
	}
	dcid := append([]byte(nil), packet[p:p+dcidLen]...)
	p += dcidLen

	scidLen := int(packet[p])
	p++
	if len(packet) < p+scidLen {
		return quicLongHeader{}, errNeedMoreData
	}
	p += scidLen

	tokenLen, n, err := quicvarint.Parse(packet[p:])
	if err != nil {
		return quicLongHeader{}, errNeedMoreData
	}
	p += n
	if len(packet) < p+int(tokenLen) {
		return quicLongHeader{}, errNeedMoreData
	}
	p += int(tokenLen)

	length, n, err := quicvarint.Parse(packet[p:])
	if err != nil {
		return quicLongHeader{}, errNeedMoreData
	}
	p += n

	return quicLongHeader{
		firstByte: firstByte,
		version:   version,
		dcid:      dcid,
		headerLen: p,
		length:    length,
	}, nil
}

func removeQUICHeaderProtection(packet []byte, parsed quicLongHeader) (byte, [4]byte, error) {
	if len(packet) < parsed.headerLen+4+16 {
		return 0, [4]byte{}, errNeedMoreData
	}

	_, _, hpKey := deriveQUICInitialSecrets(parsed.version, parsed.dcid)
	block, err := aes.NewCipher(hpKey)
	if err != nil {
		return 0, [4]byte{}, err
	}

	sample := packet[parsed.headerLen+4 : parsed.headerLen+4+16]
	mask := make([]byte, 16)
	block.Encrypt(mask, sample)

	firstByte := packet[0] ^ (mask[0] & 0x0f)
	var pnBytes [4]byte
	copy(pnBytes[:], packet[parsed.headerLen:parsed.headerLen+4])
	pnLen := int(firstByte&0x03) + 1
	for i := 0; i < pnLen; i++ {
		pnBytes[i] ^= mask[i+1]
	}
	return firstByte, pnBytes, nil
}

func openQUICInitialPayload(parsed quicLongHeader, packetNumber uint64, aad []byte, ciphertext []byte) ([]byte, error) {
	key, iv, _ := deriveQUICInitialSecrets(parsed.version, parsed.dcid)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, len(iv))
	copy(nonce, iv)
	for i := 0; i < 8; i++ {
		nonce[len(nonce)-1-i] ^= byte(packetNumber >> (8 * i))
	}
	return aead.Open(nil, nonce, ciphertext, aad)
}

func collectInitialCryptoFrames(payload []byte) ([]byte, error) {
	cryptoData := make([]byte, 0, 1024)
	p := 0
	for p < len(payload) {
		frameType, n, err := quicvarint.Parse(payload[p:])
		if err != nil {
			return nil, errNeedMoreData
		}
		if frameType == 0x00 {
			p++
			continue
		}
		p += n
		switch frameType {
		case 0x01: // PING
			continue
		case 0x06: // CRYPTO
			offset, n, err := quicvarint.Parse(payload[p:])
			if err != nil {
				return nil, errNeedMoreData
			}
			p += n
			length, n, err := quicvarint.Parse(payload[p:])
			if err != nil {
				return nil, errNeedMoreData
			}
			p += n
			if len(payload) < p+int(length) {
				return nil, errNeedMoreData
			}
			if offset <= uint64(len(cryptoData)) {
				end := int(offset) + int(length)
				if end > len(cryptoData) {
					newBuf := make([]byte, end)
					copy(newBuf, cryptoData)
					cryptoData = newBuf
				}
				copy(cryptoData[int(offset):end], payload[p:p+int(length)])
			}
			p += int(length)
		default:
			if len(cryptoData) > 0 {
				return cryptoData, nil
			}
			return nil, errors.New("unsupported frame in initial packet")
		}
	}
	if len(cryptoData) == 0 {
		return nil, errors.New("no crypto frame found")
	}
	return cryptoData, nil
}

func deriveQUICInitialSecrets(version uint32, dcid []byte) (key []byte, iv []byte, hp []byte) {
	salt := quicSaltV1
	keyLabel := "quic key"
	ivLabel := "quic iv"
	hpLabel := "quic hp"
	if version == quicVersion2 {
		salt = quicSaltV2
		keyLabel = "quicv2 key"
		ivLabel = "quicv2 iv"
		hpLabel = "quicv2 hp"
	}

	initialSecret := hkdf.Extract(crypto.SHA256.New, dcid, salt)
	clientSecret := hkdfExpandLabel(crypto.SHA256, initialSecret, nil, "client in", crypto.SHA256.Size())
	key = hkdfExpandLabel(crypto.SHA256, clientSecret, nil, keyLabel, 16)
	iv = hkdfExpandLabel(crypto.SHA256, clientSecret, nil, ivLabel, 12)
	hp = hkdfExpandLabel(crypto.SHA256, clientSecret, nil, hpLabel, 16)
	return
}

func hkdfExpandLabel(hash crypto.Hash, secret, context []byte, label string, length int) []byte {
	header := make([]byte, 3, 3+6+len(label)+1+len(context))
	binary.BigEndian.PutUint16(header, uint16(length))
	header[2] = uint8(6 + len(label))
	header = append(header, []byte("tls13 ")...)
	header = append(header, []byte(label)...)
	header = header[:3+6+len(label)+1]
	header[3+6+len(label)] = uint8(len(context))
	header = append(header, context...)

	out := make([]byte, length)
	n, err := hkdf.Expand(hash.New, secret, header).Read(out)
	if err != nil || n != length {
		panic("quic hkdf expand failed")
	}
	return out
}

func isQUICInitialType(firstByte byte, version uint32) bool {
	packetType := (firstByte >> 4) & 0x03
	if version == quicVersion2 {
		return packetType == 0x01
	}
	return packetType == 0x00
}

func readPacketNumber(buf []byte) uint64 {
	var value uint64
	for _, b := range buf {
		value = (value << 8) | uint64(b)
	}
	return value
}

func containsExact(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func containsHysteriaHint(alpns []string, sni string) bool {
	for _, value := range alpns {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "hy2") || strings.Contains(lower, "hysteria") {
			return true
		}
	}
	lowerSNI := strings.ToLower(sni)
	return strings.Contains(lowerSNI, "hy2") || strings.Contains(lowerSNI, "hysteria")
}
