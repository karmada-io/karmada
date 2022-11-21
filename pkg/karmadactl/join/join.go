package join

import (
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
	"github.com/karmada-io/karmada/pkg/util"
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
		RunE: func(cmd *cobra.Command, args []string) error {
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

	// ClusterZone represents the zone of the cluster locate in.
	ClusterZone string

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool
}

// Complete ensures that options are valid and marshals them if necessary.
func (j *CommandJoinOption) Complete(args []string) error {
	// Get cluster name from the command args.
	if len(args) > 0 {
		j.ClusterName = args[0]
	}

	// If '--cluster-context' not specified, take the cluster name as the context.
	if len(j.ClusterContext) == 0 {
		j.ClusterContext = j.ClusterName
	}

	return nil
}

// Validate checks option and return a slice of found errs.
func (j *CommandJoinOption) Validate(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("only the cluster name is allowed as an argument")
	}
	if len(j.ClusterName) == 0 {
		return fmt.Errorf("cluster name is required")
	}
	if errMsgs := validation.ValidateClusterName(j.ClusterName); len(errMsgs) != 0 {
		return fmt.Errorf("invalid cluster name(%s): %s", j.ClusterName, strings.Join(errMsgs, ";"))
	}

	if j.ClusterNamespace == util.NamespaceKarmadaSystem {
		klog.Warningf("karmada-system is always reserved for Karmada control plane. We do not recommend using karmada-system to store secrets of member clusters. It may cause mistaken cleanup of resources.")
	}
	return nil
}

// AddFlags adds flags to the specified FlagSet.
func (j *CommandJoinOption) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&j.ClusterNamespace, "cluster-namespace", options.DefaultKarmadaClusterNamespace, "Namespace in the control plane where member cluster secrets are stored.")

	flags.StringVar(&j.ClusterContext, "cluster-context", "",
		"Context name of cluster in kubeconfig. Only works when there are multiple contexts in the kubeconfig.")
	flags.StringVar(&j.ClusterKubeConfig, "cluster-kubeconfig", "",
		"Path of the cluster's kubeconfig.")
	flags.StringVar(&j.ClusterProvider, "cluster-provider", "", "Provider of the joining cluster. The Karmada scheduler can use this information to spread workloads across providers for higher availability.")
	flags.StringVar(&j.ClusterRegion, "cluster-region", "", "The region of the joining cluster. The Karmada scheduler can use this information to spread workloads across regions for higher availability.")
	flags.StringVar(&j.ClusterZone, "cluster-zone", "", "The zone of the joining cluster")
	flags.BoolVar(&j.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}

// Run is the implementation of the 'join' command.
func (j *CommandJoinOption) Run(f cmdutil.Factory) error {
	klog.V(1).Infof("joining cluster. cluster name: %s", j.ClusterName)
	klog.V(1).Infof("joining cluster. cluster namespace: %s", j.ClusterNamespace)

	// Get control plane karmada-apiserver client
	controlPlaneRestConfig, err := f.ToRawKubeConfigLoader().ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			*options.DefaultConfigFlags.Context, *options.DefaultConfigFlags.KubeConfig, err)
	}

	// Get cluster config
	clusterConfig, err := apiclient.RestConfig(j.ClusterContext, j.ClusterKubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get joining cluster config. error: %v", err)
	}

	return j.RunJoinCluster(controlPlaneRestConfig, clusterConfig)
}

// RunJoinCluster join the cluster into karmada.
func (j *CommandJoinOption) RunJoinCluster(controlPlaneRestConfig, clusterConfig *rest.Config) (err error) {
	controlPlaneKubeClient := kubeclient.NewForConfigOrDie(controlPlaneRestConfig)
	karmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)
	clusterKubeClient := kubeclient.NewForConfigOrDie(clusterConfig)

	klog.V(1).Infof("joining cluster config. endpoint: %s", clusterConfig.Host)

	registerOption := util.ClusterRegisterOption{
		ClusterNamespace:   j.ClusterNamespace,
		ClusterName:        j.ClusterName,
		ReportSecrets:      []string{util.KubeCredentials, util.KubeImpersonator},
		ClusterProvider:    j.ClusterProvider,
		ClusterRegion:      j.ClusterRegion,
		ClusterZone:        j.ClusterZone,
		DryRun:             j.DryRun,
		ControlPlaneConfig: controlPlaneRestConfig,
		ClusterConfig:      clusterConfig,
	}

	id, err := util.ObtainClusterID(clusterKubeClient)
	if err != nil {
		return err
	}

	ok, name, err := util.IsClusterIdentifyUnique(karmadaClient, id)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("the same cluster has been registered with name %s", name)
	}

	registerOption.ClusterID = id

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

	if opts.ClusterZone != "" {
		clusterObj.Spec.Zone = opts.ClusterZone
	}

	if opts.ClusterRegion != "" {
		clusterObj.Spec.Region = opts.ClusterRegion
	}

	if opts.ClusterConfig.TLSClientConfig.Insecure {
		clusterObj.Spec.InsecureSkipTLSVerification = true
	}

	if opts.ClusterConfig.Proxy != nil {
		url, err := opts.ClusterConfig.Proxy(nil)
		if err != nil {
			return nil, fmt.Errorf("clusterConfig.Proxy error, %v", err)
		}
		clusterObj.Spec.ProxyURL = url.String()
	}

	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(opts.ControlPlaneConfig)
	cluster, err := util.CreateClusterObject(controlPlaneKarmadaClient, clusterObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster(%s) object. error: %v", opts.ClusterName, err)
	}

	return cluster, nil
}
