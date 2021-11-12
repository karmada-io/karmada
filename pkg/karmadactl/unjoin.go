package karmadactl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

const (
	defaultAgentNamespace          = "karmada-system"
	defaultAgentName               = "karmada-agent"
	defaultAgentClusterRole        = "karmada-agent"
	defaultAgentClusterRoleBinding = "karmada-agent"
	defaultAgentServiceAccount     = "karmada-agent-sa"
	defaultAgentKubeconfig         = "karmada-kubeconfig"
)

var (
	unjoinShort   = `Remove the registration of a cluster from control plane`
	unjoinLong    = `Unjoin removes the registration of a cluster from control plane.`
	unjoinExample = `
# Unjoin cluster from karmada control plane
%s unjoin CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>

# Unjoin cluster from karmada control plane with timeout
%s unjoin CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG> --wait 2m

# Unjoin cluster from karmada control plane, if the agent was deployed in custom namespace
%s unjoin CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG> --agent-namespace=<CUSTOM_NAMESPACE>
`
)

// NewCmdUnjoin defines the `unjoin` command that removes registration of a cluster from control plane.
func NewCmdUnjoin(cmdOut io.Writer, karmadaConfig KarmadaConfig, cmdStr string) *cobra.Command {
	opts := CommandUnjoinOption{}

	cmd := &cobra.Command{
		Use:          "unjoin CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>",
		Short:        unjoinShort,
		Long:         unjoinLong,
		Example:      getUnjoinExample(cmdStr),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := RunUnjoin(cmdOut, karmadaConfig, opts); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	opts.AddFlags(flags)

	return cmd
}

func getUnjoinExample(cmdStr string) string {
	return fmt.Sprintf(unjoinExample, cmdStr, cmdStr, cmdStr)
}

// CommandUnjoinOption holds all command options.
type CommandUnjoinOption struct {
	options.GlobalCommandOptions

	// ClusterName is the cluster's name that we are going to join with.
	ClusterName string

	// ClusterContext is the cluster's context that we are going to join with.
	ClusterContext string

	// ClusterKubeConfig is the cluster's kubeconfig path.
	ClusterKubeConfig string

	// AgentNamespace is the namespace of agent in pull mode.
	AgentNamespace string

	// AgentName is the name of agent in pull mode.
	AgentName string

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

	flags.StringVar(&j.ClusterContext, "cluster-context", "",
		"Context name of cluster in kubeconfig. Only works when there are multiple contexts in the kubeconfig.")
	flags.StringVar(&j.ClusterKubeConfig, "cluster-kubeconfig", "",
		"Path of the cluster's kubeconfig.")
	flags.BoolVar(&j.forceDeletion, "force", false,
		"Delete cluster and secret resources even if resources in the cluster targeted for unjoin are not removed successfully.")

	flags.StringVar(&j.AgentNamespace, "agent-namespace", defaultAgentNamespace, "The namespace of agent in pull mode.")
	flags.StringVar(&j.AgentName, "agent-name", defaultAgentName, "The name of agent in pull mode.")

	flags.DurationVar(&j.Wait, "wait", 60*time.Second, "wait for the unjoin command execution process(default 60s), if there is no success after this time, timeout will be returned.")
}

// RunUnjoin is the implementation of the 'unjoin' command.
func RunUnjoin(_ io.Writer, karmadaConfig KarmadaConfig, opts CommandUnjoinOption) error {
	klog.V(1).Infof("unjoining cluster. cluster name: %s", opts.ClusterName)
	klog.V(1).Infof("unjoining cluster. cluster namespace: %s", opts.ClusterNamespace)

	// Get control plane karmada-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, opts.KubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err)
	}

	var clusterConfig *rest.Config
	if opts.ClusterKubeConfig != "" {
		// Get cluster config
		clusterConfig, err = karmadaConfig.GetRestConfig(opts.ClusterContext, opts.ClusterKubeConfig)
		if err != nil {
			return fmt.Errorf("failed to get unjoining cluster config. error: %v", err)
		}
	}

	return UnJoinCluster(controlPlaneRestConfig, clusterConfig, opts)
}

// UnJoinCluster unJoin the cluster from karmada.
func UnJoinCluster(controlPlaneRestConfig, clusterConfig *rest.Config, opts CommandUnjoinOption) (err error) {
	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)

	// get SyncMode
	cluster, err := controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), opts.ClusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get the unjoining cluster. cluster name: %s, error: %v", opts.ClusterName, err)
	}
	syncMode := cluster.Spec.SyncMode

	// delete the cluster object in host cluster that associates the unjoining cluster
	err = deleteClusterObject(controlPlaneKarmadaClient, opts)
	if err != nil {
		return fmt.Errorf("failed to delete cluster object. cluster name: %s, error: %v", opts.ClusterName, err)
	}

	// Attempt to delete the cluster role, cluster rolebindings and service account from the unjoining cluster
	// if user provides the kubeconfig of cluster
	if clusterConfig != nil {
		clusterKubeClient := kubeclient.NewForConfigOrDie(clusterConfig)

		klog.V(1).Infof("unjoining cluster config. endpoint: %s", clusterConfig.Host)

		if syncMode == clusterv1alpha1.Push {
			// delete RBAC resource from unjoining cluster
			serviceAccountName := names.GenerateServiceAccountName(opts.ClusterName)
			clusterRoleName := names.GenerateRoleName(serviceAccountName)
			clusterRoleBindingName := clusterRoleName

			err = deleteRBACResources(clusterKubeClient, clusterRoleName, clusterRoleBindingName, opts.ClusterName, opts.forceDeletion, opts.DryRun)
			if err != nil {
				return fmt.Errorf("failed to delete RBAC resource in unjoining cluster %q: %v", opts.ClusterName, err)
			}

			// delete service account from unjoining cluster
			err = deleteServiceAccount(clusterKubeClient, opts.ClusterNamespace, serviceAccountName, opts.ClusterName, opts.forceDeletion, opts.DryRun)
			if err != nil {
				return fmt.Errorf("failed to delete service account in unjoining cluster %q: %v", opts.ClusterName, err)
			}

			// delete namespace from unjoining cluster
			err = deleteNamespaceFromUnjoinCluster(clusterKubeClient, opts.ClusterNamespace, opts.ClusterName, opts.forceDeletion, opts.DryRun)
			if err != nil {
				return fmt.Errorf("failed to delete namespace in unjoining cluster %q: %v", opts.ClusterName, err)
			}
		}

		if syncMode == clusterv1alpha1.Pull {
			// delete RBAC resource from unjoining cluster
			err = deleteRBACResources(clusterKubeClient, defaultAgentClusterRole, defaultAgentClusterRoleBinding, opts.ClusterName, opts.forceDeletion, opts.DryRun)
			if err != nil {
				return fmt.Errorf("failed to delete RBAC resource in unjoining cluster %q: %v", opts.ClusterName, err)
			}

			// delete karmada-agent deployment from unjoining cluster
			err = deleteAgentDeployment(clusterKubeClient, opts.AgentNamespace, opts.AgentName, opts.ClusterName, opts.forceDeletion, opts.DryRun)
			if err != nil {
				return fmt.Errorf("failed to delete deployment in unjoining cluster %q: %v", opts.ClusterName, err)
			}

			// delete service account from unjoining cluster
			err = deleteServiceAccount(clusterKubeClient, opts.AgentNamespace, defaultAgentServiceAccount, opts.ClusterName, opts.forceDeletion, opts.DryRun)
			if err != nil {
				return fmt.Errorf("failed to delete service account in unjoining cluster %q: %v", opts.ClusterName, err)
			}

			// delete karmada-kubeconfig secret from unjoin cluster
			err = deleteAgentSecret(clusterKubeClient, opts.AgentNamespace, defaultAgentKubeconfig, opts.ClusterName, opts.forceDeletion, opts.DryRun)
			if err != nil {
				return fmt.Errorf("failed to delete secret in unjoining cluster %q: %v", opts.ClusterName, err)
			}
		}
	}

	return nil
}

// deleteRBACResources deletes the cluster role, cluster rolebindings from the unjoining cluster.
func deleteRBACResources(clusterKubeClient kubeclient.Interface, clusterRoleName, clusterRoleBindingName, unjoiningClusterName string, forceDeletion, dryRun bool) error {
	if dryRun {
		return nil
	}

	err := util.DeleteClusterRoleBinding(clusterKubeClient, clusterRoleBindingName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete cluster role binding %q in unjoining cluster %q: %v.", clusterRoleBindingName, unjoiningClusterName, err)
	}

	err = util.DeleteClusterRole(clusterKubeClient, clusterRoleName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete cluster role %q in unjoining cluster %q: %v.", clusterRoleName, unjoiningClusterName, err)
	}

	return nil
}

// deleteServiceAccount deletes the service account from the unjoining cluster.
func deleteServiceAccount(clusterKubeClient kubeclient.Interface, namespace, serviceAccountName, unjoiningClusterName string, forceDeletion, dryRun bool) error {
	if dryRun {
		return nil
	}

	err := util.DeleteServiceAccount(clusterKubeClient, namespace, serviceAccountName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete service account %q in unjoining cluster %q: %v.", serviceAccountName, unjoiningClusterName, err)
	}

	return nil
}

// deleteAgentDeployment deletes the karmada-agent deployment from the unjoining cluster.
func deleteAgentDeployment(clusterKubeClient kubeclient.Interface, namespace, deploymentName, unjoiningClusterName string, forceDeletion, dryRun bool) error {
	if dryRun {
		return nil
	}

	err := util.DeleteDeployment(clusterKubeClient, namespace, deploymentName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete deployment %q in unjoining cluster %q: %v.", deploymentName, unjoiningClusterName, err)
	}

	return nil
}

func deleteAgentSecret(clusterKubeClient kubeclient.Interface, namespace, secretName, unjoiningClusterName string, forceDeletion, dryRun bool) error {
	if dryRun {
		return nil
	}

	err := util.DeleteSecret(clusterKubeClient, namespace, secretName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete secret %q in unjoining cluster %q: %v.", secretName, unjoiningClusterName, err)
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
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to delete cluster object. cluster name: %s, error: %v", opts.ClusterName, err)
	}

	// make sure the given cluster object has been deleted
	err = wait.Poll(1*time.Second, opts.Wait, func() (done bool, err error) {
		_, err = controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), opts.ClusterName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			klog.V(1).Infof("failed to get cluster %s. err: %v", opts.ClusterName, err)
			return false, err
		}
		klog.Infof("Waiting for the cluster object %s to be deleted", opts.ClusterName)
		return false, nil
	})
	if err != nil {
		klog.V(1).Infof("failed to delete cluster object. cluster name: %s, error: %v", opts.ClusterName, err)
		return err
	}

	klog.Infof("%s has been unjoined successfully", opts.ClusterName)

	return nil
}
