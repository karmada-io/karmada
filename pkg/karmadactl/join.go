package karmadactl

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/apis/cluster/validation"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

var (
	joinShort = `Register a cluster to control plane`
	joinLong  = `Join registers a cluster to control plane.`
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

	clusterSecret, impersonatorSecret, err := obtainCredentialsFromMemberCluster(clusterKubeClient, opts.ClusterNamespace, opts.ClusterName, opts.DryRun)
	if err != nil {
		return err
	}

	if opts.DryRun {
		return nil
	}

	err = registerClusterInControllerPlane(opts, controlPlaneRestConfig, clusterConfig, controlPlaneKubeClient, clusterSecret, impersonatorSecret)
	if err != nil {
		return err
	}

	fmt.Printf("cluster(%s) is joined successfully\n", opts.ClusterName)
	return nil
}

func obtainCredentialsFromMemberCluster(clusterKubeClient kubeclient.Interface, clusterNamespace, clusterName string, dryRun bool) (*corev1.Secret, *corev1.Secret, error) {
	var err error

	// ensure namespace where the karmada control plane credential be stored exists in cluster.
	if _, err = util.EnsureNamespaceExist(clusterKubeClient, clusterNamespace, dryRun); err != nil {
		return nil, nil, err
	}

	// create a ServiceAccount in cluster.
	serviceAccountObj := &corev1.ServiceAccount{}
	serviceAccountObj.Namespace = clusterNamespace
	serviceAccountObj.Name = names.GenerateServiceAccountName(clusterName)
	if serviceAccountObj, err = util.EnsureServiceAccountExist(clusterKubeClient, serviceAccountObj, dryRun); err != nil {
		return nil, nil, err
	}

	// create a ServiceAccount for impersonation in cluster.
	impersonationSA := &corev1.ServiceAccount{}
	impersonationSA.Namespace = clusterNamespace
	impersonationSA.Name = names.GenerateServiceAccountName("impersonator")
	if impersonationSA, err = util.EnsureServiceAccountExist(clusterKubeClient, impersonationSA, dryRun); err != nil {
		return nil, nil, err
	}

	// create a ClusterRole in cluster.
	clusterRole := &rbacv1.ClusterRole{}
	clusterRole.Name = names.GenerateRoleName(serviceAccountObj.Name)
	clusterRole.Rules = clusterPolicyRules
	if _, err = ensureClusterRoleExist(clusterKubeClient, clusterRole, dryRun); err != nil {
		return nil, nil, err
	}

	// create a ClusterRoleBinding in cluster.
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{}
	clusterRoleBinding.Name = clusterRole.Name
	clusterRoleBinding.Subjects = buildRoleBindingSubjects(serviceAccountObj.Name, serviceAccountObj.Namespace)
	clusterRoleBinding.RoleRef = buildClusterRoleReference(clusterRole.Name)
	if _, err = ensureClusterRoleBindingExist(clusterKubeClient, clusterRoleBinding, dryRun); err != nil {
		return nil, nil, err
	}

	if dryRun {
		return nil, nil, nil
	}

	clusterSecret, err := util.WaitForServiceAccountSecretCreation(clusterKubeClient, serviceAccountObj)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get serviceAccount secret from cluster(%s), error: %v", clusterName, err)
	}

	impersonatorSecret, err := util.WaitForServiceAccountSecretCreation(clusterKubeClient, impersonationSA)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get serviceAccount secret for impersonation from cluster(%s), error: %v", clusterName, err)
	}

	return clusterSecret, impersonatorSecret, nil
}

func registerClusterInControllerPlane(opts CommandJoinOption, controlPlaneRestConfig, clusterConfig *rest.Config, controlPlaneKubeClient kubeclient.Interface, clusterSecret, clusterImpersonatorSecret *corev1.Secret) error {
	// create secret in control plane
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: opts.ClusterNamespace,
			Name:      opts.ClusterName,
		},
		Data: map[string][]byte{
			clusterv1alpha1.SecretCADataKey: clusterSecret.Data["ca.crt"],
			clusterv1alpha1.SecretTokenKey:  clusterSecret.Data[clusterv1alpha1.SecretTokenKey],
		},
	}

	secret, err := util.CreateSecret(controlPlaneKubeClient, secret)
	if err != nil {
		return fmt.Errorf("failed to create secret in control plane. error: %v", err)
	}

	// create secret to store impersonation info in control plane
	impersonatorSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: opts.ClusterNamespace,
			Name:      names.GenerateImpersonationSecretName(opts.ClusterName),
		},
		Data: map[string][]byte{
			clusterv1alpha1.SecretTokenKey: clusterImpersonatorSecret.Data[clusterv1alpha1.SecretTokenKey],
		},
	}

	impersonatorSecret, err = util.CreateSecret(controlPlaneKubeClient, impersonatorSecret)
	if err != nil {
		return fmt.Errorf("failed to create impersonator secret in control plane. error: %v", err)
	}

	cluster, err := generateClusterInControllerPlane(controlPlaneRestConfig, clusterConfig, opts, *secret, *impersonatorSecret)
	if err != nil {
		return err
	}

	// add OwnerReference for secrets.
	patchSecretBody := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cluster, clusterResourceKind),
			},
		},
	}

	err = util.PatchSecret(controlPlaneKubeClient, secret.Namespace, secret.Name, types.MergePatchType, patchSecretBody)
	if err != nil {
		return fmt.Errorf("failed to patch secret %s/%s, error: %v", secret.Namespace, secret.Name, err)
	}

	err = util.PatchSecret(controlPlaneKubeClient, impersonatorSecret.Namespace, impersonatorSecret.Name, types.MergePatchType, patchSecretBody)
	if err != nil {
		return fmt.Errorf("failed to patch impersonator secret %s/%s, error: %v", impersonatorSecret.Namespace, impersonatorSecret.Name, err)
	}

	return nil
}

func generateClusterInControllerPlane(controlPlaneConfig, clusterConfig *rest.Config, opts CommandJoinOption, secret, impersonatorSecret corev1.Secret) (*clusterv1alpha1.Cluster, error) {
	clusterObj := &clusterv1alpha1.Cluster{}
	clusterObj.Name = opts.ClusterName
	clusterObj.Spec.SyncMode = clusterv1alpha1.Push
	clusterObj.Spec.APIEndpoint = clusterConfig.Host
	clusterObj.Spec.SecretRef = &clusterv1alpha1.LocalSecretReference{
		Namespace: secret.Namespace,
		Name:      secret.Name,
	}
	clusterObj.Spec.ImpersonatorSecretRef = &clusterv1alpha1.LocalSecretReference{
		Namespace: impersonatorSecret.Namespace,
		Name:      impersonatorSecret.Name,
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

	if clusterConfig.TLSClientConfig.Insecure {
		clusterObj.Spec.InsecureSkipTLSVerification = true
	}

	if clusterConfig.Proxy != nil {
		url, err := clusterConfig.Proxy(nil)
		if err != nil {
			return nil, fmt.Errorf("clusterConfig.Proxy error, %v", err)
		}
		clusterObj.Spec.ProxyURL = url.String()
	}

	controlPlaneKarmadaClient := karmadaclientset.NewForConfigOrDie(controlPlaneConfig)
	cluster, err := util.CreateClusterObject(controlPlaneKarmadaClient, clusterObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster(%s) object. error: %v", opts.ClusterName, err)
	}

	return cluster, nil
}

// ensureClusterRoleExist makes sure that the specific cluster role exist in cluster.
// If cluster role not exit, just create it.
func ensureClusterRoleExist(client kubeclient.Interface, clusterRole *rbacv1.ClusterRole, dryRun bool) (*rbacv1.ClusterRole, error) {
	if dryRun {
		return clusterRole, nil
	}

	exist, err := util.IsClusterRoleExist(client, clusterRole.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check if ClusterRole exist. ClusterRole: %s, error: %v", clusterRole.Name, err)
	}
	if exist {
		klog.V(1).Infof("ensure ClusterRole succeed as already exist. ClusterRole: %s", clusterRole.Name)
		return clusterRole, nil
	}

	createdObj, err := util.CreateClusterRole(client, clusterRole)
	if err != nil {
		return nil, fmt.Errorf("ensure ClusterRole failed due to create failed. ClusterRole: %s, error: %v", clusterRole.Name, err)
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
		return nil, fmt.Errorf("failed to check if ClusterRole exist. ClusterRole: %s, error: %v", clusterRoleBinding.Name, err)
	}
	if exist {
		klog.V(1).Infof("ensure ClusterRole succeed as already exist. ClusterRole: %s", clusterRoleBinding.Name)
		return clusterRoleBinding, nil
	}

	createdObj, err := util.CreateClusterRoleBinding(client, clusterRoleBinding)
	if err != nil {
		return nil, fmt.Errorf("ensure ClusterRole failed due to create failed. ClusterRole: %s, error: %v", clusterRoleBinding.Name, err)
	}

	return createdObj, nil
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
