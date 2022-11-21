package unjoin

import (
	"context"
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
	"k8s.io/kubectl/pkg/util/templates"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	unjoinLong = templates.LongDesc(`
		Remove a cluster from Karmada control plane.`)

	unjoinExample = templates.Examples(`
		# Unjoin cluster from karmada control plane, but not to remove resources created by karmada in the unjoining cluster
		%[1]s unjoin CLUSTER_NAME

		# Unjoin cluster from karmada control plane and attempt to remove resources created by karmada in the unjoining cluster
		%[1]s unjoin CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>

		# Unjoin cluster from karmada control plane with timeout
		%[1]s unjoin CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG> --wait 2m`)
)

// NewCmdUnjoin defines the `unjoin` command that removes registration of a cluster from control plane.
func NewCmdUnjoin(f cmdutil.Factory, parentCommand string) *cobra.Command {
	opts := CommandUnjoinOption{}

	cmd := &cobra.Command{
		Use:                   "unjoin CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>",
		Short:                 "Remove a cluster from Karmada control plane",
		Long:                  unjoinLong,
		Example:               fmt.Sprintf(unjoinExample, parentCommand),
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

// CommandUnjoinOption holds all command options.
type CommandUnjoinOption struct {
	// ClusterNamespace holds namespace where the member cluster secrets are stored
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
	if len(args) > 0 {
		j.ClusterName = args[0]
	}

	// If '--cluster-context' not specified, take the cluster name as the context.
	if len(j.ClusterContext) == 0 {
		j.ClusterContext = j.ClusterName
	}

	return nil
}

// Validate ensures that command unjoin options are valid.
func (j *CommandUnjoinOption) Validate(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("only the cluster name is allowed as an argument")
	}
	if len(j.ClusterName) == 0 {
		return fmt.Errorf("cluster name is required")
	}
	if j.Wait < 0 {
		return fmt.Errorf(" --wait %v  must be a positive duration, e.g. 1m0s ", j.Wait)
	}
	return nil
}

// AddFlags adds flags to the specified FlagSet.
func (j *CommandUnjoinOption) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&j.ClusterNamespace, "cluster-namespace", options.DefaultKarmadaClusterNamespace, "Namespace in the control plane where member cluster secrets are stored.")
	flags.StringVar(&j.ClusterContext, "cluster-context", "",
		"Context name of cluster in kubeconfig. Only works when there are multiple contexts in the kubeconfig.")
	flags.StringVar(&j.ClusterKubeConfig, "cluster-kubeconfig", "",
		"Path of the cluster's kubeconfig.")
	flags.BoolVar(&j.forceDeletion, "force", false,
		"Delete cluster and secret resources even if resources in the cluster targeted for unjoin are not removed successfully.")
	flags.DurationVar(&j.Wait, "wait", 60*time.Second, "wait for the unjoin command execution process(default 60s), if there is no success after this time, timeout will be returned.")
	flags.BoolVar(&j.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
}

// Run is the implementation of the 'unjoin' command.
func (j *CommandUnjoinOption) Run(f cmdutil.Factory) error {
	klog.V(1).Infof("unjoining cluster. cluster name: %s", j.ClusterName)
	klog.V(1).Infof("unjoining cluster. cluster namespace: %s", j.ClusterNamespace)

	// Get control plane kube-apiserver client
	controlPlaneRestConfig, err := f.ToRawKubeConfigLoader().ClientConfig()
	if err != nil {
		klog.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			options.DefaultConfigFlags.Context, options.DefaultConfigFlags.KubeConfig, err)
		return err
	}

	var clusterConfig *rest.Config
	if j.ClusterKubeConfig != "" {
		// Get cluster config
		clusterConfig, err = apiclient.RestConfig(j.ClusterContext, j.ClusterKubeConfig)
		if err != nil {
			klog.V(1).Infof("failed to get unjoining cluster config. error: %v", err)
			return err
		}
	}

	return j.RunUnJoinCluster(controlPlaneRestConfig, clusterConfig)
}

// RunUnJoinCluster unJoin the cluster from karmada.
func (j *CommandUnjoinOption) RunUnJoinCluster(controlPlaneRestConfig, clusterConfig *rest.Config) error {
	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)

	// delete the cluster object in host cluster that associates the unjoining cluster
	err := j.deleteClusterObject(controlPlaneKarmadaClient)
	if err != nil {
		klog.Errorf("Failed to delete cluster object. cluster name: %s, error: %v", j.ClusterName, err)
		return err
	}

	// Attempt to delete the cluster role, cluster rolebindings and service account from the unjoining cluster
	// if user provides the kubeconfig of cluster
	if clusterConfig != nil {
		clusterKubeClient := kubeclient.NewForConfigOrDie(clusterConfig)

		klog.V(1).Infof("unjoining cluster config. endpoint: %s", clusterConfig.Host)

		// delete RBAC resource from unjoining cluster
		err = deleteRBACResources(clusterKubeClient, j.ClusterName, j.forceDeletion, j.DryRun)
		if err != nil {
			klog.Errorf("Failed to delete RBAC resource in unjoining cluster %q: %v", j.ClusterName, err)
			return err
		}

		// delete service account from unjoining cluster
		err = deleteServiceAccount(clusterKubeClient, j.ClusterNamespace, j.ClusterName, j.forceDeletion, j.DryRun)
		if err != nil {
			klog.Errorf("Failed to delete service account in unjoining cluster %q: %v", j.ClusterName, err)
			return err
		}

		// delete namespace from unjoining cluster
		err = deleteNamespaceFromUnjoinCluster(clusterKubeClient, j.ClusterNamespace, j.ClusterName, j.forceDeletion, j.DryRun)
		if err != nil {
			klog.Errorf("Failed to delete namespace in unjoining cluster %q: %v", j.ClusterName, err)
			return err
		}
	}

	return nil
}

// deleteClusterObject delete the cluster object in host cluster that associates the unjoining cluster
func (j *CommandUnjoinOption) deleteClusterObject(controlPlaneKarmadaClient *karmadaclientset.Clientset) error {
	if j.DryRun {
		return nil
	}

	err := controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Delete(context.TODO(), j.ClusterName, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("no cluster object %s found in karmada control Plane", j.ClusterName)
	}
	if err != nil {
		klog.Errorf("Failed to delete cluster object. cluster name: %s, error: %v", j.ClusterName, err)
		return err
	}

	// make sure the given cluster object has been deleted
	err = wait.Poll(1*time.Second, j.Wait, func() (done bool, err error) {
		_, err = controlPlaneKarmadaClient.ClusterV1alpha1().Clusters().Get(context.TODO(), j.ClusterName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			klog.Errorf("Failed to get cluster %s. err: %v", j.ClusterName, err)
			return false, err
		}
		klog.Infof("Waiting for the cluster object %s to be deleted", j.ClusterName)
		return false, nil
	})
	if err != nil {
		klog.Errorf("Failed to delete cluster object. cluster name: %s, error: %v", j.ClusterName, err)
		return err
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
