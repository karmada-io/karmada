package util

import (
	"os"
	"path/filepath"

	"k8s.io/client-go/util/homedir"
)

var defaultKubeConfig = filepath.Join(homedir.HomeDir(), ".kube", "config")

// GetDefaultKubeConfigPath get kubernetes config default path
func GetDefaultKubeConfigPath() string {
	if env := os.Getenv("KUBECONFIG"); env != "" {
		return env
	}
	return defaultKubeConfig
}
