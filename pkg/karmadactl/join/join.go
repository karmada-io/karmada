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

package join

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/cluster/validation"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	utilcomp "github.com/karmada-io/karmada/pkg/karmadactl/util/completion"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	joinLong = templates.LongDesc(`
		Register a cluster to Karmada control plane with Push mode.`)

	joinExample = templates.Examples(`
		# Join cluster into karmada control plane, if '--cluster-context' not specified, take the cluster name as the context
		%[1]s join CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>`)
)

// NewCmdJoin defines the `join` command that registers a cluster.
func NewCmdJoin(f cmdutil.Factory, parentCommand string) *cobra.Command {
	opts := CommandJoinOption{}

	cmd := &cobra.Command{
		Use:                   "join CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>",
		Short:                 "Register a cluster to Karmada control plane with Push mode",
		Long:                  joinLong,
		Example:               fmt.Sprintf(joinExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := opts.Validate(args); err != nil {
				return err
			}
			if err := opts.Run(f); err != nil {
				return err
			}
			return nil
		},
		Annotations: map[string]string{
			cmdutil.TagCommandGroup: cmdutil.GroupClusterRegistration,
		},
	}

	flags := cmd.Flags()
	opts.AddFlags(flags)
	options.AddKubeConfigFlags(flags)

	utilcomp.RegisterCompletionFuncForKarmadaContextFlag(cmd)
	return cmd
}

// CommandJoinOption holds all command options.
type CommandJoinOption struct {
	// ClusterNamespace holds the namespace name where the member cluster secrets are stored.
	ClusterNamespace string

	// ClusterName is the cluster's name that we are going to join with.
	ClusterName string

	// ClusterContext is the cluster's context that we are going to join with.
	ClusterContext string

	// ClusterKubeConfig is the cluster's kubeconfig path.
	ClusterKubeConfig string

	// ClusterProvider is the cluster's provider.
	ClusterProvider string

	// ClusterRegion represents the region of the cluster locate in.
	ClusterRegion string

	// ClusterZones represents the failure zones(also called availability zones) of the
	// member cluster. The zones are presented as a slice to support the case
	// that cluster runs across multiple failure zones.
	// Refer https://kubernetes.io/docs/setup/best-practices/multiple-zones/ for
	// more details about running Kubernetes in multiple zones.
	ClusterZones []string

	// KarmadaAs represents the username to impersonate for the operation in karmada control plane. User could be a regular user or a service account in a namespace
	KarmadaAs string

	// KarmadaAsGroups represents groups to impersonate for the operation in karmada control plane, this flag can be repeated to specify multiple groups
	KarmadaAsGroups []string

	// KarmadaAsUID represents the UID to impersonate for the operation in karmada control plane.
	KarmadaAsUID string

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool
}

// Complete ensures that options are valid and marshals them if necessary.
func (j *CommandJoinOption) Complete(args []string) error {
	// Get cluster name from the command args.
	if len(args) > 0 {
		j.ClusterName = args[0]
	}

	return nil
}

// Validate checks option and return a slice of found errs.
func (j *CommandJoinOption) Validate(args []string) error {
	if len(args) > 1 {
		return errors.New("only the cluster name is allowed as an argument")
	}
	if len(j.ClusterName) == 0 {
		return errors.New("cluster name is required")
	}
	if errMsgs := validation.ValidateClusterName(j.ClusterName); len(errMsgs) != 0 {
		return fmt.Errorf("invalid cluster name(%s): %s", j.ClusterName, strings.Join(errMsgs, ";"))
	}

	if j.ClusterNamespace == names.NamespaceKarmadaSystem {
		klog.Warningf("karmada-system is always reserved for Karmada control plane. We do not recommend using karmada-system to store secrets of member clusters. It may cause mistaken cleanup of resources.")
	}

	return nil
}

// AddFlags adds flags to the specified FlagSet.
func (j *CommandJoinOption) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&j.ClusterNamespace, "cluster-namespace", options.DefaultKarmadaClusterNamespace, "Namespace in the control plane where member cluster secrets are stored.")
	flags.StringVar(&j.ClusterContext, "cluster-context", "",
		"Name of cluster context in kubeconfig. The current context is used by default.")
	flags.StringVar(&j.ClusterKubeConfig, "cluster-kubeconfig", "",
		"Path of the cluster's kubeconfig.")
	flags.StringVar(&j.ClusterProvider, "cluster-provider", "", "Provider of the joining cluster. The Karmada scheduler can use this information to spread workloads across providers for higher availability.")
	flags.StringVar(&j.ClusterRegion, "cluster-region", "", "The region of the joining cluster. The Karmada scheduler can use this information to spread workloads across regions for higher availability.")
	flags.StringSliceVar(&j.ClusterZones, "cluster-zones", nil, "The zones of the joining cluster. The Karmada scheduler can use this information to spread workloads across zones for higher availability.")
	flags.StringVar(&j.KarmadaAs, "karmada-as", "",
		"Username to impersonate for the operation in karmada control plane. User could be a regular user or a service account in a namespace.")
	flags.StringArrayVar(&j.KarmadaAsGroups, "karmada-as-group", []string{},
		"Group to impersonate for the operation in karmada control plane, this flag can be repeated to specify multiple groups.")
	flags.StringVar(&j.KarmadaAsUID, "karmada-as-uid", "",
		"UID to impersonate for the operation in karmada control plane.")
	flags.BoolVar(&j.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}

// Run is the implementation of the 'join' command.
func (j *CommandJoinOption) Run(f cmdutil.Factory) error {
	klog.V(1).Infof("Joining cluster. cluster name: %s", j.ClusterName)
	klog.V(1).Infof("Joining cluster. cluster namespace: %s", j.ClusterNamespace)

	// Get control plane karmada-apiserver client
	controlPlaneRestConfig, err := f.ToRawKubeConfigLoader().ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			*options.DefaultConfigFlags.Context, *options.DefaultConfigFlags.KubeConfig, err)
	}

	// Configure impersonation
	controlPlaneRestConfig.Impersonate.UserName = j.KarmadaAs
	controlPlaneRestConfig.Impersonate.Groups = j.KarmadaAsGroups
	controlPlaneRestConfig.Impersonate.UID = j.KarmadaAsUID

	// Get cluster config
	clusterConfig, err := apiclient.RestConfig(j.ClusterContext, j.ClusterKubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get joining cluster config. error: %v", err)
	}

	return j.RunJoinCluster(controlPlaneRestConfig, clusterConfig)
}

var controlPlaneKubeClientBuilder = func(controlPlaneRestConfig *rest.Config) kubeclient.Interface {
	return kubeclient.NewForConfigOrDie(controlPlaneRestConfig)
}
var karmadaClientBuilder = func(controlPlaneRestConfig *rest.Config) karmadaclientset.Interface {
	return karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)
}
var clusterKubeClientBuilder = func(clusterConfig *rest.Config) kubeclient.Interface {
	return kubeclient.NewForConfigOrDie(clusterConfig)
}

// RunJoinCluster join the cluster into karmada.
func (j *CommandJoinOption) RunJoinCluster(controlPlaneRestConfig, clusterConfig *rest.Config) (err error) {
	controlPlaneKubeClient := controlPlaneKubeClientBuilder(controlPlaneRestConfig)
	karmadaClient := karmadaClientBuilder(controlPlaneRestConfig)
	clusterKubeClient := clusterKubeClientBuilder(clusterConfig)

	klog.V(1).Infof("Joining cluster config. endpoint: %s", clusterConfig.Host)

	registerOption := util.ClusterRegisterOption{
		ClusterNamespace:   j.ClusterNamespace,
		ClusterName:        j.ClusterName,
		ReportSecrets:      []string{util.KubeCredentials, util.KubeImpersonator},
		ClusterProvider:    j.ClusterProvider,
		ClusterRegion:      j.ClusterRegion,
		ClusterZones:       j.ClusterZones,
		DryRun:             j.DryRun,
		ControlPlaneConfig: controlPlaneRestConfig,
		ClusterConfig:      clusterConfig,
	}

	registerOption.ClusterID, err = util.ObtainClusterID(clusterKubeClient)
	if err != nil {
		return err
	}

	if err = registerOption.Validate(karmadaClient, false); err != nil {
		return err
	}

	clusterSecret, impersonatorSecret, err := util.ObtainCredentialsFromMemberCluster(clusterKubeClient, registerOption)
	if err != nil {
		return err
	}

	if j.DryRun {
		return nil
	}
	registerOption.Secret = *clusterSecret
	registerOption.ImpersonatorSecret = *impersonatorSecret
	err = util.RegisterClusterInControllerPlane(registerOption, controlPlaneKubeClient, generateClusterInControllerPlane)
	if err != nil {
		return err
	}

	fmt.Printf("cluster(%s) is joined successfully\n", j.ClusterName)
	return nil
}

func generateClusterInControllerPlane(opts util.ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
	clusterObj := &clusterv1alpha1.Cluster{}
	clusterObj.Name = opts.ClusterName
	clusterObj.Spec.SyncMode = clusterv1alpha1.Push
	clusterObj.Spec.APIEndpoint = opts.ClusterConfig.Host
	clusterObj.Spec.ID = opts.ClusterID
	clusterObj.Spec.SecretRef = &clusterv1alpha1.LocalSecretReference{
		Namespace: opts.Secret.Namespace,
		Name:      opts.Secret.Name,
	}
	clusterObj.Spec.ImpersonatorSecretRef = &clusterv1alpha1.LocalSecretReference{
		Namespace: opts.ImpersonatorSecret.Namespace,
		Name:      opts.ImpersonatorSecret.Name,
	}

	if opts.ClusterProvider != "" {
		clusterObj.Spec.Provider = opts.ClusterProvider
	}

	if len(opts.ClusterZones) > 0 {
		clusterObj.Spec.Zones = opts.ClusterZones
	}

	if opts.ClusterRegion != "" {
		clusterObj.Spec.Region = opts.ClusterRegion
	}

	clusterObj.Spec.InsecureSkipTLSVerification = opts.ClusterConfig.TLSClientConfig.Insecure

	if opts.ClusterConfig.Proxy != nil {
		url, err := opts.ClusterConfig.Proxy(nil)
		if err != nil {
			return nil, fmt.Errorf("clusterConfig.Proxy error, %v", err)
		}
		clusterObj.Spec.ProxyURL = url.String()
	}

	controlPlaneKarmadaClient := karmadaClientBuilder(opts.ControlPlaneConfig)
	cluster, err := util.CreateClusterObject(controlPlaneKarmadaClient, clusterObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster(%s) object. error: %v", opts.ClusterName, err)
	}

	return cluster, nil
}
