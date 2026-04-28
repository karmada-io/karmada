/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
