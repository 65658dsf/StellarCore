package protoinspect

import (
	"maps"
	"slices"
	"strings"
)

type DetectionResult struct {
	Matched      bool
	Protocol     string
	Confidence   int
	NeedMoreData bool
	ReasonCodes  []string
	Features     map[string]string
}

func NoDetection() DetectionResult {
	return DetectionResult{
		Features: map[string]string{},
	}
}

func MatchDetection(protocol string, confidence int, needMoreData bool, reasonCodes []string, features map[string]string) DetectionResult {
	result := DetectionResult{
		Matched:      true,
		Protocol:     CanonicalProtocol(protocol),
		Confidence:   confidence,
		NeedMoreData: needMoreData,
		ReasonCodes:  append([]string(nil), reasonCodes...),
		Features:     map[string]string{},
	}
	if features != nil {
		result.Features = maps.Clone(features)
	}
	return result
}

func BestMatch(results ...DetectionResult) DetectionResult {
	best := NoDetection()
	for _, result := range results {
		if betterDetection(result, best) {
			best = result
		}
	}
	return best
}

func JoinReasonCodes(reasonCodes []string) string {
	if len(reasonCodes) == 0 {
		return "none"
	}
	uniq := append([]string(nil), reasonCodes...)
	slices.Sort(uniq)
	uniq = slices.Compact(uniq)
	return strings.Join(uniq, "+")
}

func CanonicalProtocol(protocol string) string {
	value := strings.ToLower(strings.TrimSpace(protocol))
	switch value {
	case "openvpn", "openvpn-tcp":
		return "openvpn"
	case "wireguard":
		return "wireguard"
	case "ikev2", "ipsec":
		return "ikev2"
	case "socks5":
		return "socks5"
	case "vless":
		return "vless"
	case "trojan":
		return "trojan"
	case "tuic":
		return "tuic"
	case "hy2", "hysteria2", "hysteria-2":
		return "hysteria2"
	case "tls_candidate":
		return "tls_candidate"
	case "generic_quic":
		return "generic_quic"
	case "quic_tunnel_candidate":
		return "quic_tunnel_candidate"
	case "http":
		return "http"
	default:
		return value
	}
}

func IsKnownVPNProtocol(protocol string) bool {
	switch CanonicalProtocol(protocol) {
	case "openvpn", "wireguard", "ikev2", "socks5", "vless", "trojan", "tuic", "hysteria2":
		return true
	default:
		return false
	}
}

func IsCandidateProtocol(protocol string) bool {
	switch CanonicalProtocol(protocol) {
	case "tls_candidate", "generic_quic", "quic_tunnel_candidate":
		return true
	default:
		return false
	}
}

func betterDetection(candidate DetectionResult, current DetectionResult) bool {
	if candidate.Matched != current.Matched {
		return candidate.Matched
	}
	if !candidate.Matched {
		return false
	}
	if candidate.Confidence != current.Confidence {
		return candidate.Confidence > current.Confidence
	}
	if candidate.NeedMoreData != current.NeedMoreData {
		return !candidate.NeedMoreData
	}
	if IsCandidateProtocol(candidate.Protocol) != IsCandidateProtocol(current.Protocol) {
		return !IsCandidateProtocol(candidate.Protocol)
	}
	if len(candidate.ReasonCodes) != len(current.ReasonCodes) {
		return len(candidate.ReasonCodes) > len(current.ReasonCodes)
	}
	return candidate.Protocol < current.Protocol
}
