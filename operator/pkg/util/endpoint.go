package util

import (
	"context"
	"fmt"
	"net"
	"net/url"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	clientset "k8s.io/client-go/kubernetes"
	netutils "k8s.io/utils/net"
)

// GetControlplaneEndpoint parses an Endpoint and returns it as a string,
// or returns an error in case it cannot be parsed.
func GetControlplaneEndpoint(address, port string) (string, error) {
	var ip = netutils.ParseIPSloppy(address)
	if ip == nil {
		return "", fmt.Errorf("invalid value `%s` given for address", address)
	}
	return formatURL(ip.String(), port).String(), nil
}

// formatURL takes a host and a port string and creates a net.URL using https scheme
func formatURL(host, port string) *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   net.JoinHostPort(host, port),
	}
}

// GetAPIServiceIP returns a valid node IP address.
func GetAPIServiceIP(clientset clientset.Interface) (string, error) {
	nodes, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil || len(nodes.Items) == 0 {
		return "", fmt.Errorf("there are no nodes in cluster, err: %w", err)
	}

	var (
		masterLabel       = labels.Set{"node-role.kubernetes.io/master": ""}
		controlplaneLabel = labels.Set{"node-role.kubernetes.io/control-plane": ""}
	)
	// first, select the master node as the IP of APIServer. if there is
	// no master nodes, randomly select a worker node.
	for _, node := range nodes.Items {
		ls := labels.Set(node.GetLabels())

		if masterLabel.AsSelector().Matches(ls) || controlplaneLabel.AsSelector().Matches(ls) {
			if ip := netutils.ParseIPSloppy(node.Status.Addresses[0].Address); ip != nil {
				return ip.String(), nil
			}
		}
	}
	return nodes.Items[0].Status.Addresses[0].Address, nil
}
