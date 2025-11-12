package protoinspect

import "testing"

func TestDetectHTTP(t *testing.T) {
    req := []byte("GET /path HTTP/1.1\r\nHost: example.com\r\n\r\n")
    ok, feat := DetectHTTP(req)
    if !ok {
        t.Fatalf("expected http detected")
    }
    if feat["Method"] != "GET" {
        t.Fatalf("expected method GET, got %s", feat["Method"])
    }
    if feat["HostHeader"] != "true" {
        t.Fatalf("expected host header true")
    }
}
