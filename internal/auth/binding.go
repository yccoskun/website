package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
)

// HashUA returns the SHA-256 hex of ua, or "" when ua is empty.
func HashUA(ua string) string {
	if ua == "" {
		return ""
	}
	return hashString(ua)
}

// HashIPPrefix normalizes clientIP to an IPv4 /24 or IPv6 /48 prefix string,
// then returns its SHA-256 hex. Empty or unparseable IP → "".
func HashIPPrefix(clientIP string) string {
	prefix := IPPrefix(clientIP)
	if prefix == "" {
		return ""
	}
	return hashString(prefix)
}

// IPPrefix returns "a.b.c.0/24" for IPv4 or a /48 CIDR string for IPv6.
// Empty or unparseable input returns "".
func IPPrefix(clientIP string) string {
	if clientIP == "" {
		return ""
	}
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		masked := net.IPv4(v4[0], v4[1], v4[2], 0).To4()
		return masked.String() + "/24"
	}
	v6 := ip.To16()
	if v6 == nil {
		return ""
	}
	masked := make(net.IP, net.IPv6len)
	copy(masked, v6)
	for i := 6; i < net.IPv6len; i++ {
		masked[i] = 0
	}
	return masked.String() + "/48"
}

func hashString(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
