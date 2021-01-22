package karmadactl

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	memberclusterapi "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/propagationstrategy/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	joinLong = `Join registers a member cluster to control plane.`

	joinExample = `
karmadactl join CLUSTER_NAME --member-cluster-kubeconfig=<KUBECONFIG>
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

var resourceKind = v1alpha1.SchemeGroupVersion.WithKind("Cluster")

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
		Use:     "join CLUSTER_NAME --member-cluster-kubeconfig=<KUBECONFIG>",
		Short:   "Register a member cluster to control plane",
		Long:    joinLong,
		Example: joinExample,
		Run: func(cmd *cobra.Command, args []string) {
			err := opts.Complete(args)
			if err != nil {
				klog.Errorf("Error: %v", err)
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

	// MemberClusterName is the member cluster's name that we are going to join with.
	MemberClusterName string

	// MemberClusterContext is the member cluster's context that we are going to join with.
	MemberClusterContext string

	// MemberClusterKubeConfig is the member cluster's kubeconfig path.
	MemberClusterKubeConfig string
}

// Complete ensures that options are valid and marshals them if necessary.
func (j *CommandJoinOption) Complete(args []string) error {
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
func (j *CommandJoinOption) AddFlags(flags *pflag.FlagSet) {
	j.GlobalCommandOptions.AddFlags(flags)

	flags.StringVar(&j.MemberClusterContext, "member-cluster-context", "",
		"Context name of member cluster in kubeconfig. Only works when there are multiple contexts in the kubeconfig.")
	flags.StringVar(&j.MemberClusterKubeConfig, "member-cluster-kubeconfig", "",
		"Path of the member cluster's kubeconfig.")
}

// RunJoin is the implementation of the 'join' command.
// TODO(RainbowMango): consider to remove the 'KarmadaConfig'.
func RunJoin(cmdOut io.Writer, karmadaConfig KarmadaConfig, opts CommandJoinOption) error {
	klog.V(1).Infof("joining member cluster. member cluster name: %s", opts.MemberClusterName)
	klog.V(1).Infof("joining member cluster. cluster namespace: %s", opts.ClusterNamespace)

	// Get control plane kube-apiserver client
	controlPlaneRestConfig, err := karmadaConfig.GetRestConfig(opts.ClusterContext, opts.KubeConfig)
	if err != nil {
		klog.Errorf("failed to get control plane rest config. context: %s, kube-config: %s, error: %v",
			opts.ClusterContext, opts.KubeConfig, err)
		return err
	}

	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneRestConfig)
	controlPlaneKubeClient := kubeclient.NewForConfigOrDie(controlPlaneRestConfig)

	// Get member cluster config
	memberClusterConfig, err := karmadaConfig.GetRestConfig(opts.MemberClusterContext, opts.MemberClusterKubeConfig)
	if err != nil {
		klog.V(1).Infof("failed to get joining member cluster config. error: %v", err)
		return err
	}
	memberClusterKubeClient := kubeclient.NewForConfigOrDie(memberClusterConfig)

	klog.V(1).Infof("joining member cluster config. endpoint: %s", memberClusterConfig.Host)

	// ensure namespace where the member cluster object be stored exists in control plane.
	if _, err := ensureNamespaceExist(controlPlaneKubeClient, opts.ClusterNamespace, opts.DryRun); err != nil {
		return err
	}
	// ensure namespace where the karmada control plane credential be stored exists in member cluster.
	if _, err := ensureNamespaceExist(memberClusterKubeClient, opts.ClusterNamespace, opts.DryRun); err != nil {
		return err
	}

	// create a ServiceAccount in member cluster.
	serviceAccountObj := &corev1.ServiceAccount{}
	serviceAccountObj.Namespace = opts.ClusterNamespace
	serviceAccountObj.Name = names.GenerateServiceAccountName(opts.MemberClusterName)
	if serviceAccountObj, err = ensureServiceAccountExist(memberClusterKubeClient, serviceAccountObj, opts.DryRun); err != nil {
		return err
	}

	// create a ClusterRole in member cluster.
	clusterRole := &rbacv1.ClusterRole{}
	clusterRole.Name = names.GenerateRoleName(serviceAccountObj.Name)
	clusterRole.Rules = clusterPolicyRules
	if _, err := ensureClusterRoleExist(memberClusterKubeClient, clusterRole, opts.DryRun); err != nil {
		return err
	}

	// create a ClusterRoleBinding in member cluster.
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	clusterRoleBinding.Name = clusterRole.Name
	clusterRoleBinding.Subjects = buildRoleBindingSubjects(serviceAccountObj.Name, serviceAccountObj.Namespace)
	clusterRoleBinding.RoleRef = buildClusterRoleReference(clusterRole.Name)
	if _, err := ensureClusterRoleBindingExist(memberClusterKubeClient, clusterRoleBinding, opts.DryRun); err != nil {
		return err
	}

	var memberClusterSecret *corev1.Secret
	// It will take a short time to create service account secret for member cluster.
	err = wait.Poll(1*time.Second, 30*time.Second, func() (done bool, err error) {
		serviceAccountObj, err = memberClusterKubeClient.CoreV1().ServiceAccounts(serviceAccountObj.Namespace).Get(context.TODO(), serviceAccountObj.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to retrieve service account(%s/%s) from member cluster. err: %v", serviceAccountObj.Namespace, serviceAccountObj.Name, err)
			return false, err
		}
		memberClusterSecret, err = util.GetTargetSecret(memberClusterKubeClient, serviceAccountObj.Secrets, corev1.SecretTypeServiceAccountToken, opts.ClusterNamespace)
		if err != nil {
			return false, err
		}

		return true, nil
	})
	if err != nil {
		klog.Errorf("Failed to get service account secret from member cluster. error: %v", err)
		return err
	}

	// create secret in control plane
	secretNamespace := opts.ClusterNamespace
	secretName := opts.MemberClusterName
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: secretNamespace,
			Name:      secretName,
		},
		Data: map[string][]byte{
			caDataKey: memberClusterSecret.Data["ca.crt"], // TODO(RainbowMango): change ca bundle key to 'ca.crt'.
			tokenKey:  memberClusterSecret.Data[tokenKey],
		},
	}
	secret, err = util.CreateSecret(controlPlaneKubeClient, secret)
	if err != nil {
		klog.Errorf("Failed to create secret in control plane. error: %v", err)
		return err
	}

	if opts.DryRun {
		return nil
	}

	memberClusterObj := &memberclusterapi.Cluster{}
	memberClusterObj.Name = opts.MemberClusterName
	memberClusterObj.Spec.APIEndpoint = memberClusterConfig.Host
	memberClusterObj.Spec.SecretRef = &memberclusterapi.LocalSecretReference{
		Namespace: secretNamespace,
		Name:      secretName,
	}
	memberCluster, err := createMemberClusterObject(controlPlaneKarmadaClient, memberClusterObj, false)
	if err != nil {
		klog.Errorf("failed to create member cluster object. cluster name: %s, error: %v", opts.MemberClusterName, err)
		return err
	}

	patchSecretBody := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(memberCluster, resourceKind),
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

func createMemberClusterObject(controlPlaneClient *karmadaclientset.Clientset, memberClusterObj *memberclusterapi.Cluster, errorOnExisting bool) (*memberclusterapi.Cluster, error) {
	memberCluster, exist, err := GetMemberCluster(controlPlaneClient, memberClusterObj.Namespace, memberClusterObj.Name)
	if err != nil {
		klog.Errorf("failed to create member cluster object. member cluster: %s/%s, error: %v", memberClusterObj.Namespace, memberClusterObj.Name, err)
		return nil, err
	}

	if exist {
		if errorOnExisting {
			klog.Errorf("failed to create member cluster object. member cluster: %s/%s, error: %v", memberClusterObj.Namespace, memberClusterObj.Name, err)
			return memberCluster, err
		}

		klog.V(1).Infof("create member cluster succeed as already exist. member cluster: %s/%s", memberClusterObj.Namespace, memberClusterObj.Name)
		return memberCluster, nil
	}

	if memberCluster, err = CreateMemberCluster(controlPlaneClient, memberClusterObj); err != nil {
		klog.Warningf("failed to create member cluster. member cluster: %s/%s, error: %v", memberClusterObj.Namespace, memberClusterObj.Name, err)
		return nil, err
	}

	return memberCluster, nil
}

// GetMemberCluster tells if a member cluster (namespace/name) already joined to control plane.
func GetMemberCluster(client karmadaclientset.Interface, namespace string, name string) (*memberclusterapi.Cluster, bool, error) {
	memberCluster, err := client.MemberclusterV1alpha1().MemberClusters().Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, false, nil
		}

		klog.Warningf("failed to retrieve member cluster. member cluster: %s/%s, error: %v", namespace, name, err)
		return nil, false, err
	}

	return memberCluster, true, nil
}

// CreateMemberCluster creates a new member cluster object in control plane.
func CreateMemberCluster(controlPlaneClient karmadaclientset.Interface, cluster *memberclusterapi.Cluster) (*memberclusterapi.Cluster, error) {
	memberCluster, err := controlPlaneClient.MemberclusterV1alpha1().MemberClusters().Create(context.TODO(), cluster, metav1.CreateOptions{})
	if err != nil {
		klog.Warningf("failed to create member cluster. member cluster: %s/%s, error: %v", cluster.Namespace, cluster.Name, err)
		return memberCluster, err
	}

	return memberCluster, nil
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
