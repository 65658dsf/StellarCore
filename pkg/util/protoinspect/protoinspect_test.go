package protoinspect

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/tls"
	"encoding/binary"
	"net"
	"testing"
	"time"

	quic "github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/quicvarint"
	"github.com/stretchr/testify/require"
)

func TestDetectHTTP(t *testing.T) {
	result := DetectHTTP([]byte("GET /path HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	require.True(t, result.Matched)
	require.Equal(t, "http", result.Protocol)
	require.Equal(t, "GET", result.Features["Method"])
}

func TestDetectTLSClientHello(t *testing.T) {
	data := captureTLSClientHello(t, "example.com", []string{"h2"})

	result := DetectTLSClientHello(data)
	require.True(t, result.Matched)
	require.Equal(t, "tls_candidate", result.Protocol)
	require.Equal(t, 60, result.Confidence)
	require.Equal(t, "example.com", result.Features["SNI"])
	require.Contains(t, result.Features["ALPN"], "h2")
}

func TestDetectOpenVPN(t *testing.T) {
	result := DetectOpenVPN([]byte{0x38, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07})
	require.True(t, result.Matched)
	require.Equal(t, "openvpn", result.Protocol)
	require.Equal(t, 100, result.Confidence)

	result = DetectOpenVPN([]byte{0x00, 0x08, 0x38, 0x01, 0x02, 0x03})
	require.True(t, result.Matched)
	require.True(t, result.NeedMoreData)
}

func TestDetectWireGuard(t *testing.T) {
	packet := make([]byte, 148)
	packet[0] = 0x01

	result := DetectWireGuard(packet)
	require.True(t, result.Matched)
	require.Equal(t, "wireguard", result.Protocol)
	require.Equal(t, 100, result.Confidence)

	result = DetectWireGuard(packet[:16])
	require.True(t, result.Matched)
	require.True(t, result.NeedMoreData)
}

func TestDetectIPSec(t *testing.T) {
	packet := make([]byte, 32)
	binary.BigEndian.PutUint64(packet[4:12], 1)
	packet[21] = 0x20
	packet[22] = 34
	binary.BigEndian.PutUint32(packet[28:32], 28)

	result := DetectIPSec(packet)
	require.True(t, result.Matched)
	require.Equal(t, "ikev2", result.Protocol)
	require.Equal(t, "true", result.Features["NATT"])
}

func TestDetectSOCKS5(t *testing.T) {
	result := DetectSOCKS5([]byte{0x05, 0x01, 0x00})
	require.True(t, result.Matched)
	require.Equal(t, "socks5", result.Protocol)
	require.Equal(t, 100, result.Confidence)

	result = DetectSOCKS5([]byte{0x05, 0x20, 0x00})
	require.False(t, result.Matched)
}

func TestDetectVLESS(t *testing.T) {
	packet := []byte{
		0x00,
		0x01, 0x02, 0x03, 0x04,
		0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0x0c,
		0x0d, 0x0e, 0x0f, 0x10,
		0x00,
		0x01,
		0x01, 0xbb,
		0x02,
		0x0b,
	}
	packet = append(packet, []byte("example.com")...)

	result := DetectVLESS(packet)
	require.True(t, result.Matched)
	require.Equal(t, "vless", result.Protocol)
	require.Equal(t, 100, result.Confidence)

	invalid := make([]byte, 18)
	require.False(t, DetectVLESS(invalid).Matched)
}

func TestDetectTrojan(t *testing.T) {
	packet := []byte("0123456789abcdef0123456789abcdef0123456789abcdef01234567\r\n")
	packet = append(packet, 0x01, 0x03, 0x0b)
	packet = append(packet, []byte("example.com")...)
	packet = append(packet, 0x01, 0xbb)
	packet = append(packet, '\r', '\n')

	result := DetectTrojan(packet)
	require.True(t, result.Matched)
	require.Equal(t, "trojan", result.Protocol)
	require.Equal(t, 100, result.Confidence)
}

func TestDetectQUIC(t *testing.T) {
	tuicPacket := buildQUICInitialPacket(t, "tuic.example", []string{"tuic"})
	result := DetectQUIC(tuicPacket)
	require.True(t, result.Matched)
	require.Equal(t, "tuic", result.Protocol)
	require.Equal(t, 100, result.Confidence)

	h3Packet := buildQUICInitialPacket(t, "h3.example", []string{"h3"})
	result = DetectQUIC(h3Packet)
	require.True(t, result.Matched)
	require.NotEqual(t, "tuic", result.Protocol)
	require.NotEqual(t, "hysteria2", result.Protocol)
	require.True(t, result.NeedMoreData)
}

func TestDetectTCPVPNPrefersStrongMatch(t *testing.T) {
	packet := []byte{0x05, 0x01, 0x00}
	result := DetectTCPVPN(packet)
	require.True(t, result.Matched)
	require.Equal(t, "socks5", result.Protocol)
}

func captureTLSClientHello(t *testing.T, serverName string, alpn []string) []byte {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	dataCh := make(chan []byte, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		buf := make([]byte, 4096)
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, _ := conn.Read(buf)
		dataCh <- append([]byte(nil), buf[:n]...)
	}()

	go func() {
		_, _ = tls.Dial("tcp", listener.Addr().String(), &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         serverName,
			NextProtos:         alpn,
		})
	}()

	select {
	case data := <-dataCh:
		return data
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for tls client hello")
		return nil
	}
}

func captureQUICInitialPacket(t *testing.T, serverName string, alpn []string) []byte {
	t.Helper()

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	defer conn.Close()

	dataCh := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 2048)
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		dataCh <- append([]byte(nil), buf[:n]...)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_, _ = quic.DialAddr(ctx, conn.LocalAddr().String(), &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         serverName,
		NextProtos:         alpn,
	}, &quic.Config{
		HandshakeIdleTimeout: 100 * time.Millisecond,
	})

	select {
	case data := <-dataCh:
		return data
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for quic initial packet")
		return nil
	}
}

func TestDetectTLSClientHelloPartial(t *testing.T) {
	result := DetectTLSClientHello([]byte{0x16, 0x03, 0x03})
	require.True(t, result.Matched)
	require.True(t, result.NeedMoreData)
}

func TestDetectQUICPartial(t *testing.T) {
	result := DetectQUIC([]byte{0xc0, 0x00, 0x00, 0x00})
	require.True(t, result.Matched)
	require.True(t, result.NeedMoreData)
}

func TestCaptureHelpersProduceData(t *testing.T) {
	require.NotEmpty(t, captureTLSClientHello(t, "bench.example", nil))
	require.NotEmpty(t, captureQUICInitialPacket(t, "bench.example", []string{"h3"}))
	require.NotEmpty(t, buildQUICInitialPacket(t, "bench.example", []string{"tuic"}))
}

func TestDetectHTTPDoesNotMatchBinaryPrefix(t *testing.T) {
	result := DetectHTTP([]byte{0x00, 0x01, 0x02, 0x03})
	require.False(t, result.Matched)
}

func TestDetectSOCKS5Request(t *testing.T) {
	packet := []byte{0x05, 0x01, 0x00, 0x03, 0x0b}
	packet = append(packet, []byte("example.com")...)
	packet = append(packet, 0x01, 0xbb)

	result := DetectSOCKS5(packet)
	require.True(t, result.Matched)
	require.Equal(t, "socks5", result.Protocol)
}

func TestDetectTrojanRejectsTLSClientHello(t *testing.T) {
	result := DetectTrojan(captureTLSClientHello(t, "trojan.example", nil))
	require.False(t, result.Matched)
}

func TestDetectQUICCapturedInitialFallsBackToCandidate(t *testing.T) {
	packet := captureQUICInitialPacket(t, "hy2.example", []string{"hy2"})
	result := DetectQUIC(packet)
	require.True(t, result.Matched)
	require.Equal(t, "generic_quic", result.Protocol)
}

func TestDetectIPSecWithoutNATT(t *testing.T) {
	packet := make([]byte, 28)
	binary.BigEndian.PutUint64(packet[0:8], 1)
	packet[17] = 0x20
	packet[18] = 34
	binary.BigEndian.PutUint32(packet[24:28], 28)

	result := DetectIPSec(packet)
	require.True(t, result.Matched)
	require.Equal(t, "ikev2", result.Protocol)
}

func TestDetectQUICInvalidPacketReturnsCandidate(t *testing.T) {
	result := DetectQUIC([]byte{0xc0, 0x00, 0x00, 0x00, 0x01, 0x08, 0x01, 0x01})
	require.True(t, result.Matched)
	require.Equal(t, "generic_quic", result.Protocol)
}

func TestDetectQUICSyntheticHysteria2(t *testing.T) {
	packet := buildQUICInitialPacket(t, "hy2.example", []string{"hy2"})
	result := DetectQUIC(packet)
	require.True(t, result.Matched)
	require.Equal(t, "hysteria2", result.Protocol)
}

func TestDetectTCPVPNAllowsTLSCandidate(t *testing.T) {
	result := DetectTCPVPN(captureTLSClientHello(t, "tls.example", []string{"h2"}))
	require.True(t, result.Matched)
	require.Equal(t, "tls_candidate", result.Protocol)
}

func buildQUICInitialPacket(t *testing.T, serverName string, alpn []string) []byte {
	t.Helper()

	record := captureTLSClientHello(t, serverName, alpn)
	require.Greater(t, len(record), 5)

	handshake := append([]byte(nil), record[5:]...)
	payload := append([]byte{0x06}, quicvarint.Append(nil, 0)...)
	payload = quicvarint.Append(payload, uint64(len(handshake)))
	payload = append(payload, handshake...)

	dcid := []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02, 0x03, 0x04}
	scid := []byte{0xca, 0xfe, 0xba, 0xbe, 0x05, 0x06, 0x07, 0x08}
	pnLen := 2
	headerLen := 1 + 4 + 1 + len(dcid) + 1 + len(scid) + 1 + 2
	minPayloadLen := 1200 - headerLen - pnLen - 16
	for len(payload) < minPayloadLen {
		payload = append(payload, 0x00)
	}

	key, iv, hpKey := deriveQUICInitialSecrets(quicVersion1, dcid)
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)

	firstByte := byte(0xc0 | byte(pnLen-1))
	header := []byte{firstByte}
	header = binary.BigEndian.AppendUint32(header, quicVersion1)
	header = append(header, byte(len(dcid)))
	header = append(header, dcid...)
	header = append(header, byte(len(scid)))
	header = append(header, scid...)
	header = append(header, 0x00)
	header = quicvarint.Append(header, uint64(pnLen+len(payload)+16))

	pnBytes := []byte{0x00, 0x00}
	aad := append(append([]byte(nil), header...), pnBytes...)
	nonce := append([]byte(nil), iv...)
	ciphertext := aead.Seal(nil, nonce, payload, aad)

	packet := append(append([]byte(nil), header...), pnBytes...)
	packet = append(packet, ciphertext...)

	hpBlock, err := aes.NewCipher(hpKey)
	require.NoError(t, err)
	sampleOffset := len(header) + 4
	mask := make([]byte, 16)
	hpBlock.Encrypt(mask, packet[sampleOffset:sampleOffset+16])
	packet[0] ^= mask[0] & 0x0f
	for i := 0; i < pnLen; i++ {
		packet[len(header)+i] ^= mask[i+1]
	}
	return packet
}
