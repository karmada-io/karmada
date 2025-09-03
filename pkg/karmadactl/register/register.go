/*
Copyright 2022 The Karmada Authors.

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

package register

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	bootstrap "k8s.io/cluster-bootstrap/token/jws"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/apis/cluster/validation"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	tokenutil "github.com/karmada-io/karmada/pkg/karmadactl/util/bootstraptoken"
	karmadautil "github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/lifted/pubkeypin"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/version"
)

var (
	registerLong = templates.LongDesc(`
		Register a cluster to Karmada control plane with Pull mode.`)

	registerExample = templates.Examples(`
		# Register cluster into karmada control plane with Pull mode.
		# If '--cluster-name' isn't specified, the cluster of current-context will be used by default.
		%[1]s register [karmada-apiserver-endpoint] --cluster-name=<CLUSTER_NAME> --token=<TOKEN>  --discovery-token-ca-cert-hash=<CA-CERT-HASH>
		
		# UnsafeSkipCAVerification allows token-based discovery without CA verification via CACertHashes. This can weaken
		# the security of register command since other clusters can impersonate the control-plane.
		%[1]s register [karmada-apiserver-endpoint] --token=<TOKEN>  --discovery-token-unsafe-skip-ca-verification=true
		`)
)

const (
	// KarmadaDir is the directory Karmada owns for storing various configuration files
	KarmadaDir = "/etc/karmada"
	// CACertPath defines default location of CA certificate on Linux
	CACertPath = "/etc/karmada/pki/ca.crt"
	// ClusterPermissionPrefix defines the common name of karmada agent certificate
	ClusterPermissionPrefix = "system:karmada:agent:"
	// ClusterPermissionGroups defines the organization of karmada agent certificate
	ClusterPermissionGroups = "system:karmada:agents"
	// AgentRBACGenerator defines the common name of karmada agent rbac generator certificate
	AgentRBACGenerator = "system:karmada:agent:rbac-generator"
	// KarmadaAgentKubeConfigFileName defines the file name for the kubeconfig that the karmada-agent will use to do
	// the TLS bootstrap to get itself an unique credential
	KarmadaAgentKubeConfigFileName = "karmada-agent.conf"
	// KarmadaKubeconfigName is the name of karmada kubeconfig
	KarmadaKubeconfigName = "karmada-kubeconfig"
	// KarmadaAgentServiceAccountName is the name of karmada-agent serviceaccount
	KarmadaAgentServiceAccountName = "karmada-agent-sa"
	// SignerName defines the signer name for csr, 'kubernetes.io/kube-apiserver-client' can sign the csr with `O=system:karmada:agents,CN=system:karmada:agent:` automatically if agentcsrapproving controller is enabled.
	SignerName = "kubernetes.io/kube-apiserver-client"
	// BootstrapUserName defines bootstrap user name
	BootstrapUserName = "token-bootstrap-client"
	// DefaultClusterName defines the default cluster name
	DefaultClusterName = "karmada-apiserver"
	// TokenUserName defines token user
	TokenUserName = "tls-bootstrap-token-user"
	// DefaultDiscoveryTimeout specifies the default discovery timeout for register command
	DefaultDiscoveryTimeout = 5 * time.Minute
	// DiscoveryRetryInterval specifies how long register command should wait before retrying to connect to the control-plane when doing discovery
	DiscoveryRetryInterval = 5 * time.Second
	// DefaultCertExpirationSeconds define the expiration time of certificate
	DefaultCertExpirationSeconds int32 = 86400 * 365
)

var karmadaAgentLabels = map[string]string{"app": names.KarmadaAgentComponentName}

// BootstrapTokenDiscovery is used to set the options for bootstrap token based discovery
type BootstrapTokenDiscovery struct {
	// Token is a token used to validate cluster information
	// fetched from the control-plane.
	Token string

	// APIServerEndpoint is an IP or domain name to the API server from which info will be fetched.
	APIServerEndpoint string

	// CACertHashes specifies a set of public key pins to verify
	// when token-based discovery is used. The root CA found during discovery
	// must match one of these values. Specifying an empty set disables root CA
	// pinning, which can be unsafe. Each hash is specified as "<type>:<value>",
	// where the only currently supported type is "sha256". This is a hex-encoded
	// SHA-256 hash of the Subject Public Key Info (SPKI) object in DER-encoded
	// ASN.1. These hashes can be calculated using, for example, OpenSSL.
	CACertHashes []string

	// UnsafeSkipCAVerification allows token-based discovery
	// without CA verification via CACertHashes. This can weaken
	// the security of register command since other clusters can impersonate the control-plane.
	UnsafeSkipCAVerification bool
}

// NewCmdRegister defines the `register` command that registers a cluster.
func NewCmdRegister(parentCommand string) *cobra.Command {
	opts := CommandRegisterOption{
		BootstrapToken: &BootstrapTokenDiscovery{},
	}

	cmd := &cobra.Command{
		Use:                   "register [karmada-apiserver-endpoint]",
		Short:                 "Register a cluster to Karmada control plane with Pull mode",
		Long:                  registerLong,
		Example:               fmt.Sprintf(registerExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if err := opts.Complete(args); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Run(parentCommand); err != nil {
				return err
			}
			return nil
		},
		Annotations: map[string]string{
			cmdutil.TagCommandGroup: cmdutil.GroupClusterRegistration,
		},

		// We accept the control-plane location as a required positional argument karmada apiserver endpoint
		Args: cobra.ExactArgs(1),
	}
	flags := cmd.Flags()

	releaseVer, err := version.ParseGitVersion(version.Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", version.Get().String())
		releaseVer = &version.ReleaseVersion{} // initialize to avoid panic
	}

	flags.StringVar(&opts.KubeConfig, "kubeconfig", "", "Path to the kubeconfig file of member cluster.")
	flags.StringVar(&opts.Context, "context", "", "Name of the cluster context in kubeconfig file.")
	flags.StringVarP(&opts.Namespace, "namespace", "n", "karmada-system", "Namespace the karmada-agent component deployed.")
	flags.StringVar(&opts.ClusterName, "cluster-name", "", "The name of member cluster in the control plane, if not specified, the cluster of current-context is used by default.")
	flags.StringVar(&opts.ClusterNamespace, "cluster-namespace", options.DefaultKarmadaClusterNamespace, "Namespace in the control plane where member cluster secrets are stored.")
	flags.StringVar(&opts.ClusterProvider, "cluster-provider", "", "Provider of the joining cluster. The Karmada scheduler can use this information to spread workloads across providers for higher availability.")
	flags.StringVar(&opts.ClusterRegion, "cluster-region", "", "The region of the joining cluster. The Karmada scheduler can use this information to spread workloads across regions for higher availability.")
	flags.StringSliceVar(&opts.ClusterZones, "cluster-zones", []string{}, "The zones of the joining cluster. The Karmada scheduler can use this information to spread workloads across zones for higher availability.")
	flags.BoolVar(&opts.EnableCertRotation, "enable-cert-rotation", false, "Enable means controller would rotate certificate for karmada-agent when the certificate is about to expire.")
	// nolint: errcheck
	flags.StringVar(&opts.BootstrapToken.Token, "token", "", "For token-based discovery, the token used to validate cluster information fetched from the API server.")
	flags.StringSliceVar(&opts.BootstrapToken.CACertHashes, "discovery-token-ca-cert-hash", []string{}, "For token-based discovery, validate that the root CA public key matches this hash (format: \"<type>:<value>\").")
	flags.BoolVar(&opts.BootstrapToken.UnsafeSkipCAVerification, "discovery-token-unsafe-skip-ca-verification", false, "For token-based discovery, allow joining without --discovery-token-ca-cert-hash pinning.")
	flags.DurationVar(&opts.Timeout, "discovery-timeout", DefaultDiscoveryTimeout, "The timeout to discovery karmada apiserver client.")
	flags.StringVar(&opts.KarmadaAgentImage, "karmada-agent-image", fmt.Sprintf("docker.io/karmada/karmada-agent:%s", releaseVer.ReleaseVersion()), "Karmada agent image.")
	flags.Int32Var(&opts.KarmadaAgentReplicas, "karmada-agent-replicas", 1, "Karmada agent replicas.")
	flags.Int32Var(&opts.CertExpirationSeconds, "cert-expiration-seconds", DefaultCertExpirationSeconds, "The expiration time of certificate.")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Run the command in dry-run mode, without making any server requests.")
	flags.StringVar(&opts.ProxyServerAddress, "proxy-server-address", "", "Address of the proxy server that is used to proxy to the cluster.")

	return cmd
}

// CommandRegisterOption holds all command options.
type CommandRegisterOption struct {
	// KubeConfig holds the KUBECONFIG file path.
	KubeConfig string

	// Context is the name of the cluster context in KUBECONFIG file.
	// Default value is the current-context.
	Context string

	// Namespace is the namespace that karmada-agent component deployed.
	Namespace string

	// ClusterNamespace holds the namespace name where the member cluster secrets are stored.
	ClusterNamespace string

	// ClusterName is the cluster's name that we are going to join with.
	ClusterName string

	// ClusterProvider is the cluster's provider.
	ClusterProvider string

	// ClusterRegion represents the region of the cluster locate in.
	ClusterRegion string

	// ClusterZones represents the zones of the cluster locate in.
	ClusterZones []string

	// EnableCertRotation indicates if enable certificate rotation for karmada-agent.
	EnableCertRotation bool

	// BootstrapToken is used to set the options for bootstrap token based discovery
	BootstrapToken *BootstrapTokenDiscovery

	// Timeout is the max discovery time
	Timeout time.Duration

	// CertExpirationSeconds define the expiration time of certificate
	CertExpirationSeconds int32

	// KarmadaSchedulerImage is the image of karmada agent.
	KarmadaAgentImage string

	// KarmadaAgentReplicas is the number of karmada agent.
	KarmadaAgentReplicas int32

	// DryRun tells if run the command in dry-run mode, without making any server requests.
	DryRun bool

	// ProxyServerAddress holds the proxy server address that is used to proxy to the cluster.
	ProxyServerAddress string

	memberClusterEndpoint string
	memberClusterClient   *kubeclient.Clientset

	// rbacResources contains RBAC resources that grant the necessary permissions for pull mode cluster to access to Karmada control plane.
	rbacResources *RBACResources
}

// Complete ensures that options are valid and marshals them if necessary.
func (o *CommandRegisterOption) Complete(args []string) error {
	// Get karmada apiserver endpoint from the command args.
	o.BootstrapToken.APIServerEndpoint = args[0]

	restConfig, err := apiclient.RestConfig(o.Context, o.KubeConfig)
	if err != nil {
		return err
	}

	if len(o.ClusterName) == 0 {
		o.KubeConfig = apiclient.KubeConfigPath(o.KubeConfig)

		configBytes, err := os.ReadFile(o.KubeConfig)
		if err != nil {
			return fmt.Errorf("failed to read kubeconfig file %s, err: %w", o.KubeConfig, err)
		}

		config, err := clientcmd.Load(configBytes)
		if err != nil {
			return err
		}

		o.ClusterName = config.Contexts[config.CurrentContext].Cluster
	}

	o.rbacResources = GenerateRBACResources(o.ClusterName, o.ClusterNamespace)

	o.memberClusterEndpoint = restConfig.Host

	o.memberClusterClient, err = apiclient.NewClientSet(restConfig)
	if err != nil {
		return err
	}

	return nil
}

// Validate checks option and return a slice of found errs.
func (o *CommandRegisterOption) Validate() error {
	if errMsgs := validation.ValidateClusterName(o.ClusterName); len(errMsgs) != 0 {
		return fmt.Errorf("invalid cluster name(%s): %s", o.ClusterName, strings.Join(errMsgs, ";"))
	}

	if len(o.BootstrapToken.Token) == 0 {
		return fmt.Errorf("token is required")
	}

	if !o.BootstrapToken.UnsafeSkipCAVerification && len(o.BootstrapToken.CACertHashes) == 0 {
		return fmt.Errorf("need to verify CACertHashes, or set --discovery-token-unsafe-skip-ca-verification=true")
	}

	return nil
}

// Run is the implementation of the 'register' command.
func (o *CommandRegisterOption) Run(parentCommand string) error {
	klog.V(1).Infof("Registering cluster. cluster name: %s", o.ClusterName)
	klog.V(1).Infof("Registering cluster. cluster namespace: %s", o.ClusterNamespace)

	fmt.Println("[preflight] Running pre-flight checks")
	errlist := o.preflight()
	if len(errlist) > 0 {
		fmt.Println("error execution phase preflight: [preflight] Some fatal errors occurred:")
		for _, err := range errlist {
			fmt.Printf("\t[ERROR]: %s\n", err)
		}

		fmt.Printf("\n[preflight] Please check the above errors\n")
		return nil
	}
	fmt.Println("[preflight] All pre-flight checks were passed")

	if o.DryRun {
		return nil
	}

	// fetch the bootstrap client to connect to karmada apiserver temporarily
	fmt.Println("[karmada-agent-start] Waiting to perform the TLS Bootstrap")
	bootstrapClient, karmadaClusterInfo, err := o.discoveryBootstrapConfigAndClusterInfo(parentCommand)
	if err != nil {
		return err
	}

	var rbacClient *kubeclient.Clientset
	defer func() {
		if err != nil && rbacClient != nil {
			fmt.Println("karmadactl register failed and started deleting the created resources")
			err = o.rbacResources.Delete(rbacClient)
			if err != nil {
				klog.Warningf("Failed to delete rbac resources: %v", err)
			}
		}
	}()

	rbacClient, err = o.EnsureNecessaryResourcesExistInControlPlane(bootstrapClient, karmadaClusterInfo)
	if err != nil {
		return err
	}

	err = o.EnsureNecessaryResourcesExistInMemberCluster(bootstrapClient, karmadaClusterInfo)
	if err != nil {
		return err
	}

	fmt.Printf("\ncluster(%s) is joined successfully\n", o.ClusterName)

	return nil
}

// EnsureNecessaryResourcesExistInControlPlane ensures that all necessary resources are exist in Karmada control plane.
func (o *CommandRegisterOption) EnsureNecessaryResourcesExistInControlPlane(bootstrapClient *kubeclient.Clientset, karmadaClusterInfo *clientcmdapi.Cluster) (*kubeclient.Clientset, error) {
	csrName := "agent-rbac-generator-" + o.ClusterName + k8srand.String(5)
	rbacCfg, err := o.constructAgentRBACGeneratorConfig(bootstrapClient, karmadaClusterInfo, csrName)
	if err != nil {
		return nil, err
	}

	kubeClient, err := ToClientSet(rbacCfg)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = kubeClient.CertificatesV1().CertificateSigningRequests().Delete(context.Background(), csrName, metav1.DeleteOptions{})
		if err != nil {
			klog.Warningf("Failed to delete CertificateSigningRequests %s: %v", csrName, err)
		}
	}()

	fmt.Println("[karmada-agent-start] Waiting to check cluster exists")
	karmadaClient, err := ToKarmadaClient(rbacCfg)
	if err != nil {
		return kubeClient, err
	}
	_, exist, err := karmadautil.GetClusterWithKarmadaClient(karmadaClient, o.ClusterName)
	if err != nil {
		return kubeClient, err
	} else if exist {
		return kubeClient, fmt.Errorf("failed to register as cluster with name %s already exists", o.ClusterName)
	}

	fmt.Println("[karmada-agent-start] Assign the necessary RBAC permissions to the agent")
	err = o.ensureAgentRBACResourcesExistInControlPlane(kubeClient)
	if err != nil {
		return kubeClient, err
	}

	return kubeClient, nil
}

// EnsureNecessaryResourcesExistInMemberCluster ensures that all necessary resources are exist in the registering cluster.
func (o *CommandRegisterOption) EnsureNecessaryResourcesExistInMemberCluster(bootstrapClient *kubeclient.Clientset, karmadaClusterInfo *clientcmdapi.Cluster) error {
	// construct the final kubeconfig file used by karmada agent to connect to karmada apiserver
	fmt.Println("[karmada-agent-start] Waiting to construct karmada-agent kubeconfig")
	karmadaAgentCfg, err := o.constructKarmadaAgentConfig(bootstrapClient, karmadaClusterInfo)
	if err != nil {
		return err
	}

	// It's necessary to set the label of namespace to make sure that the namespace is created by Karmada.
	labels := map[string]string{
		karmadautil.KarmadaSystemLabel: karmadautil.KarmadaSystemLabelValue,
	}
	// ensure namespace where the karmada-agent resources be deployed exists in the member cluster
	if _, err = karmadautil.EnsureNamespaceExistWithLabels(o.memberClusterClient, o.Namespace, o.DryRun, labels); err != nil {
		return err
	}

	// create the necessary secret and RBAC in the member cluster
	fmt.Println("[karmada-agent-start] Waiting the necessary secret and RBAC")
	if err = o.createSecretAndRBACInMemberCluster(karmadaAgentCfg); err != nil {
		return err
	}

	// create karmada-agent Deployment in the member cluster
	fmt.Println("[karmada-agent-start] Waiting karmada-agent Deployment")
	KarmadaAgentDeployment := o.makeKarmadaAgentDeployment()
	if _, err = o.memberClusterClient.AppsV1().Deployments(o.Namespace).Create(context.TODO(), KarmadaAgentDeployment, metav1.CreateOptions{}); err != nil {
		return err
	}

	if err = cmdutil.WaitForDeploymentRollout(o.memberClusterClient, KarmadaAgentDeployment, o.Timeout); err != nil {
		return err
	}

	return nil
}

// preflight checks the deployment environment of the member cluster
func (o *CommandRegisterOption) preflight() []error {
	var errlist []error

	// check if relative resources already exist in member cluster
	_, err := o.memberClusterClient.CoreV1().Namespaces().Get(context.TODO(), o.Namespace, metav1.GetOptions{})
	if err == nil {
		_, err = o.memberClusterClient.CoreV1().Secrets(o.Namespace).Get(context.TODO(), KarmadaKubeconfigName, metav1.GetOptions{})
		if err == nil {
			errlist = append(errlist, fmt.Errorf("%s/%s Secret already exists", o.Namespace, KarmadaKubeconfigName))
		} else if !apierrors.IsNotFound(err) {
			errlist = append(errlist, err)
		}

		_, err = o.memberClusterClient.CoreV1().ServiceAccounts(o.Namespace).Get(context.TODO(), KarmadaAgentServiceAccountName, metav1.GetOptions{})
		if err == nil {
			errlist = append(errlist, fmt.Errorf("%s/%s ServiceAccount already exists", o.Namespace, KarmadaAgentServiceAccountName))
		} else if !apierrors.IsNotFound(err) {
			errlist = append(errlist, err)
		}

		_, err = o.memberClusterClient.AppsV1().Deployments(o.Namespace).Get(context.TODO(), names.KarmadaAgentComponentName, metav1.GetOptions{})
		if err == nil {
			errlist = append(errlist, fmt.Errorf("%s/%s Deployment already exists", o.Namespace, names.KarmadaAgentComponentName))
		} else if !apierrors.IsNotFound(err) {
			errlist = append(errlist, err)
		}
	}

	return errlist
}

// discoveryBootstrapConfigAndClusterInfo get bootstrap-config and cluster-info from control plane
func (o *CommandRegisterOption) discoveryBootstrapConfigAndClusterInfo(parentCommand string) (*kubeclient.Clientset, *clientcmdapi.Cluster, error) {
	config, err := retrieveValidatedConfigInfo(nil, o.BootstrapToken, o.Timeout, DiscoveryRetryInterval, parentCommand)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't validate the identity of the API Server: %w", err)
	}

	klog.V(1).Info("[discovery] Using provided TLSBootstrapToken as authentication credentials for the join process")
	clusterinfo := tokenutil.GetClusterFromKubeConfig(config, "")
	tlsBootstrapCfg := CreateWithToken(
		clusterinfo.Server,
		DefaultClusterName,
		TokenUserName,
		clusterinfo.CertificateAuthorityData,
		o.BootstrapToken.Token,
	)

	bootstrapClient, err := ToClientSet(tlsBootstrapCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create bootstrap client: %w", err)
	}

	return bootstrapClient, clusterinfo, nil
}

// ensureAgentRBACResourcesExistInControlPlane ensures that necessary RBAC resources for karmada-agent are exist in control plane.
func (o *CommandRegisterOption) ensureAgentRBACResourcesExistInControlPlane(client kubeclient.Interface) error {
	for i := range o.rbacResources.ClusterRoles {
		_, err := karmadautil.CreateClusterRole(client, o.rbacResources.ClusterRoles[i])
		if err != nil {
			return err
		}
	}
	for i := range o.rbacResources.ClusterRoleBindings {
		_, err := karmadautil.CreateClusterRoleBinding(client, o.rbacResources.ClusterRoleBindings[i])
		if err != nil {
			return err
		}
	}

	for i := range o.rbacResources.Roles {
		roleNamespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: o.rbacResources.Roles[i].GetNamespace(),
				Labels: map[string]string{
					karmadautil.KarmadaSystemLabel: karmadautil.KarmadaSystemLabelValue,
				},
			},
		}
		_, err := karmadautil.CreateNamespace(client, roleNamespace)
		if err != nil {
			return err
		}
		_, err = karmadautil.CreateRole(client, o.rbacResources.Roles[i])
		if err != nil {
			return err
		}
	}

	for i := range o.rbacResources.RoleBindings {
		_, err := karmadautil.CreateRoleBinding(client, o.rbacResources.RoleBindings[i])
		if err != nil {
			return err
		}
	}

	return nil
}

// RBACResources defines the list of rbac resources.
type RBACResources struct {
	ClusterRoles        []*rbacv1.ClusterRole
	ClusterRoleBindings []*rbacv1.ClusterRoleBinding
	Roles               []*rbacv1.Role
	RoleBindings        []*rbacv1.RoleBinding
}

// GenerateRBACResources generates rbac resources.
func GenerateRBACResources(clusterName, clusterNamespace string) *RBACResources {
	return &RBACResources{
		ClusterRoles:        []*rbacv1.ClusterRole{GenerateClusterRole(clusterName)},
		ClusterRoleBindings: []*rbacv1.ClusterRoleBinding{GenerateClusterRoleBinding(clusterName)},
		Roles:               []*rbacv1.Role{GenerateSecretAccessRole(clusterName, clusterNamespace), GenerateWorkAccessRole(clusterName)},
		RoleBindings:        []*rbacv1.RoleBinding{GenerateSecretAccessRoleBinding(clusterName, clusterNamespace), GenerateWorkAccessRoleBinding(clusterName)},
	}
}

// List return the list of rbac resources.
func (r *RBACResources) List() []Obj {
	var obj []Obj
	for i := range r.ClusterRoles {
		obj = append(obj, Obj{Kind: "ClusterRole", Name: r.ClusterRoles[i].GetName()})
	}
	for i := range r.ClusterRoleBindings {
		obj = append(obj, Obj{Kind: "ClusterRoleBinding", Name: r.ClusterRoleBindings[i].GetName()})
	}
	for i := range r.Roles {
		obj = append(obj, Obj{Kind: "Role", Name: r.Roles[i].GetName(), Namespace: r.Roles[i].GetNamespace()})
	}
	for i := range r.RoleBindings {
		obj = append(obj, Obj{Kind: "RoleBinding", Name: r.RoleBindings[i].GetName(), Namespace: r.RoleBindings[i].GetNamespace()})
	}
	return obj
}

// ToString returns a list of RBAC resources in string format.
func (r *RBACResources) ToString() string {
	var resources []string
	for i := range r.List() {
		resources = append(resources, r.List()[i].ToString())
	}
	return strings.Join(resources, "\n")
}

// Delete deletes RBAC resources.
func (r *RBACResources) Delete(client kubeclient.Interface) error {
	var err error
	for _, resource := range r.List() {
		switch resource.Kind {
		case "ClusterRole":
			err = karmadautil.DeleteClusterRole(client, resource.Name)
		case "ClusterRoleBinding":
			err = karmadautil.DeleteClusterRoleBinding(client, resource.Name)
		case "Role":
			err = karmadautil.DeleteRole(client, resource.Namespace, resource.Name)
		case "RoleBinding":
			err = karmadautil.DeleteRoleBinding(client, resource.Namespace, resource.Name)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Obj defines the struct which contains the information of kind, name and namespace.
type Obj struct{ Kind, Name, Namespace string }

// ToString returns a string that concatenates kind, name, and namespace using "/".
func (o *Obj) ToString() string {
	if o.Namespace == "" {
		return fmt.Sprintf("%s/%s", o.Kind, o.Name)
	}
	return fmt.Sprintf("%s/%s/%s", o.Kind, o.Namespace, o.Name)
}

// GenerateClusterRole generates the clusterRole that karmada-agent needed.
func GenerateClusterRole(clusterName string) *rbacv1.ClusterRole {
	clusterRoleName := fmt.Sprintf("system:karmada:%s:agent", clusterName)
	return &rbacv1.ClusterRole{
		TypeMeta: metav1.TypeMeta{
			APIVersion: rbacv1.SchemeGroupVersion.String(),
			Kind:       "ClusterRole",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{"cluster.karmada.io"},
				Resources:     []string{"clusters"},
				ResourceNames: []string{clusterName},
				Verbs:         []string{"get", "delete"},
			},
			{
				APIGroups: []string{"cluster.karmada.io"},
				Resources: []string{"clusters"},
				Verbs:     []string{"create", "list", "watch"},
			},
			{
				APIGroups:     []string{"cluster.karmada.io"},
				Resources:     []string{"clusters/status"},
				ResourceNames: []string{clusterName},
				Verbs:         []string{"update"},
			},
			{
				APIGroups: []string{"config.karmada.io"},
				Resources: []string{"resourceinterpreterwebhookconfigurations", "resourceinterpretercustomizations"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"namespaces"},
				Verbs:     []string{"get"},
			},
			{
				APIGroups: []string{"coordination.k8s.io"},
				Resources: []string{"leases"},
				Verbs:     []string{"get", "create", "update"},
			},
			{
				APIGroups: []string{"certificates.k8s.io"},
				Resources: []string{"certificatesigningrequests"},
				Verbs:     []string{"get", "create"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"events"},
				Verbs:     []string{"patch", "create", "update"},
			},
		},
	}
}

// GenerateClusterRoleBinding generates the clusterRoleBinding that karmada-agent needed.
func GenerateClusterRoleBinding(clusterName string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "ClusterRoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("system:karmada:%s:agent", clusterName)},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "User",
				Name:     generateAgentUserName(clusterName),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     fmt.Sprintf("system:karmada:%s:agent", clusterName),
		},
	}
}

// GenerateSecretAccessRole generates the secret-related Role that karmada-agent needed.
func GenerateSecretAccessRole(clusterName, clusterNamespace string) *rbacv1.Role {
	secretAccessRoleName := fmt.Sprintf("system:karmada:%s:agent-secret", clusterName)
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretAccessRoleName,
			Namespace: clusterNamespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"get", "patch"},
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{clusterName, clusterName + "-impersonator"},
			},
			{
				Verbs:     []string{"create"},
				APIGroups: []string{""},
				Resources: []string{"secrets"},
			},
		},
	}
}

// GenerateSecretAccessRoleBinding generates the secret-related RoleBinding that karmada-agent needed.
func GenerateSecretAccessRoleBinding(clusterName, clusterNamespace string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("system:karmada:%s:agent-secret", clusterName),
			Namespace: clusterNamespace,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "User",
				Name:     generateAgentUserName(clusterName),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     fmt.Sprintf("system:karmada:%s:agent-secret", clusterName),
		},
	}
}

// GenerateWorkAccessRole generates the work-related Role that karmada-agent needed.
func GenerateWorkAccessRole(clusterName string) *rbacv1.Role {
	workAccessRoleName := fmt.Sprintf("system:karmada:%s:agent-work", clusterName)
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      workAccessRoleName,
			Namespace: "karmada-es-" + clusterName,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{"get", "create", "list", "watch", "update", "delete"},
				APIGroups: []string{"work.karmada.io"},
				Resources: []string{"works"},
			},
			{
				Verbs:     []string{"patch", "update"},
				APIGroups: []string{"work.karmada.io"},
				Resources: []string{"works/status"},
			},
		},
	}
}

// GenerateWorkAccessRoleBinding generates the work-related RoleBinding that karmada-agent needed.
func GenerateWorkAccessRoleBinding(clusterName string) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "rbac.authorization.k8s.io/v1",
			Kind:       "RoleBinding",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("system:karmada:%s:agent-work", clusterName),
			Namespace: "karmada-es-" + clusterName,
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "User",
				Name:     generateAgentUserName(clusterName),
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     fmt.Sprintf("system:karmada:%s:agent-work", clusterName),
		},
	}
}

func (o *CommandRegisterOption) constructKubeConfig(bootstrapClient *kubeclient.Clientset, karmadaClusterInfo *clientcmdapi.Cluster, csrName, commonName string, organization []string) (*clientcmdapi.Config, error) {
	var cert []byte

	pk, csr, err := generateKeyAndCSR(commonName, organization)
	if err != nil {
		return nil, err
	}

	pkData, err := keyutil.MarshalPrivateKeyToPEM(pk)
	if err != nil {
		return nil, err
	}

	certificateSigningRequest := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: csrName,
			Labels: map[string]string{
				karmadautil.KarmadaSystemLabel: karmadautil.KarmadaSystemLabelValue,
			},
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request: pem.EncodeToMemory(&pem.Block{
				Type:  certutil.CertificateRequestBlockType,
				Bytes: csr,
			}),
			SignerName:        SignerName,
			ExpirationSeconds: &o.CertExpirationSeconds,
			Usages: []certificatesv1.KeyUsage{
				certificatesv1.UsageDigitalSignature,
				certificatesv1.UsageKeyEncipherment,
				certificatesv1.UsageClientAuth,
			},
		},
	}

	_, err = bootstrapClient.CertificatesV1().CertificateSigningRequests().Create(context.TODO(), certificateSigningRequest, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	klog.V(1).Infof("Waiting for the client certificate %s to be issued", csrName)
	err = wait.PollUntilContextTimeout(context.TODO(), 1*time.Second, o.Timeout, false, func(context.Context) (done bool, err error) {
		csrOK, err := bootstrapClient.CertificatesV1().CertificateSigningRequests().Get(context.TODO(), csrName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get the cluster csr %s. err: %v", csrName, err)
		}

		if csrOK.Status.Certificate != nil {
			klog.V(1).Infof("Signing certificate of csr %s successfully", csrName)
			cert = csrOK.Status.Certificate
			return true, nil
		}

		klog.V(1).Infof("Waiting for the client certificate of csr %s to be issued", csrName)
		klog.V(1).Infof("Approve the CSR %s manually by executing `kubectl certificate approve %s` on the control plane\nOr enable the agentcsrapproving controller of karmada-controller-manager to automatically approve agent CSR.", csrName, csrName)
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	// Using o.BootstrapToken.APIServerEndpoint instead of the endpoint in discovered cluster-info
	// because discivered endpoint can often be unreachable from member cluster
	karmadaServer := karmadaClusterInfo.Server
	if o.BootstrapToken.APIServerEndpoint != "" {
		karmadaServer = fmt.Sprintf("https://%s", o.BootstrapToken.APIServerEndpoint)
	}

	return CreateWithCert(
		karmadaServer,
		DefaultClusterName,
		o.ClusterName,
		karmadaClusterInfo.CertificateAuthorityData,
		cert,
		pkData,
	), nil
}

// constructKarmadaAgentConfig constructs the final kubeconfig used by karmada-agent
func (o *CommandRegisterOption) constructKarmadaAgentConfig(bootstrapClient *kubeclient.Clientset, karmadaClusterInfo *clientcmdapi.Cluster) (*clientcmdapi.Config, error) {
	csrName := o.ClusterName + "-" + k8srand.String(5)

	return o.constructKubeConfig(bootstrapClient, karmadaClusterInfo, csrName, generateAgentUserName(o.ClusterName), []string{ClusterPermissionGroups})
}

// constructKarmadaAgentConfig constructs the kubeconfig to generate rbac config for karmada-agent.
func (o *CommandRegisterOption) constructAgentRBACGeneratorConfig(bootstrapClient *kubeclient.Clientset, karmadaClusterInfo *clientcmdapi.Cluster, csrName string) (*clientcmdapi.Config, error) {
	return o.constructKubeConfig(bootstrapClient, karmadaClusterInfo, csrName, AgentRBACGenerator, []string{ClusterPermissionGroups})
}

// createSecretAndRBACInMemberCluster create required secrets and rbac in member cluster
func (o *CommandRegisterOption) createSecretAndRBACInMemberCluster(karmadaAgentCfg *clientcmdapi.Config) error {
	configBytes, err := clientcmd.Write(*karmadaAgentCfg)
	if err != nil {
		return fmt.Errorf("failure while serializing karmada-agent kubeConfig. %w", err)
	}

	// It's necessary to set the label of namespace to make sure that the namespace is created by Karmada.
	labels := map[string]string{
		karmadautil.KarmadaSystemLabel: karmadautil.KarmadaSystemLabelValue,
	}

	kubeConfigSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      KarmadaKubeconfigName,
			Namespace: o.Namespace,
			Labels:    labels,
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{KarmadaKubeconfigName: string(configBytes)},
	}

	// create karmada-kubeconfig secret to be used by karmada-agent component.
	if err := cmdutil.CreateOrUpdateSecret(o.memberClusterClient, kubeConfigSecret); err != nil {
		return fmt.Errorf("create secret %s failed: %v", kubeConfigSecret.Name, err)
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   names.KarmadaAgentComponentName,
			Labels: labels,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
			{
				NonResourceURLs: []string{"*"},
				Verbs:           []string{"get"},
			},
		},
	}

	// create a karmada-agent ClusterRole in member cluster.
	if err := cmdutil.CreateOrUpdateClusterRole(o.memberClusterClient, clusterRole); err != nil {
		return err
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KarmadaAgentServiceAccountName,
			Namespace: o.Namespace,
			Labels:    labels,
		},
	}

	// create service account for karmada-agent
	_, err = karmadautil.EnsureServiceAccountExist(o.memberClusterClient, sa, o.DryRun)
	if err != nil {
		return err
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   names.KarmadaAgentComponentName,
			Labels: labels,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		},
	}

	// grant karmada-agent clusterrole to karmada-agent service account
	if err := cmdutil.CreateOrUpdateClusterRoleBinding(o.memberClusterClient, clusterRoleBinding); err != nil {
		return err
	}

	return nil
}

// makeKarmadaAgentDeployment generate karmada-agent Deployment
func (o *CommandRegisterOption) makeKarmadaAgentDeployment() *appsv1.Deployment {
	karmadaAgent := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.KarmadaAgentComponentName,
			Namespace: o.Namespace,
			Labels:    karmadaAgentLabels,
		},
	}

	var controllers []string
	if o.EnableCertRotation {
		controllers = []string{"*", "certRotation"}
	} else {
		controllers = []string{"*"}
	}

	podSpec := corev1.PodSpec{
		ServiceAccountName: KarmadaAgentServiceAccountName,
		Containers: []corev1.Container{
			{
				Name:  names.KarmadaAgentComponentName,
				Image: o.KarmadaAgentImage,
				Command: []string{
					"/bin/karmada-agent",
					"--karmada-kubeconfig=/etc/kubeconfig/karmada-kubeconfig",
					fmt.Sprintf("--cluster-name=%s", o.ClusterName),
					fmt.Sprintf("--cluster-api-endpoint=%s", o.memberClusterEndpoint),
					fmt.Sprintf("--cluster-provider=%s", o.ClusterProvider),
					fmt.Sprintf("--cluster-region=%s", o.ClusterRegion),
					fmt.Sprintf("--cluster-zones=%s", strings.Join(o.ClusterZones, ",")),
					fmt.Sprintf("--controllers=%s", strings.Join(controllers, ",")),
					fmt.Sprintf("--proxy-server-address=%s", o.ProxyServerAddress),
					fmt.Sprintf("--leader-elect-resource-namespace=%s", o.Namespace),
					fmt.Sprintf("--cluster-namespace=%s", o.ClusterNamespace),
					"--cluster-status-update-frequency=10s",
					"--metrics-bind-address=:8080",
					"--health-probe-bind-address=0.0.0.0:10357",
					"--v=4",
				},
				Ports: []corev1.ContainerPort{
					{
						Name:          "metrics",
						ContainerPort: 8080,
						Protocol:      corev1.ProtocolTCP,
					},
				},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "kubeconfig",
						MountPath: "/etc/kubeconfig",
					},
				},
			},
		},
		Volumes: []corev1.Volume{
			{
				Name: "kubeconfig",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: KarmadaKubeconfigName,
					},
				},
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Key:      "node-role.kubernetes.io/master",
				Operator: corev1.TolerationOpExists,
			},
		},
	}
	// PodTemplateSpec
	podTemplateSpec := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Name:      names.KarmadaAgentComponentName,
			Namespace: o.Namespace,
			Labels:    karmadaAgentLabels,
		},
		Spec: podSpec,
	}
	// DeploymentSpec
	karmadaAgent.Spec = appsv1.DeploymentSpec{
		Replicas: &o.KarmadaAgentReplicas,
		Template: podTemplateSpec,
		Selector: &metav1.LabelSelector{
			MatchLabels: karmadaAgentLabels,
		},
	}

	return karmadaAgent
}

// generateKeyAndCSR generate private key and csr
func generateKeyAndCSR(commonName string, organization []string) (*rsa.PrivateKey, []byte, error) {
	pk, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return nil, nil, err
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: organization,
		},
	}, pk)
	if err != nil {
		return nil, nil, err
	}

	return pk, csr, nil
}

// retrieveValidatedConfigInfo is a private implementation of RetrieveValidatedConfigInfo.
func retrieveValidatedConfigInfo(client kubeclient.Interface, bootstrapTokenDiscovery *BootstrapTokenDiscovery, duration, interval time.Duration, parentCommand string) (*clientcmdapi.Config, error) {
	token, err := tokenutil.NewToken(bootstrapTokenDiscovery.Token)
	if err != nil {
		return nil, err
	}

	// Load the CACertHashes into a pubkeypin.Set
	pubKeyPins := pubkeypin.NewSet()
	if err = pubKeyPins.Allow(bootstrapTokenDiscovery.CACertHashes...); err != nil {
		return nil, fmt.Errorf("invalid discovery token CA certificate hash: %v", err)
	}

	// Make sure the interval is not bigger than the duration
	if interval > duration {
		interval = duration
	}

	endpoint := bootstrapTokenDiscovery.APIServerEndpoint
	insecureBootstrapConfig := buildInsecureBootstrapKubeConfig(endpoint, DefaultClusterName)
	clusterName := insecureBootstrapConfig.Contexts[insecureBootstrapConfig.CurrentContext].Cluster

	klog.V(1).Infof("[discovery] Created cluster-info discovery client, requesting info from %q", endpoint)
	insecureClusterInfo, err := getClusterInfoFromControlPlane(client, insecureBootstrapConfig, token, interval, duration)
	if err != nil {
		return nil, err
	}

	// Validate the token in the cluster info
	insecureKubeconfigBytes, err := validateClusterInfoToken(insecureClusterInfo, token, parentCommand)
	if err != nil {
		return nil, err
	}

	// Load the insecure config
	insecureConfig, err := clientcmd.Load(insecureKubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse the kubeconfig file in the %s ConfigMap: %w", bootstrapapi.ConfigMapClusterInfo, err)
	}

	// The ConfigMap should contain a single cluster
	if len(insecureConfig.Clusters) != 1 {
		return nil, fmt.Errorf("expected the kubeconfig file in the %s ConfigMap to have a single cluster, but it had %d", bootstrapapi.ConfigMapClusterInfo, len(insecureConfig.Clusters))
	}

	// If no TLS root CA pinning was specified, we're done
	if pubKeyPins.Empty() {
		klog.V(1).Infof("[discovery] Cluster info signature and contents are valid and no TLS pinning was specified, will use API Server %q", endpoint)
		return insecureConfig, nil
	}

	// Load and validate the cluster CA from the insecure kubeconfig
	clusterCABytes, err := validateClusterCA(insecureConfig, pubKeyPins)
	if err != nil {
		return nil, err
	}

	// Now that we know the cluster CA, connect back a second time validating with that CA
	secureBootstrapConfig := buildSecureBootstrapKubeConfig(endpoint, clusterCABytes, clusterName)

	klog.V(1).Infof("[discovery] Requesting info from %q again to validate TLS against the pinned public key", endpoint)
	secureClusterInfo, err := getClusterInfoFromControlPlane(client, secureBootstrapConfig, token, interval, duration)
	if err != nil {
		return nil, err
	}

	// Pull the kubeconfig from the securely-obtained ConfigMap and validate that it's the same as what we found the first time
	secureKubeconfigBytes := []byte(secureClusterInfo.Data[bootstrapapi.KubeConfigKey])
	if !bytes.Equal(secureKubeconfigBytes, insecureKubeconfigBytes) {
		return nil, fmt.Errorf("the second kubeconfig from the %s ConfigMap (using validated TLS) was different from the first", bootstrapapi.ConfigMapClusterInfo)
	}

	secureKubeconfig, err := clientcmd.Load(secureKubeconfigBytes)
	if err != nil {
		return nil, fmt.Errorf("couldn't parse the kubeconfig file in the %s ConfigMap: %w", bootstrapapi.ConfigMapClusterInfo, err)
	}

	klog.V(1).Infof("[discovery] Cluster info signature and contents are valid and TLS certificate validates against pinned roots, will use API Server %q", endpoint)

	return secureKubeconfig, nil
}

// buildInsecureBootstrapKubeConfig makes a kubeconfig object that connects insecurely to the API Server for bootstrapping purposes
func buildInsecureBootstrapKubeConfig(endpoint, clustername string) *clientcmdapi.Config {
	controlPlaneEndpoint := fmt.Sprintf("https://%s", endpoint)
	bootstrapConfig := CreateBasic(controlPlaneEndpoint, clustername, BootstrapUserName, []byte{})
	bootstrapConfig.Clusters[clustername].InsecureSkipTLSVerify = true
	return bootstrapConfig
}

// buildSecureBootstrapKubeConfig makes a kubeconfig object that connects securely to the API Server for bootstrapping purposes (validating with the specified CA)
func buildSecureBootstrapKubeConfig(endpoint string, caCert []byte, clustername string) *clientcmdapi.Config {
	controlPlaneEndpoint := fmt.Sprintf("https://%s", endpoint)
	bootstrapConfig := CreateBasic(controlPlaneEndpoint, clustername, BootstrapUserName, caCert)
	return bootstrapConfig
}

// getClusterInfoFromControlPlane creates a client from the given kubeconfig if the given client is nil,
// and requests the cluster info ConfigMap using PollImmediate.
func getClusterInfoFromControlPlane(client kubeclient.Interface, kubeconfig *clientcmdapi.Config, token *tokenutil.Token, interval, duration time.Duration) (*corev1.ConfigMap, error) {
	var cm *corev1.ConfigMap
	var err error

	// Create client from kubeconfig
	if client == nil {
		client, err = ToClientSet(kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.TODO(), duration)
	defer cancel()

	wait.JitterUntil(func() {
		cm, err = client.CoreV1().ConfigMaps(metav1.NamespacePublic).Get(context.TODO(), bootstrapapi.ConfigMapClusterInfo, metav1.GetOptions{})
		if err != nil {
			klog.V(1).Infof("[discovery] Failed to request cluster-info, will try again: %v", err)
			return
		}
		// Even if the ConfigMap is available the JWS signature is patched-in a bit later.
		// Make sure we retry util then.
		if _, ok := cm.Data[bootstrapapi.JWSSignatureKeyPrefix+token.ID]; !ok {
			klog.V(1).Infof("[discovery] The cluster-info ConfigMap does not yet contain a JWS signature for token ID %q, will try again", token.ID)
			err = fmt.Errorf("could not find a JWS signature in the cluster-info ConfigMap for token ID %q", token.ID)
			return
		}
		// Cancel the context on success
		cancel()
	}, interval, 0.3, true, ctx.Done())

	if err != nil {
		return nil, err
	}

	return cm, nil
}

// validateClusterInfoToken validates that the JWS token present in the cluster info ConfigMap is valid
func validateClusterInfoToken(insecureClusterInfo *corev1.ConfigMap, token *tokenutil.Token, parentCommand string) ([]byte, error) {
	insecureKubeconfigString, ok := insecureClusterInfo.Data[bootstrapapi.KubeConfigKey]
	if !ok || len(insecureKubeconfigString) == 0 {
		return nil, fmt.Errorf("there is no %s key in the %s ConfigMap. This API Server isn't set up for token bootstrapping, can't connect",
			bootstrapapi.KubeConfigKey, bootstrapapi.ConfigMapClusterInfo)
	}

	detachedJWSToken, ok := insecureClusterInfo.Data[bootstrapapi.JWSSignatureKeyPrefix+token.ID]
	if !ok || len(detachedJWSToken) == 0 {
		return nil, fmt.Errorf("token id %q is invalid for this cluster or it has expired. Use \"%s token create\" on the karmada-control-plane to create a new valid token", token.ID, parentCommand)
	}

	if !bootstrap.DetachedTokenIsValid(detachedJWSToken, insecureKubeconfigString, token.ID, token.Secret) {
		return nil, fmt.Errorf("failed to verify JWS signature of received cluster info object, can't trust this API Server")
	}

	return []byte(insecureKubeconfigString), nil
}

// validateClusterCA validates the cluster CA found in the insecure kubeconfig
func validateClusterCA(insecureConfig *clientcmdapi.Config, pubKeyPins *pubkeypin.Set) ([]byte, error) {
	var clusterCABytes []byte
	for _, cluster := range insecureConfig.Clusters {
		clusterCABytes = cluster.CertificateAuthorityData
	}

	clusterCAs, err := certutil.ParseCertsPEM(clusterCABytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cluster CA from the %s ConfigMap: %w", bootstrapapi.ConfigMapClusterInfo, err)
	}

	// Validate the cluster CA public key against the pinned set
	err = pubKeyPins.CheckAny(clusterCAs)
	if err != nil {
		return nil, fmt.Errorf("cluster CA found in %s ConfigMap is invalid: %w", bootstrapapi.ConfigMapClusterInfo, err)
	}

	return clusterCABytes, nil
}

// CreateWithToken creates a KubeConfig object with access to the API server with a token
func CreateWithToken(serverURL, clusterName, userName string, caCert []byte, token string) *clientcmdapi.Config {
	config := CreateBasic(serverURL, clusterName, userName, caCert)
	config.AuthInfos[userName] = &clientcmdapi.AuthInfo{
		Token: token,
	}
	return config
}

// CreateWithCert creates a KubeConfig object with access to the API server with a cert
func CreateWithCert(serverURL, clusterName, userName string, caCert []byte, cert []byte, key []byte) *clientcmdapi.Config {
	config := CreateBasic(serverURL, clusterName, userName, caCert)
	config.AuthInfos[userName] = &clientcmdapi.AuthInfo{
		ClientCertificateData: cert,
		ClientKeyData:         key,
	}
	return config
}

// CreateBasic creates a basic, general KubeConfig object that then can be extended
func CreateBasic(serverURL, clusterName, userName string, caCert []byte) *clientcmdapi.Config {
	// Use the cluster and the username as the context name
	contextName := fmt.Sprintf("%s@%s", userName, clusterName)

	return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			clusterName: {
				Server:                   serverURL,
				CertificateAuthorityData: caCert,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:  clusterName,
				AuthInfo: userName,
			},
		},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{},
		CurrentContext: contextName,
	}
}

// ToClientSet converts a KubeConfig object to a client
func ToClientSet(config *clientcmdapi.Config) (*kubeclient.Clientset, error) {
	overrides := clientcmd.ConfigOverrides{Timeout: "10s"}
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create API client configuration from kubeconfig: %w", err)
	}

	client, err := kubeclient.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}
	return client, nil
}

// ToKarmadaClient converts a KubeConfig object to a client
func ToKarmadaClient(config *clientcmdapi.Config) (*karmadaclientset.Clientset, error) {
	overrides := clientcmd.ConfigOverrides{Timeout: "10s"}
	clientConfig, err := clientcmd.NewDefaultClientConfig(*config, &overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create API client configuration from kubeconfig: %w", err)
	}

	karmadaClient, err := karmadaclientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	return karmadaClient, nil
}

func generateAgentUserName(clusterName string) string {
	return ClusterPermissionPrefix + clusterName
}
