package karmadactl

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	unjoinShort = `Remove the registration of a cluster from control plane`
	unjoinLong  = `Unjoin removes the registration of a cluster from control plane.`
)

// NewCmdUnjoin defines the `unjoin` command that removes registration of a cluster from control plane.
func NewCmdUnjoin(karmadaConfig KarmadaConfig, parentCommand string) *cobra.Command {
	opts := CommandUnjoinOption{}

	cmd := &cobra.Command{
		Use:          "unjoin CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>",
		Short:        unjoinShort,
		Long:         unjoinLong,
		Example:      unjoinExample(parentCommand),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := RunUnjoin(karmadaConfig, opts); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	opts.AddFlags(flags)

	return cmd
}

func unjoinExample(parentCommand string) string {
	example := `
# Unjoin cluster from karamada control plane, but not to remove resources created by karmada in the unjoining cluster` + "\n" +
		fmt.Sprintf("%s unjoin CLUSTER_NAME", parentCommand) + `

# Unjoin cluster from karamada control plane and attempt to remove resources created by karmada in the unjoining cluster` + "\n" +
		fmt.Sprintf("%s unjoin CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>", parentCommand) + `
		
# Unjoin cluster from karamada control plane with timeout` + "\n" +
		fmt.Sprintf("%s unjoin CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG> --wait 2m", parentCommand)
	return example
}

// CommandUnjoinOption holds all command options.
type CommandUnjoinOption struct {
	options.GlobalCommandOptions

	// ClusterNamespace holds the namespace name where the member cluster objects are stored.
	ClusterNamespace string

	// ClusterName is the cluster's name that we are going to join with.
	ClusterName string

	// ClusterContext is the cluster's context that we are going to join with.
	ClusterContext string

	// ClusterKubeConfig is the cluster's kubeconfig path.
	ClusterKubeConfig string

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool

	forceDeletion bool

	// Wait tells maximum command execution time
	Wait time.Duration
}

// Complete ensures that options are valid and marshals them if necessary.
func (j *CommandUnjoinOption) Complete(args []string) error {
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

// Validate ensures that command unjoin options are valid.
func (j *CommandUnjoinOption) Validate() error {
	if j.Wait < 0 {
		return fmt.Errorf(" --wait %v  must be a positive duration, e.g. 1m0s ", j.Wait)
	}
	return nil
}

// AddFlags adds flags to the specified FlagSet.
func (j *CommandUnjoinOption) AddFlags(flags *pflag.FlagSet) {
	j.GlobalCommandOptions.AddFlags(flags)

	flags.StringVar(&j.ClusterNamespace, "cluster-namespace", options.DefaultKarmadaClusterNamespace, "Namespace in the control plane where member cluster are stored.")
	flags.StringVar(&j.ClusterContext, "cluster-context", "",
		"Context name of cluster in kubeconfig. Only works when there are multiple contexts in the kubeconfig.")
	flags.StringVar(&j.ClusterKubeConfig, "cluster-kubeconfig", "",
		"Path of the cluster's kubeconfig.")
	flags.BoolVar(&j.forceDeletion, "force", false,
		"Delete cluster and secret resources even if resources in the cluster targeted for unjoin are not removed successfully.")
	flags.DurationVar(&j.Wait, "wait", 60*time.Second, "wait for the unjoin command execution process(default 60s), if there is no success after this time, timeout will be returned.")
	flags.BoolVar(&j.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}

// RunUnjoin is the implementation of the 'unjoin' command.
func RunUnjoin(karmadaConfig KarmadaConfig, opts CommandUnjoinOption) error {
	klog.V(1).Infof("unjoining cluster. cluster name: %s", opts.ClusterName)
	klog.V(1).Infof("unjoining cluster. cluster namespace: %s", opts.ClusterNamespace)

	// Get control plane kube-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, opts.KubeConfig)
	if err != nil {
		klog.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err)
		return err
	}

	var clusterConfig *rest.Config
	if opts.ClusterKubeConfig != "" {
		// Get cluster config
		clusterConfig, err = karmadaConfig.GetRestConfig(opts.ClusterContext, opts.ClusterKubeConfig)
		if err != nil {
			klog.V(1).Infof("failed to get unjoining cluster config. error: %v", err)
			return err
		}
	}

	return UnJoinCluster(controlPlaneRestConfig, clusterConfig, opts)
}

// UnJoinCluster unJoin the cluster from karmada.
func UnJoinCluster(controlPlaneRestConfig, clusterConfig *rest.Config, opts CommandUnjoinOption) (err error) {
	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)

	// delete the cluster object in host cluster that associates the unjoining cluster
	err = deleteClusterObject(controlPlaneKarmadaClient, opts)
	if err != nil {
		klog.Errorf("Failed to delete cluster object. cluster name: %s, error: %v", opts.ClusterName, err)
		return err
	}

	// Attempt to delete the cluster role, cluster rolebindings and service account from the unjoining cluster
	// if user provides the kubeconfig of cluster
	if clusterConfig != nil {
		clusterKubeClient := kubeclient.NewForConfigOrDie(clusterConfig)

		klog.V(1).Infof("unjoining cluster config. endpoint: %s", clusterConfig.Host)

		// delete RBAC resource from unjoining cluster
		err = deleteRBACResources(clusterKubeClient, opts.ClusterName, opts.forceDeletion, opts.DryRun)
		if err != nil {
			klog.Errorf("Failed to delete RBAC resource in unjoining cluster %q: %v", opts.ClusterName, err)
			return err
		}

		// delete service account from unjoining cluster
		err = deleteServiceAccount(clusterKubeClient, opts.ClusterNamespace, opts.ClusterName, opts.forceDeletion, opts.DryRun)
		if err != nil {
			klog.Errorf("Failed to delete service account in unjoining cluster %q: %v", opts.ClusterName, err)
			return err
		}

		// delete namespace from unjoining cluster
		err = deleteNamespaceFromUnjoinCluster(clusterKubeClient, opts.ClusterNamespace, opts.ClusterName, opts.forceDeletion, opts.DryRun)
		if err != nil {
			klog.Errorf("Failed to delete namespace in unjoining cluster %q: %v", opts.ClusterName, err)
			return err
		}
	}

	return nil
}

// deleteRBACResources deletes the cluster role, cluster rolebindings from the unjoining cluster.
func deleteRBACResources(clusterKubeClient kubeclient.Interface, unjoiningClusterName string, forceDeletion, dryRun bool) error {
	if dryRun {
		return nil
	}

	serviceAccountName := names.GenerateServiceAccountName(unjoiningClusterName)
	clusterRoleName := names.GenerateRoleName(serviceAccountName)
	clusterRoleBindingName := clusterRoleName

	err := util.DeleteClusterRoleBinding(clusterKubeClient, clusterRoleBindingName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete cluster role binding %q for service account %q in unjoining cluster %q: %v.", clusterRoleBindingName, serviceAccountName, unjoiningClusterName, err)
	}

	err = util.DeleteClusterRole(clusterKubeClient, clusterRoleName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete cluster role %q for service account %q in unjoining cluster %q: %v.", clusterRoleName, serviceAccountName, unjoiningClusterName, err)
	}

	return nil
}

// deleteServiceAccount deletes the service account from the unjoining cluster.
func deleteServiceAccount(clusterKubeClient kubeclient.Interface, namespace, unjoiningClusterName string, forceDeletion, dryRun bool) error {
	if dryRun {
		return nil
	}

	serviceAccountName := names.GenerateServiceAccountName(unjoiningClusterName)
	err := util.DeleteServiceAccount(clusterKubeClient, namespace, serviceAccountName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete service account %q in unjoining cluster %q: %v.", serviceAccountName, unjoiningClusterName, err)
	}

	return nil
}

// deleteNSFromUnjoinCluster deletes the namespace from the unjoining cluster.
func deleteNamespaceFromUnjoinCluster(clusterKubeClient kubeclient.Interface, namespace, unjoiningClusterName string, forceDeletion, dryRun bool) error {
	if dryRun {
		return nil
	}

	err := util.DeleteNamespace(clusterKubeClient, namespace)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete namespace %q in unjoining cluster %q: %v.", namespace, unjoiningClusterName, err)
	}

	return nil
}

// deleteClusterObject delete the cluster object in host cluster that associates the unjoining cluster
func deleteClusterObject(controlPlaneKarmadaClient *karmadaclientset.Clientset, opts CommandUnjoinOption) error {
	if opts.DryRun {
		return nil
	}

	err := controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Delete(context.TODO(), opts.ClusterName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("no cluster object %s found in karmada control Plane", opts.ClusterName)
	}
	if err != nil {
		klog.Errorf("Failed to delete cluster object. cluster name: %s, error: %v", opts.ClusterName, err)
		return err
	}

	// make sure the given cluster object has been deleted
	err = wait.Poll(1*time.Second, opts.Wait, func() (done bool, err error) {
		_, err = controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), opts.ClusterName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			klog.Errorf("Failed to get cluster %s. err: %v", opts.ClusterName, err)
			return false, err
		}
		klog.Infof("Waiting for the cluster object %s to be deleted", opts.ClusterName)
		return false, nil
	})
	if err != nil {
		klog.Errorf("Failed to delete cluster object. cluster name: %s, error: %v", opts.ClusterName, err)
		return err
	}

	return nil
}
