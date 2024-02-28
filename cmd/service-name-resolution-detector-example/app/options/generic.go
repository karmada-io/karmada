/*
Copyright 2024 The Karmada Authors.

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
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	componentbaseconfig "k8s.io/component-base/config"
	componentbaseoptions "k8s.io/component-base/config/options"

	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
)

// GenericOptions contains some generic options.
type GenericOptions struct {
	KarmadaKubeConfig   string
	KarmadaContext      string
	KarmadaConfig       *rest.Config
	MemberClusterConfig *rest.Config

	ClusterName string
	Hostname    string

	BindAddress string
	HealthzPort int

	Detectors []string

	LeaderElection componentbaseconfig.LeaderElectionConfiguration
}

// NewGenericOptions return default generic options.
func NewGenericOptions() *GenericOptions {
	return &GenericOptions{
		LeaderElection: componentbaseconfig.LeaderElectionConfiguration{
			LeaderElect:       true,
			ResourceLock:      resourcelock.LeasesResourceLock,
			ResourceName:      "cluster-problem-detector",
			ResourceNamespace: "kube-system",
			LeaseDuration:     metav1.Duration{Duration: 15 * time.Second},
			RetryPeriod:       metav1.Duration{Duration: 2 * time.Second},
			RenewDeadline:     metav1.Duration{Duration: 10 * time.Second},
		},
	}
}

// AddFlags adds flags of generic options to the specified FlagSet.
func (o *GenericOptions) AddFlags(fs *pflag.FlagSet, allDetectors []string) {
	fs.StringVar(&o.KarmadaKubeConfig, "karmada-kubeconfig", o.KarmadaKubeConfig, "Path to karmada control plane kubeconfig file.")
	fs.StringVar(&o.KarmadaContext, "karmada-context", "", "Name of the cluster context in karmada control plane kubeconfig file.")

	fs.StringVar(&o.ClusterName, "cluster-name", o.ClusterName, "Name of member cluster that the agent serves for.")
	fs.StringVar(&o.Hostname, "host-name", o.Hostname, "Name of host that the agent runs on, used as lock holder identity.")

	fs.StringVar(&o.BindAddress, "bind-address", "127.0.0.1", "The IP address on which to listen.")
	fs.IntVar(&o.HealthzPort, "healthz-port", o.HealthzPort, "The port on which to serve health check.")

	fs.StringSliceVar(&o.Detectors, "detectors", []string{"*"}, fmt.Sprintf(
		"A list of detectors to enable. '*' enables all on-by-default detectors, 'foo' enables the detector named 'foo', '-foo' disables the detector named 'foo'. All detectors: %s.",
		strings.Join(allDetectors, ", "),
	))

	componentbaseoptions.BindLeaderElectionFlags(&o.LeaderElection, fs)
}

// Complete fills in fields required to have valid data.
func (o *GenericOptions) Complete() error {
	if o == nil {
		return nil
	}

	controlPlaneRestConfig, err := apiclient.RestConfig(o.KarmadaContext, o.KarmadaKubeConfig)
	if err != nil {
		return fmt.Errorf("failed to build kubeconfig of karmada control plane: %v", err)
	}
	o.KarmadaConfig = controlPlaneRestConfig

	memberClusterRestConfig, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to build in-cluster config: %v", err)
	}
	o.MemberClusterConfig = memberClusterRestConfig

	hostName, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %v", err)
	}
	if len(o.Hostname) == 0 {
		o.Hostname = hostName
	}
	return nil
}

// Validate checks options and return a slice of found errs.
func (o *GenericOptions) Validate() field.ErrorList {
	errs := field.ErrorList{}
	if err := validateAPIServerAddr(o.MemberClusterConfig.Host); err != nil {
		errs = append(errs, field.InternalError(field.NewPath("InClusterConfig"), err))
	}
	return errs
}

func validateAPIServerAddr(addr string) error {
	server, err := url.Parse(addr)
	if err != nil {
		return err
	}
	host, _, err := net.SplitHostPort(server.Host)
	if err != nil {
		return err
	}
	// The host must be an IP rather than a domain name.
	if net.ParseIP(host) == nil {
		return fmt.Errorf("server address of member cluster kubeconfig must be an IP")
	}
	return nil
}
