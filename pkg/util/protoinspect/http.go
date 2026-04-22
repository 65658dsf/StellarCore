package protoinspect

import (
	"bytes"
	"strings"
)

func DetectHTTP(buf []byte) DetectionResult {
	b := bytes.TrimLeft(buf, " \t\r\n")
	if len(b) < 3 {
		return NoDetection()
	}

	methods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS", "PATCH", "CONNECT"}
	upper := strings.ToUpper(string(b))
	matchedMethod := ""
	for _, method := range methods {
		if strings.HasPrefix(upper, method+" ") || strings.HasPrefix(upper, method+"/") {
			matchedMethod = method
			break
		}
	}
	if matchedMethod == "" {
		return NoDetection()
	}

	features := map[string]string{
		"Method": matchedMethod,
	}
	if strings.Contains(upper, "HTTP/") {
		features["HTTPVersion"] = "true"
	}
	if strings.Contains(upper, "\r\nHOST:") || strings.Contains(upper, "\nHOST:") || strings.HasPrefix(upper, "HOST:") {
		features["HostHeader"] = "true"
	}

	return MatchDetection("http", 100, false, []string{"http_method"}, features)
}
