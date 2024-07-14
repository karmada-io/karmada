/*
Copyright 2020 The Karmada Authors.

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
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// DefaultKarmadaClusterNamespace defines the default namespace where the member cluster secrets are stored.
const DefaultKarmadaClusterNamespace = "karmada-cluster"

// DefaultHostClusterDomain defines the default cluster domain of karmada host cluster.
const DefaultHostClusterDomain = "cluster.local"

// DefaultKarmadactlCommandDuration defines the default timeout for karmadactl execute
const DefaultKarmadactlCommandDuration = 60 * time.Second

const (
	// KarmadaCertsName the secret name of karmada certs
	KarmadaCertsName = "karmada-cert"
	// CaCertAndKeyName ca certificate cert/key name in karmada certs secret
	CaCertAndKeyName = "ca"
)

// DefaultConfigFlags It composes the set of values necessary for obtaining a REST client config with default values set.
var DefaultConfigFlags = genericclioptions.NewConfigFlags(true).WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).WithDiscoveryQPS(50.0)

// AddKubeConfigFlags adds flags to the specified FlagSet.
func AddKubeConfigFlags(flags *pflag.FlagSet) {
	flags.StringVar(DefaultConfigFlags.KubeConfig, "kubeconfig", *DefaultConfigFlags.KubeConfig, "Path to the kubeconfig file to use for CLI requests.")
	flags.StringVar(DefaultConfigFlags.Context, "karmada-context", *DefaultConfigFlags.Context, "The name of the kubeconfig context to use")
}
