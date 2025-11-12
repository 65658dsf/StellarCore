package protoinspect

import (
    "bytes"
    "strings"
)

func DetectHTTP(buf []byte) (bool, map[string]string) {
    features := map[string]string{}
    b := bytes.TrimLeft(buf, " \t\r\n")
    if len(b) < 3 {
        return false, features
    }
    methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH", "CONNECT"}
    upper := strings.ToUpper(string(b))
    matchedMethod := ""
    for _, m := range methods {
        if strings.HasPrefix(upper, m+" ") || strings.HasPrefix(upper, m+"/") {
            matchedMethod = m
            break
        }
    }
    if matchedMethod == "" {
        return false, features
    }
    features["Method"] = matchedMethod
    if strings.Contains(upper, "HTTP/") {
        features["HTTPVersion"] = "true"
    }
    if strings.Contains(upper, "\r\nHOST:") || strings.Contains(upper, "\nHOST:") || strings.HasPrefix(upper, "HOST:") {
        features["HostHeader"] = "true"
    }
    return true, features
}
