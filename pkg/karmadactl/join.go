package karmadactl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/cluster/validation"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
)

var (
	joinShort = `Register a cluster to control plane`
	joinLong  = `Join registers a cluster to control plane.`
)

// NewCmdJoin defines the `join` command that registers a cluster.
func NewCmdJoin(karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	opts := CommandJoinOption{}

	cmd := &cobra.Command{
		Use:          "join CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>",
		Short:        joinShort,
		Long:         joinLong,
		Example:      joinExample(parentCommand),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := RunJoin(karmadaConfig, opts); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	opts.AddFlags(flags)

	return cmd
}

func joinExample(parentCommand string) string {
	example := `
# Join cluster into karamada control plane, if '--cluster-context' not specified, take the cluster name as the context` + "\n" +
		fmt.Sprintf("%s join CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>", parentCommand)
	return example
}

// CommandJoinOption holds all command options.
type CommandJoinOption struct {
	options.GlobalCommandOptions

	// ClusterNamespace holds the namespace name where the member cluster objects are stored.
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
	if len(args) == 0 {
		return errors.New("cluster name is required")
	}
	j.ClusterName = args[0]

	// If '--cluster-context' not specified, take the cluster name as the context.
	if len(j.ClusterContext) == 0 {
		j.ClusterContext = j.ClusterName
	}

	return nil
}

// Validate checks option and return a slice of found errs.
func (j *CommandJoinOption) Validate() error {
	if errMsgs := validation.ValidateClusterName(j.ClusterName); len(errMsgs) != 0 {
		return fmt.Errorf("invalid cluster name(%s): %s", j.ClusterName, strings.Join(errMsgs, ";"))
	}

	return nil
}

// AddFlags adds flags to the specified FlagSet.
func (j *CommandJoinOption) AddFlags(flags *pflag.FlagSet) {
	j.GlobalCommandOptions.AddFlags(flags)

	flags.StringVar(&j.ClusterNamespace, "cluster-namespace", options.DefaultKarmadaClusterNamespace, "Namespace in the control plane where member cluster secrets are stored.")

	flags.StringVar(&j.ClusterContext, "cluster-context", "",
		"Context name of cluster in kubeconfig. Only works when there are multiple contexts in the kubeconfig.")
	flags.StringVar(&j.ClusterKubeConfig, "cluster-kubeconfig", "",
		"Path of the cluster's kubeconfig.")
	flags.StringVar(&j.ClusterProvider, "cluster-provider", "", "Provider of the joining cluster.")
	flags.StringVar(&j.ClusterRegion, "cluster-region", "", "The region of the joining cluster.")
	flags.StringVar(&j.ClusterZone, "cluster-zone", "", "The zone of the joining cluster")
	flags.BoolVar(&j.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}

// RunJoin is the implementation of the 'join' command.
func RunJoin(karmadaConfig KarmadaConfig, opts CommandJoinOption) error {
	klog.V(1).Infof("joining cluster. cluster name: %s", opts.ClusterName)
	klog.V(1).Infof("joining cluster. cluster namespace: %s", opts.ClusterNamespace)

	// Get control plane karmada-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err)
	}

	// Get cluster config
	clusterConfig, err := karmadaConfig.GetRestConfig(opts.ClusterContext, opts.ClusterKubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get joining cluster config. error: %v", err)
	}

	return JoinCluster(controlPlaneRestConfig, clusterConfig, opts)
}

// JoinCluster join the cluster into karmada.
func JoinCluster(controlPlaneRestConfig, clusterConfig *rest.Config, opts CommandJoinOption) (err error) {
	controlPlaneKubeClient := kubeclient.NewForConfigOrDie(controlPlaneRestConfig)
	clusterKubeClient := kubeclient.NewForConfigOrDie(clusterConfig)

	klog.V(1).Infof("joining cluster config. endpoint: %s", clusterConfig.Host)

	// ensure namespace where the cluster object be stored exists in control plane.
	if _, err = util.EnsureNamespaceExist(controlPlaneKubeClient, opts.ClusterNamespace, opts.DryRun); err != nil {
		return err
	}

	registerOption := util.ClusterRegisterOption{
		ClusterNamespace:   opts.ClusterNamespace,
		ClusterName:        opts.ClusterName,
		ReportSecrets:      []string{util.KubeCredentials, util.KubeImpersonator},
		ClusterProvider:    opts.ClusterProvider,
		ClusterRegion:      opts.ClusterRegion,
		ClusterZone:        opts.ClusterZone,
		DryRun:             opts.DryRun,
		ControlPlaneConfig: controlPlaneRestConfig,
		ClusterConfig:      clusterConfig,
	}

	clusterSecret, impersonatorSecret, err := util.ObtainCredentialsFromMemberCluster(clusterKubeClient, registerOption)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return nil
	}
	registerOption.Secret = *clusterSecret
	registerOption.ImpersonatorSecret = *impersonatorSecret
	err = util.RegisterClusterInControllerPlane(registerOption, controlPlaneKubeClient, generateClusterInControllerPlane)
	if err != nil {
		return err
	}

	fmt.Printf("cluster(%s) is joined successfully\n", opts.ClusterName)
	return nil
}

func generateClusterInControllerPlane(opts util.ClusterRegisterOption) (*clusterv1alpha1.Cluster, error) {
	clusterObj := &clusterv1alpha1.Cluster{}
	clusterObj.Name = opts.ClusterName
	clusterObj.Spec.SyncMode = clusterv1alpha1.Push
	clusterObj.Spec.APIEndpoint = opts.ClusterConfig.Host
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
