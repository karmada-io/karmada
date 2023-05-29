package karmada

import (
	"errors"
	"fmt"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	tasks "github.com/karmada-io/karmada/operator/pkg/tasks/deinit"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// DeInitOptions defines all the Deinit workflow options.
type DeInitOptions struct {
	Name        string
	Namespace   string
	Kubeconfig  *rest.Config
	HostCluster *operatorv1alpha1.HostCluster
}

// DeInitOpt defines a type of function to set DeInitOptions values.
type DeInitOpt func(o *DeInitOptions)

var _ tasks.DeInitData = &deInitData{}

// deInitData defines all the runtime information used when ruing deinit workflow;
// this data is shared across all the tasks of workflow.
type deInitData struct {
	name         string
	namespace    string
	remoteClient clientset.Interface
}

// NewDeInitDataJob initializes a deInit job with a list of sub tasks. and build
// deinit runData object
func NewDeInitDataJob(opt *DeInitOptions) *workflow.Job {
	deInitJob := workflow.NewJob()

	deInitJob.AppendTask(tasks.NewRemoveComponentTask())
	deInitJob.AppendTask(tasks.NewCleanupCertTask())
	deInitJob.AppendTask(tasks.NewCleanupKubeconfigTask())

	deInitJob.SetDataInitializer(func() (workflow.RunData, error) {
		// if there is no endpoint info, we are consider that the local cluster
		// is remote cluster to install karmada.
		var remoteClient clientset.Interface
		if opt.HostCluster.SecretRef == nil && len(opt.HostCluster.APIEndpoint) == 0 {
			client, err := clientset.NewForConfig(opt.Kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("error when create cluster client to install karmada, err: %w", err)
			}

			remoteClient = client
		}

		if len(opt.Name) == 0 || len(opt.Namespace) == 0 {
			return nil, errors.New("unexpected empty name or namespace")
		}

		return &deInitData{
			name:         opt.Name,
			namespace:    opt.Namespace,
			remoteClient: remoteClient,
		}, nil
	})

	return deInitJob
}

func (data *deInitData) GetName() string {
	return data.name
}

func (data *deInitData) GetNamespace() string {
	return data.namespace
}

func (data *deInitData) RemoteClient() clientset.Interface {
	return data.remoteClient
}

// NewJobDeInitOptions calls all of DeInitOpt func to initialize a DeInitOptions.
// if there is not DeInitOpt functions, it will return a default DeInitOptions.
func NewJobDeInitOptions(opts ...DeInitOpt) *DeInitOptions {
	options := defaultJobDeInitOptions()

	for _, c := range opts {
		c(options)
	}
	return options
}

func defaultJobDeInitOptions() *DeInitOptions {
	return &DeInitOptions{
		Name:        "karmada",
		Namespace:   constants.KarmadaSystemNamespace,
		HostCluster: &operatorv1alpha1.HostCluster{},
	}
}

// NewDeInitOptWithKarmada returns a DeInitOpt function to initialize DeInitOptions with karmada resource
func NewDeInitOptWithKarmada(karmada *operatorv1alpha1.Karmada) DeInitOpt {
	return func(o *DeInitOptions) {
		o.Name = karmada.GetName()
		o.Namespace = karmada.GetNamespace()

		if karmada.Spec.HostCluster != nil {
			o.HostCluster = karmada.Spec.HostCluster
		}
	}
}

// NewDeInitOptWithKubeconfig returns a DeInitOpt function to set kubeconfig to DeInitOptions with rest config
func NewDeInitOptWithKubeconfig(config *rest.Config) DeInitOpt {
	return func(o *DeInitOptions) {
		o.Kubeconfig = config
	}
}
