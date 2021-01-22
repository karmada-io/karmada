package karmadactl

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	unjoinLong = `Unjoin removes the registration of a member cluster from control plane.`

	unjoinExample = `
karmadactl unjoin CLUSTER_NAME --member-cluster-kubeconfig=<KUBECONFIG>
`
)

// NewCmdUnjoin defines the `unjoin` command that removes registration of a member cluster from control plane.
func NewCmdUnjoin(cmdOut io.Writer, karmadaConfig KarmadaConfig) *cobra.Command {
	opts := CommandUnjoinOption{}

	cmd := &cobra.Command{
		Use:     "unjoin CLUSTER_NAME --member-cluster-kubeconfig=<KUBECONFIG>",
		Short:   "Remove the registration of a member cluster from control plane",
		Long:    unjoinLong,
		Example: unjoinExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := opts.Complete(args)
			if err != nil {
				klog.Errorf("Error: %v", err)
				return
			}

			err = RunUnjoin(cmdOut, karmadaConfig, opts)
			if err != nil {
				klog.Errorf("Error: %v", err)
				return
			}
		},
	}

	flags := cmd.Flags()
	opts.AddFlags(flags)

	return cmd
}

// CommandUnjoinOption holds all command options.
type CommandUnjoinOption struct {
	options.GlobalCommandOptions

	// MemberClusterName is the member cluster's name that we are going to join with.
	MemberClusterName string

	// MemberClusterContext is the member cluster's context that we are going to join with.
	MemberClusterContext string

	// MemberClusterKubeConfig is the member cluster's kubeconfig path.
	MemberClusterKubeConfig string

	forceDeletion bool
}

// Complete ensures that options are valid and marshals them if necessary.
func (j *CommandUnjoinOption) Complete(args []string) error {
	// Get member cluster name from the command args.
	if len(args) == 0 {
		return errors.New("member cluster name is required")
	}
	j.MemberClusterName = args[0]

	// If '--member-cluster-context' not specified, take the cluster name as the context.
	if len(j.MemberClusterContext) == 0 {
		j.MemberClusterContext = j.MemberClusterName
	}

	return nil
}

// AddFlags adds flags to the specified FlagSet.
func (j *CommandUnjoinOption) AddFlags(flags *pflag.FlagSet) {
	j.GlobalCommandOptions.AddFlags(flags)

	flags.StringVar(&j.MemberClusterContext, "member-cluster-context", "",
		"Context name of member cluster in kubeconfig. Only works when there are multiple contexts in the kubeconfig.")
	flags.StringVar(&j.MemberClusterKubeConfig, "member-cluster-kubeconfig", "",
		"Path of the member cluster's kubeconfig.")
	flags.BoolVar(&j.forceDeletion, "force", false,
		"Delete cluster and secret resources even if resources in the member cluster targeted for unjoin are not removed successfully.")
}

// RunUnjoin is the implementation of the 'unjoin' command.
// TODO(RainbowMango): consider to remove the 'KarmadaConfig'.
func RunUnjoin(cmdOut io.Writer, karmadaConfig KarmadaConfig, opts CommandUnjoinOption) error {
	klog.V(1).Infof("unjoining member cluster. member cluster name: %s", opts.MemberClusterName)
	klog.V(1).Infof("unjoining member cluster. cluster namespace: %s", opts.ClusterNamespace)

	// Get control plane kube-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.ClusterContext, opts.KubeConfig)
	if err != nil {
		klog.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			opts.ClusterContext, opts.KubeConfig, err)
		return err
	}

	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)
	controlPlaneKubeClient := kubeclient.NewForConfigOrDie(controlPlaneRestConfig)

	// todo: taint member cluster object instead of deleting execution space.
	//  Once the member cluster is tainted, eviction controller will delete all propagationwork in the execution space of the member cluster.
	executionSpaceName, err := names.GenerateExecutionSpaceName(opts.MemberClusterName)
	if err != nil {
		return err
	}

	err = deleteExecutionSpace(controlPlaneKubeClient, executionSpaceName, opts.DryRun)
	if err != nil {
		klog.Errorf("Failed to delete execution space %s, error: %v", executionSpaceName, err)
		return err
	}

	// Attempt to delete the cluster role, cluster rolebindings and service account from the unjoining member cluster
	// if user provides the kubeconfig of member cluster
	if opts.MemberClusterKubeConfig != "" {
		// Get member cluster config
		memberClusterConfig, err := karmadaConfig.GetRestConfig(opts.MemberClusterContext, opts.MemberClusterKubeConfig)
		if err != nil {
			klog.V(1).Infof("failed to get unjoining member cluster config. error: %v", err)
			return err
		}
		memberClusterKubeClient := kubeclient.NewForConfigOrDie(memberClusterConfig)

		klog.V(1).Infof("unjoining member cluster config. endpoint: %s", memberClusterConfig.Host)

		// delete RBAC resource from unjoining member cluster
		err = deleteRBACResources(memberClusterKubeClient, opts.MemberClusterName, opts.forceDeletion, opts.DryRun)
		if err != nil {
			klog.Errorf("Failed to delete RBAC resource in unjoining member cluster %q: %v", opts.MemberClusterName, err)
			return err
		}

		// delete service account from unjoining member cluster
		err = deleteServiceAccount(memberClusterKubeClient, opts.ClusterNamespace, opts.MemberClusterName, opts.forceDeletion, opts.DryRun)
		if err != nil {
			klog.Errorf("Failed to delete service account in unjoining member cluster %q: %v", opts.MemberClusterName, err)
			return err
		}

		// delete namespace from unjoining member cluster
		err = deleteNamespaceFromUnjoinCluster(memberClusterKubeClient, opts.ClusterNamespace, opts.MemberClusterName, opts.forceDeletion, opts.DryRun)
		if err != nil {
			klog.Errorf("Failed to delete namespace in unjoining member cluster %q: %v", opts.MemberClusterName, err)
			return err
		}
	}

	// delete the member cluster object in host cluster that associates the unjoining member cluster
	err = deleteMemberClusterObject(controlPlaneKarmadaClient, opts.MemberClusterName, opts.DryRun)
	if err != nil {
		klog.Errorf("Failed to delete member cluster object. cluster name: %s, error: %v", opts.MemberClusterName, err)
		return err
	}

	return nil
}

// deleteRBACResources deletes the cluster role, cluster rolebindings from the unjoining member cluster.
func deleteRBACResources(memberClusterKubeClient kubeclient.Interface, unjoiningClusterName string, forceDeletion, dryRun bool) error {
	if dryRun {
		return nil
	}

	serviceAccountName := names.GenerateServiceAccountName(unjoiningClusterName)
	clusterRoleName := names.GenerateRoleName(serviceAccountName)
	clusterRoleBindingName := clusterRoleName

	err := util.DeleteClusterRoleBinding(memberClusterKubeClient, clusterRoleBindingName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete cluster role binding %q for service account %q in unjoining member cluster %q: %v.", clusterRoleBindingName, serviceAccountName, unjoiningClusterName, err)
	}

	err = util.DeleteClusterRole(memberClusterKubeClient, clusterRoleName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete cluster role %q for service account %q in unjoining member cluster %q: %v.", clusterRoleName, serviceAccountName, unjoiningClusterName, err)
	}

	return nil
}

// deleteServiceAccount deletes the service account from the unjoining member cluster.
func deleteServiceAccount(memberClusterKubeClient kubeclient.Interface, namespace, unjoiningClusterName string, forceDeletion, dryRun bool) error {
	if dryRun {
		return nil
	}

	serviceAccountName := names.GenerateServiceAccountName(unjoiningClusterName)
	err := util.DeleteServiceAccount(memberClusterKubeClient, namespace, serviceAccountName)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete service account %q in unjoining member cluster %q: %v.", serviceAccountName, unjoiningClusterName, err)
	}

	return nil
}

// deleteNSFromUnjoinCluster deletes the namespace from the unjoining member cluster.
func deleteNamespaceFromUnjoinCluster(memberClusterKubeClient kubeclient.Interface, namespace, unjoiningClusterName string, forceDeletion, dryRun bool) error {
	if dryRun {
		return nil
	}

	err := util.DeleteNamespace(memberClusterKubeClient, namespace)
	if err != nil {
		if !forceDeletion {
			return err
		}
		klog.Errorf("Force deletion. Could not delete namespace %q in unjoining member cluster %q: %v.", namespace, unjoiningClusterName, err)
	}

	return nil
}

func deleteExecutionSpace(hostClient kubeclient.Interface, namespace string, dryRun bool) error {
	if dryRun {
		return nil
	}

	err := util.DeleteNamespace(hostClient, namespace)
	if err != nil {
		return err
	}

	// make sure the execution space has been deleted
	err = wait.Poll(1*time.Second, 30*time.Second, func() (done bool, err error) {
		exist, err := util.IsNamespaceExist(hostClient, namespace)
		if err != nil {
			klog.Errorf("Failed to get execution space %s. err: %v", namespace, err)
			return false, err
		}
		if !exist {
			return true, nil
		}
		klog.Infof("Waiting for the execution space %s to be deleted", namespace)
		return false, nil
	})
	if err != nil {
		klog.Errorf("Failed to delete execution space %s, error: %v", namespace, err)
		return err
	}

	return nil
}

// deleteMemberClusterObject delete the member cluster object in host cluster that associates the unjoining member cluster
func deleteMemberClusterObject(controlPlaneKarmadaClient *karmadaclientset.Clientset, memberClusterName string, dryRun bool) error {
	if dryRun {
		return nil
	}

	err := controlPlaneKarmadaClient.MemberclusterV1alpha1().MemberClusters().Delete(context.TODO(), memberClusterName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		klog.Errorf("Failed to delete member cluster object. cluster name: %s, error: %v", memberClusterName, err)
		return err
	}

	// make sure the given member cluster object has been deleted
	err = wait.Poll(1*time.Second, 30*time.Second, func() (done bool, err error) {
		_, err = controlPlaneKarmadaClient.MemberclusterV1alpha1().MemberClusters().Get(context.TODO(), memberClusterName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			klog.Errorf("Failed to get member cluster %s. err: %v", memberClusterName, err)
			return false, err
		}
		klog.Infof("Waiting for the member cluster object %s to be deleted", memberClusterName)
		return false, nil
	})
	if err != nil {
		klog.Errorf("Failed to delete member cluster object. cluster name: %s, error: %v", memberClusterName, err)
		return err
	}

	return nil
}
