/*
Copyright The Karmada Authors.

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

package options

import (
	"time"

	"github.com/spf13/pflag"
)

// DefaultKarmadaClusterNamespace defines the default namespace where the member cluster objects are stored.
// The secret owns by cluster objects will be stored in the namespace too.
const DefaultKarmadaClusterNamespace = "karmada-cluster"

// DefaultKarmadactlCommandDuration defines the default timeout for karmadactl execute
const DefaultKarmadactlCommandDuration = 60 * time.Second

// GlobalCommandOptions holds the configuration shared by the all sub-commands of `karmadactl`.
type GlobalCommandOptions struct {
	// KubeConfig holds the control plane KUBECONFIG file path.
	KubeConfig string

	// ClusterContext is the name of the cluster context in control plane KUBECONFIG file.
	// Default value is the current-context.
	KarmadaContext string

	// ClusterNamespace holds the namespace name where the member cluster objects are stored.
	ClusterNamespace string

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool
}

// AddFlags adds flags to the specified FlagSet.
func (o *GlobalCommandOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Path to the control plane kubeconfig file.")
	flags.StringVar(&o.KarmadaContext, "karmada-context", "", "Name of the cluster context in control plane kubeconfig file.")
	flags.StringVar(&o.ClusterNamespace, "cluster-namespace", DefaultKarmadaClusterNamespace, "Namespace in the control plane where member cluster are stored.")
	flags.BoolVar(&o.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}
