package util

import "k8s.io/client-go/rest"

// ControllerConfig defines the common configuration shared by most of controllers.
type ControllerConfig struct {
	// HeadClusterConfig holds the configuration of head cluster.
	HeadClusterConfig *rest.Config
}
