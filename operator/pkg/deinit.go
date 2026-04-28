/*
Copyright 2023 The Karmada Authors.

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

package karmada

import (
	"errors"
	"fmt"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	tasks "github.com/karmada-io/karmada/operator/pkg/tasks/deinit"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

// DeInitOptions defines all the Deinit workflow options.
type DeInitOptions struct {
	Name        string
	Namespace   string
	Kubeconfig  *rest.Config
	HostCluster *operatorv1alpha1.HostCluster
	Karmada     *operatorv1alpha1.Karmada
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

	deInitJob.AppendTask(tasks.NewRemoveComponentTask(opt.Karmada))
	deInitJob.AppendTask(tasks.NewCleanupCertTask(opt.Karmada))
	deInitJob.AppendTask(tasks.NewCleanupKubeconfigTask())

	deInitJob.SetDataInitializer(func() (workflow.RunData, error) {
		localClusterClient, err := clientset.NewForConfig(opt.Kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("error when creating local cluster client, err: %w", err)
		}

		// if there is no endpoint info, we are consider that the local cluster
		// is remote cluster to install karmada.
		var remoteClient clientset.Interface
		if util.IsInCluster(opt.HostCluster) {
			remoteClient = localClusterClient
		} else {
			remoteClient, err = util.BuildClientFromSecretRef(localClusterClient, opt.HostCluster.SecretRef)
			if err != nil {
				return nil, fmt.Errorf("error when creating cluster client to install karmada, err: %w", err)
			}
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
		o.Karmada = karmada
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
