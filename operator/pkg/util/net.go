package util

import (
	"io"
	"net"
	"net/http"

	netutils "k8s.io/utils/net"
)

const (
	// Query the IP address of the current host accessing the Internet
	getInternetIPUrl = "https://myexternalip.com/raw"
)

// InternetIP Current host Internet IP.
func InternetIP() (net.IP, error) {
	resp, err := http.Get(getInternetIPUrl)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return netutils.ParseIPSloppy(string(content)), nil
}
