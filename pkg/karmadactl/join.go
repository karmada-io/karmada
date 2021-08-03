package karmadactl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/validation"
)

var (
	joinLong = `Join registers a cluster to control plane.`

	joinExample = `
karmadactl join CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>
`
)

var (
	// Policy rules allowing full access to resources in the cluster or namespace.
	namespacedPolicyRules = []rbacv1.PolicyRule{
		{
			Verbs:     []string{rbacv1.VerbAll},
			APIGroups: []string{rbacv1.APIGroupAll},
			Resources: []string{rbacv1.ResourceAll},
		},
	}
	clusterPolicyRules = []rbacv1.PolicyRule{
		namespacedPolicyRules[0],
		{
			NonResourceURLs: []string{rbacv1.NonResourceAll},
			Verbs:           []string{"get"},
		},
	}
)

var clusterResourceKind = clusterv1alpha1.SchemeGroupVersion.WithKind("Cluster")

const (
	// TODO(RainbowMango) token and caData key both used by command and controller.
	// It's better to put them to a common place, such as API definition.
	tokenKey  = "token"
	caDataKey = "caBundle"
)

// NewCmdJoin defines the `join` command that registers a cluster.
func NewCmdJoin(cmdOut io.Writer, karmadaConfig KarmadaConfig) *cobra.Command {
	opts := CommandJoinOption{}

	cmd := &cobra.Command{
		Use:     "join CLUSTER_NAME --cluster-kubeconfig=<KUBECONFIG>",
		Short:   "Register a cluster to control plane",
		Long:    joinLong,
		Example: joinExample,
		Run: func(cmd *cobra.Command, args []string) {
			// Set default values
			err := opts.Complete(args)
			if err != nil {
				klog.Errorf("Error: %v", err)
				return
			}

			if errs := opts.Validate(); len(errs) != 0 {
				klog.Error(utilerrors.NewAggregate(errs).Error())
				return
			}

			err = RunJoin(cmdOut, karmadaConfig, opts)
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

// CommandJoinOption holds all command options.
type CommandJoinOption struct {
	options.GlobalCommandOptions

	// ClusterName is the cluster's name that we are going to join with.
	ClusterName string

	// ClusterContext is the cluster's context that we are going to join with.
	ClusterContext string

	// ClusterKubeConfig is the cluster's kubeconfig path.
	ClusterKubeConfig string
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
func (j *CommandJoinOption) Validate() []error {
	var errs []error
	if errMsgs := validation.ValidateClusterName(j.ClusterName); len(errMsgs) != 0 {
		errs = append(errs, fmt.Errorf("invalid cluster name(%s): %s", j.ClusterName, strings.Join(errMsgs, ";")))
	}

	return errs
}

// AddFlags adds flags to the specified FlagSet.
func (j *CommandJoinOption) AddFlags(flags *pflag.FlagSet) {
	j.GlobalCommandOptions.AddFlags(flags)

	flags.StringVar(&j.ClusterContext, "cluster-context", "",
		"Context name of cluster in kubeconfig. Only works when there are multiple contexts in the kubeconfig.")
	flags.StringVar(&j.ClusterKubeConfig, "cluster-kubeconfig", "",
		"Path of the cluster's kubeconfig.")
}

// RunJoin is the implementation of the 'join' command.
//nolint:gocyclo
// Note: ignore the cyclomatic complexity issue to get gocyclo on board. Tracked by: https://github.com/karmada-io/karmada/issues/460
func RunJoin(cmdOut io.Writer, karmadaConfig KarmadaConfig, opts CommandJoinOption) error {
	klog.V(1).Infof("joining cluster. cluster name: %s", opts.ClusterName)
	klog.V(1).Infof("joining cluster. cluster namespace: %s", opts.ClusterNamespace)

	// Get control plane karmada-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.KarmadaContext, opts.KubeConfig)
	if err != nil {
		klog.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			opts.KarmadaContext, opts.KubeConfig, err)
		return err
	}

	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)
	controlPlaneKubeClient := kubeclient.NewForConfigOrDie(controlPlaneRestConfig)

	// Get cluster config
	clusterConfig, err := karmadaConfig.GetRestConfig(opts.ClusterContext, opts.ClusterKubeConfig)
	if err != nil {
		klog.V(1).Infof("failed to get joining cluster config. error: %v", err)
		return err
	}
	clusterKubeClient := kubeclient.NewForConfigOrDie(clusterConfig)

	klog.V(1).Infof("joining cluster config. endpoint: %s", clusterConfig.Host)

	// ensure namespace where the cluster object be stored exists in control plane.
	if _, err := ensureNamespaceExist(controlPlaneKubeClient, opts.ClusterNamespace, opts.DryRun); err != nil {
		return err
	}
	// ensure namespace where the karmada control plane credential be stored exists in cluster.
	if _, err := ensureNamespaceExist(clusterKubeClient, opts.ClusterNamespace, opts.DryRun); err != nil {
		return err
	}

	// create a ServiceAccount in cluster.
	serviceAccountObj := &corev1.ServiceAccount{}
	serviceAccountObj.Namespace = opts.ClusterNamespace
	serviceAccountObj.Name = names.GenerateServiceAccountName(opts.ClusterName)
	if serviceAccountObj, err = ensureServiceAccountExist(clusterKubeClient, serviceAccountObj, opts.DryRun); err != nil {
		return err
	}

	// create a ClusterRole in cluster.
	clusterRole := &rbacv1.ClusterRole{}
	clusterRole.Name = names.GenerateRoleName(serviceAccountObj.Name)
	clusterRole.Rules = clusterPolicyRules
	if _, err := ensureClusterRoleExist(clusterKubeClient, clusterRole, opts.DryRun); err != nil {
		return err
	}

	// create a ClusterRoleBinding in cluster.
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	clusterRoleBinding.Name = clusterRole.Name
	clusterRoleBinding.Subjects = buildRoleBindingSubjects(serviceAccountObj.Name, serviceAccountObj.Namespace)
	clusterRoleBinding.RoleRef = buildClusterRoleReference(clusterRole.Name)
	if _, err := ensureClusterRoleBindingExist(clusterKubeClient, clusterRoleBinding, opts.DryRun); err != nil {
		return err
	}

	if opts.DryRun {
		return nil
	}

	var clusterSecret *corev1.Secret
	// It will take a short time to create service account secret for cluster.
	err = wait.Poll(1*time.Second, 30*time.Second, func() (done bool, err error) {
		serviceAccountObj, err = clusterKubeClient.CoreV1().ServiceAccounts(serviceAccountObj.Namespace).Get(context.TODO(), serviceAccountObj.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to retrieve service account(%s/%s) from cluster. err: %v", serviceAccountObj.Namespace, serviceAccountObj.Name, err)
			return false, err
		}
		clusterSecret, err = util.GetTargetSecret(clusterKubeClient, serviceAccountObj.Secrets, corev1.SecretTypeServiceAccountToken, opts.ClusterNamespace)
		if err != nil {
			return false, err
		}

		return true, nil
	})
	if err != nil {
		klog.Errorf("Failed to get service account secret from  cluster. error: %v", err)
		return err
	}

	// create secret in control plane
	secretNamespace := opts.ClusterNamespace
	secretName := opts.ClusterName
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: secretNamespace,
			Name:      secretName,
		},
		Data: map[string][]byte{
			caDataKey: clusterSecret.Data["ca.crt"], // TODO(RainbowMango): change ca bundle key to 'ca.crt'.
			tokenKey:  clusterSecret.Data[tokenKey],
		},
	}
	secret, err = util.CreateSecret(controlPlaneKubeClient, secret)
	if err != nil {
		klog.Errorf("Failed to create secret in control plane. error: %v", err)
		return err
	}

	clusterObj := &clusterv1alpha1.Cluster{}
	clusterObj.Name = opts.ClusterName
	clusterObj.Spec.SyncMode = clusterv1alpha1.Push
	clusterObj.Spec.APIEndpoint = clusterConfig.Host
	clusterObj.Spec.SecretRef = &clusterv1alpha1.LocalSecretReference{
		Namespace: secretNamespace,
		Name:      secretName,
	}

	if clusterConfig.TLSClientConfig.Insecure {
		clusterObj.Spec.InsecureSkipTLSVerification = true
	}

	if clusterConfig.Proxy != nil {
		url, err := clusterConfig.Proxy(nil)
		if err != nil {
			klog.Errorf("clusterConfig.Proxy error, %v", err)
			return err
		}
		clusterObj.Spec.ProxyURL = url.String()
	}

	cluster, err := CreateClusterObject(controlPlaneKarmadaClient, clusterObj, false)
	if err != nil {
		klog.Errorf("failed to create cluster object. cluster name: %s, error: %v", opts.ClusterName, err)
		return err
	}

	patchSecretBody := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterResourceKind),
			},
		},
	}

	err = util.PatchSecret(controlPlaneKubeClient, secretNamespace, secretName, types.MergePatchType, patchSecretBody)
	if err != nil {
		klog.Errorf("failed to patch secret %s/%s, error: %v", secret.Namespace, secret.Name, err)
		return err
	}

	return nil
}

// ensureNamespaceExist makes sure that the specific namespace exist in cluster.
// If namespace not exit, just create it.
func ensureNamespaceExist(client kubeclient.Interface, namespace string, dryRun bool) (*corev1.Namespace, error) {
	namespaceObj := &corev1.Namespace{}
	namespaceObj.ObjectMeta.Name = namespace

	if dryRun {
		return namespaceObj, nil
	}

	exist, err := util.IsNamespaceExist(client, namespace)
	if err != nil {
		klog.Errorf("failed to check if namespace exist. namespace: %s, error: %v", namespace, err)
		return nil, err
	}
	if exist {
		klog.V(1).Infof("ensure namespace succeed as already exist. namespace: %s", namespace)
		return namespaceObj, nil
	}

	createdObj, err := util.CreateNamespace(client, namespaceObj)
	if err != nil {
		klog.Errorf("ensure namespace failed due to create failed. namespace: %s, error: %v", namespace, err)
		return nil, err
	}

	return createdObj, nil
}

// ensureServiceAccountExist makes sure that the specific service account exist in cluster.
// If service account not exit, just create it.
func ensureServiceAccountExist(client kubeclient.Interface, serviceAccountObj *corev1.ServiceAccount, dryRun bool) (*corev1.ServiceAccount, error) {
	if dryRun {
		return serviceAccountObj, nil
	}

	exist, err := util.IsServiceAccountExist(client, serviceAccountObj.Namespace, serviceAccountObj.Name)
	if err != nil {
		klog.Errorf("failed to check if service account exist. service account: %s/%s, error: %v", serviceAccountObj.Namespace, serviceAccountObj.Name, err)
		return nil, err
	}
	if exist {
		klog.V(1).Infof("ensure service account succeed as already exist. service account: %s/%s", serviceAccountObj.Namespace, serviceAccountObj.Name)
		return serviceAccountObj, nil
	}

	createdObj, err := util.CreateServiceAccount(client, serviceAccountObj)
	if err != nil {
		klog.Errorf("ensure service account failed due to create failed. service account: %s/%s, error: %v", serviceAccountObj.Namespace, serviceAccountObj.Name, err)
		return nil, err
	}

	return createdObj, nil
}

// ensureClusterRoleExist makes sure that the specific cluster role exist in cluster.
// If cluster role not exit, just create it.
func ensureClusterRoleExist(client kubeclient.Interface, clusterRole *rbacv1.ClusterRole, dryRun bool) (*rbacv1.ClusterRole, error) {
	if dryRun {
		return clusterRole, nil
	}

	exist, err := util.IsClusterRoleExist(client, clusterRole.Name)
	if err != nil {
		klog.Errorf("failed to check if ClusterRole exist. ClusterRole: %s, error: %v", clusterRole.Name, err)
		return nil, err
	}
	if exist {
		klog.V(1).Infof("ensure ClusterRole succeed as already exist. ClusterRole: %s", clusterRole.Name)
		return clusterRole, nil
	}

	createdObj, err := util.CreateClusterRole(client, clusterRole)
	if err != nil {
		klog.Errorf("ensure ClusterRole failed due to create failed. ClusterRole: %s, error: %v", clusterRole.Name, err)
		return nil, err
	}

	return createdObj, nil
}

// ensureClusterRoleBindingExist makes sure that the specific ClusterRoleBinding exist in cluster.
// If ClusterRoleBinding not exit, just create it.
func ensureClusterRoleBindingExist(client kubeclient.Interface, clusterRoleBinding *rbacv1.ClusterRoleBinding, dryRun bool) (*rbacv1.ClusterRoleBinding, error) {
	if dryRun {
		return clusterRoleBinding, nil
	}

	exist, err := util.IsClusterRoleBindingExist(client, clusterRoleBinding.Name)
	if err != nil {
		klog.Errorf("failed to check if ClusterRole exist. ClusterRole: %s, error: %v", clusterRoleBinding.Name, err)
		return nil, err
	}
	if exist {
		klog.V(1).Infof("ensure ClusterRole succeed as already exist. ClusterRole: %s", clusterRoleBinding.Name)
		return clusterRoleBinding, nil
	}

	createdObj, err := util.CreateClusterRoleBinding(client, clusterRoleBinding)
	if err != nil {
		klog.Errorf("ensure ClusterRole failed due to create failed. ClusterRole: %s, error: %v", clusterRoleBinding.Name, err)
		return nil, err
	}

	return createdObj, nil
}

// CreateClusterObject create cluster object in karmada control plane
func CreateClusterObject(controlPlaneClient *karmadaclientset.Clientset, clusterObj *clusterv1alpha1.Cluster, errorOnExisting bool) (*clusterv1alpha1.Cluster, error) {
	cluster, exist, err := GetCluster(controlPlaneClient, clusterObj.Name)
	if err != nil {
		klog.Errorf("failed to create cluster(%s), error: %v", clusterObj.Name, err)
		return nil, err
	}

	if exist {
		if errorOnExisting {
			klog.Errorf("failed to create cluster(%s) as it's already exist.", clusterObj.Name)
			return cluster, fmt.Errorf("cluster already exist")
		}
		klog.V(1).Infof("create cluster(%s) succeed as already exist.", clusterObj.Name)
		return cluster, nil
	}

	if cluster, err = CreateCluster(controlPlaneClient, clusterObj); err != nil {
		klog.Warningf("failed to create cluster(%s). error: %v", clusterObj.Name, err)
		return nil, err
	}

	return cluster, nil
}

// GetCluster tells if a cluster already joined to control plane.
func GetCluster(client karmadaclientset.Interface, name string) (*clusterv1alpha1.Cluster, bool, error) {
	cluster, err := client.ClusterV1alpha1().Clusters().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		klog.Warningf("failed to retrieve cluster(%s). error: %v", cluster.Name, err)
		return nil, false, err
	}

	return cluster, true, nil
}

// CreateCluster creates a new cluster object in control plane.
func CreateCluster(controlPlaneClient karmadaclientset.Interface, cluster *clusterv1alpha1.Cluster) (*clusterv1alpha1.Cluster, error) {
	cluster, err := controlPlaneClient.ClusterV1alpha1().Clusters().Create(context.TODO(), cluster, metav1.CreateOptions{})
	if err != nil {
		klog.Warningf("failed to create cluster(%s). error: %v", cluster.Name, err)
		return cluster, err
	}

	return cluster, nil
}

// buildRoleBindingSubjects will generate a subject as per service account.
// The subject used by RoleBinding or ClusterRoleBinding.
func buildRoleBindingSubjects(serviceAccountName, serviceAccountNamespace string) []rbacv1.Subject {
	return []rbacv1.Subject{
		{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: serviceAccountNamespace,
		},
	}
}

// buildClusterRoleReference will generate a ClusterRole reference.
func buildClusterRoleReference(roleName string) rbacv1.RoleRef {
	return rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     roleName,
	}
}
