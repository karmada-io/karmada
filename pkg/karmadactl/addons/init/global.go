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

package init

import (
	"fmt"

	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
)

// GlobalCommandOptions holds the configuration shared by the all sub-commands of `karmadactl`.
type GlobalCommandOptions struct {
	// KubeConfig holds host cluster KUBECONFIG file path.
	KubeConfig string
	Context    string

	// KarmadaConfig holds karmada control plane KUBECONFIG file path.
	KarmadaConfig  string
	KarmadaContext string

	// Namespace holds the namespace where Karmada components installed
	Namespace string

	// Cluster holds the name of member cluster to enable or disable scheduler estimator
	Cluster string

	KubeClientSet kubernetes.Interface

	KarmadaRestConfig *rest.Config

	KarmadaAggregatorClientSet aggregator.Interface
}

// AddFlags adds flags to the specified FlagSet.
func (o *GlobalCommandOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.Namespace, "namespace", "n", "karmada-system", "namespace where Karmada components are installed.")
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Path to the host cluster kubeconfig file.")
	flags.StringVar(&o.Context, "context", "", "The name of the kubeconfig context to use.")
	flags.StringVar(&o.KarmadaConfig, "karmada-kubeconfig", "/etc/karmada/karmada-apiserver.config", "Path to the karmada control plane kubeconfig file.")
	flags.StringVar(&o.KarmadaContext, "karmada-context", "", "The name of the karmada control plane kubeconfig context to use.")
	flags.StringVarP(&o.Cluster, "cluster", "C", "", "Name of the member cluster that enables or disables the scheduler estimator.")
}

// Complete the conditions required to be able to run list.
func (o *GlobalCommandOptions) Complete() error {
	restConfig, err := apiclient.RestConfig(o.Context, o.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get karmada-host config. error: %v", err)
	}

	o.KubeClientSet, err = apiclient.NewClientSet(restConfig)
	if err != nil {
		return err
	}

	o.KarmadaRestConfig, err = apiclient.RestConfig(o.KarmadaContext, o.KarmadaConfig)
	if err != nil {
		return fmt.Errorf("failed to get karmada-apiserver config from %s. please use --karmada-kubeconfig to point the config file. \n error: %v", o.KarmadaConfig, err)
	}

	o.KarmadaAggregatorClientSet, err = apiclient.NewAPIRegistrationClient(o.KarmadaRestConfig)
	if err != nil {
		return err
	}

	return nil
}
