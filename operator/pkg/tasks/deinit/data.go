package tasks

import (
	clientset "k8s.io/client-go/kubernetes"
)

// DeInitData is interface to operate the runData of DeInitData workflow
type DeInitData interface {
	GetName() string
	GetNamespace() string
	RemoteClient() clientset.Interface
}
