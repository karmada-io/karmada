package utils

import (
	"fmt"
	"net"
)

//ParseIP parse an ip address and return whether it is a v4 or v6 ip address
func ParseIP(s string) (net.IP, int, error) {
	ip := net.ParseIP(s)
	if ip == nil {
		return nil, 0, fmt.Errorf("%s is not a valid ip address", s)
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.':
			return ip, 4, nil
		case ':':
			return ip, 6, nil
		}
	}
	return nil, 0, fmt.Errorf("%s is not a valid ip address", s)
}
