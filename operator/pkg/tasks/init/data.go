package tasks

import (
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/certs"
)

// InitData is interface to operate the runData in workflow
type InitData interface {
	certs.CertStore
	GetName() string
	GetNamespace() string
	SetControlplaneConfig(config *rest.Config)
	ControlplaneConfig() *rest.Config
	ControlplaneAddress() string
	RemoteClient() clientset.Interface
	KarmadaClient() clientset.Interface
	DataDir() string
	CrdsRemoteURL() string
	KarmadaVersion() string
	Components() *operatorv1alpha1.KarmadaComponents
}
