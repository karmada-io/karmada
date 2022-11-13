package utils

import (
	"fmt"
	"net"
)

// ParseIP parses an IP address and returns whether it is a v4 or v6 IP address
func ParseIP(s string) (net.IP, int, error) {
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, 0, fmt.Errorf("%s is not a valid ip address", s)
	}
	if ip.To4() != nil {
		return ip, 4, nil
	}
	return ip, 6, nil
}
