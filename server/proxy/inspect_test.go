package proxy

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	v1 "github.com/65658dsf/StellarCore/pkg/config/v1"
	"github.com/65658dsf/StellarCore/pkg/util/protoinspect"
)

func TestDetectRWCBlocksFragmentedStrongVPN(t *testing.T) {
	cases := []struct {
		name      string
		payload   []byte
		fragments []int
	}{
		{
			name:      "socks5",
			payload:   []byte{0x05, 0x01, 0x00},
			fragments: []int{2, 1},
		},
		{
			name:      "openvpn-tcp",
			payload:   []byte{0x00, 0x08, 0x38, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
			fragments: []int{3, 3, 4},
		},
		{
			name: "vless",
			payload: append([]byte{
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
			}, []byte("example.com")...),
			fragments: []int{5, 7, 6, 7, 9},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, client := net.Pipe()
			defer server.Close()
			defer client.Close()

			rwc := &detectRWC{
				c:         server,
				ctx:       contextBackground(),
				serverCfg: newTrafficMonitorServerConfig(),
				proxyName: "demo",
				runID:     "run-1",
			}

			go writeFragments(client, tc.payload, tc.fragments)

			buf := make([]byte, len(tc.payload))
			n, err := rwc.Read(buf)
			require.Equal(t, 0, n)
			require.ErrorIs(t, err, io.EOF)
		})
	}
}

func TestDetectRWCAllowsTLSCandidateAndReplaysBuffer(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	payload := captureTLSClientHelloForProxy(t, "example.com")
	rwc := &detectRWC{
		c:         server,
		ctx:       contextBackground(),
		serverCfg: newTrafficMonitorServerConfig(),
		proxyName: "demo",
		runID:     "run-2",
	}

	go func() {
		_, _ = client.Write(payload)
		_ = client.Close()
	}()

	buf := make([]byte, 128)
	var got []byte
	for {
		n, err := rwc.Read(buf)
		if n > 0 {
			got = append(got, buf[:n]...)
		}
		if err != nil {
			require.ErrorIs(t, err, io.EOF)
			break
		}
	}
	require.Equal(t, payload, got)
}

func TestDetectRWCRespondsToHTTPProbe(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	done := make(chan error, 1)
	go func() {
		server, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer server.Close()

		rwc := &detectRWC{
			c:         server,
			ctx:       contextBackground(),
			serverCfg: newTrafficMonitorServerConfig(),
			proxyName: "demo",
			runID:     "run-3",
		}

		buf := make([]byte, 128)
		_, err = rwc.Read(buf)
		done <- err
	}()

	client, err := net.Dial("tcp", listener.Addr().String())
	require.NoError(t, err)
	defer client.Close()

	_, err = client.Write([]byte("GET / HTTP/1.1\r\nHost: example.com\r\n\r\n"))
	require.NoError(t, err)

	buf := make([]byte, 256)
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := client.Read(buf)
	require.NoError(t, err)
	require.Contains(t, string(buf[:n]), "403")

	require.ErrorIs(t, <-done, io.EOF)
}

func TestShouldSkipTrafficInspection(t *testing.T) {
	cfg := newTrafficMonitorServerConfig()
	cfg.TrafficMonitor.WhitelistProxies = []string{"demo"}
	cfg.TrafficMonitor.WhitelistRunIDs = []string{"run-1"}

	require.True(t, shouldSkipTrafficInspection(cfg, "demo", ""))
	require.True(t, shouldSkipTrafficInspection(cfg, "other", "run-1"))
	require.False(t, shouldSkipTrafficInspection(cfg, "other", "run-2"))
}

func TestShouldBlockVPNDetectionUsesCanonicalProtocol(t *testing.T) {
	cfg := newTrafficMonitorServerConfig()
	cfg.TrafficMonitor.VPNProtocols = []string{"openvpn", "tuic"}

	require.True(t, shouldBlockVPNDetection(cfg, protoinspect.MatchDetection("openvpn", 100, false, []string{"openvpn_tcp_control"}, nil)))
	require.False(t, shouldBlockVPNDetection(cfg, protoinspect.MatchDetection("generic_quic", 60, true, []string{"quic_initial"}, nil)))
}

func newTrafficMonitorServerConfig() *v1.ServerConfig {
	cfg := &v1.ServerConfig{}
	cfg.Complete()
	cfg.TrafficMonitor.BlockVPN = lo.ToPtr(true)
	cfg.TrafficMonitor.InspectTimeoutMS = 80
	cfg.TrafficMonitor.InspectMaxBytes = 128
	return cfg
}

func writeFragments(conn net.Conn, payload []byte, fragments []int) {
	offset := 0
	for _, size := range fragments {
		end := offset + size
		if end > len(payload) {
			end = len(payload)
		}
		_, _ = conn.Write(payload[offset:end])
		offset = end
		time.Sleep(10 * time.Millisecond)
	}
	_ = conn.Close()
}

func captureTLSClientHelloForProxy(t *testing.T, serverName string) []byte {
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
			NextProtos:         []string{"h2"},
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

func contextBackground() context.Context {
	return context.Background()
}
