package protoinspect

import (
    "crypto/tls"
    "net"
    "testing"
    "time"
)

func BenchmarkDetectHTTP(b *testing.B) {
    buf := []byte("GET /a HTTP/1.1\r\nHost: x\r\n\r\n")
    for i := 0; i < b.N; i++ {
        DetectHTTP(buf)
    }
}

func BenchmarkDetectTLSClientHello(b *testing.B) {
    l, _ := net.Listen("tcp", "127.0.0.1:")
    defer l.Close()
    done := make(chan []byte, 1)
    go func() {
        c, _ := l.Accept()
        buf := make([]byte, 4096)
        c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
        n, _ := c.Read(buf)
        _ = c.Close()
        done <- buf[:n]
    }()
    go func() {
        time.Sleep(50 * time.Millisecond)
        _, _ = tls.Dial("tcp", l.Addr().String(), &tls.Config{InsecureSkipVerify: true, ServerName: "bench.example"})
    }()
    data := <-done
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        DetectTLSClientHello(data)
    }
}
