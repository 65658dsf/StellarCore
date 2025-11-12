package protoinspect

import (
    "crypto/tls"
    "net"
    "testing"
    "time"
)

func TestDetectTLSClientHello(t *testing.T) {
    l, err := net.Listen("tcp", "127.0.0.1:")
    if err != nil {
        t.Fatalf("listen error: %v", err)
    }
    defer l.Close()

    var serverConn net.Conn
    done := make(chan struct{})
    go func() {
        c, _ := l.Accept()
        serverConn = c
        close(done)
    }()

    go func() {
        time.Sleep(50 * time.Millisecond)
        _, _ = tls.Dial("tcp", l.Addr().String(), &tls.Config{InsecureSkipVerify: true, ServerName: "example.com"})
    }()

    <-done
    buf := make([]byte, 4096)
    serverConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
    n, _ := serverConn.Read(buf)
    _ = serverConn.Close()
    if n == 0 {
        t.Fatalf("no data read")
    }
    ok, sni, _ := DetectTLSClientHello(buf[:n])
    if !ok {
        t.Fatalf("expected tls clienthello detected")
    }
    if sni != "example.com" && sni != "" {
        t.Fatalf("unexpected sni: %s", sni)
    }
}
