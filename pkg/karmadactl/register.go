package karmadactl

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
	"path/filepath"
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
	check "github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	tokenutil "github.com/karmada-io/karmada/pkg/karmadactl/util/bootstraptoken"
	karmadautil "github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/lifted/pubkeypin"
	"github.com/karmada-io/karmada/pkg/version"
)

var (
	registerShort = `Register a cluster to Karmada control plane with PULL mode`
	registerLong  = `Register a cluster to Karmada control plane with PULL mode.`

	registerExample = templates.Examples(`
		# Register cluster into karmada control plane with PULL mode.
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
	ClusterPermissionPrefix = "system:node:"
	// ClusterPermissionGroups defines the organization of karmada agent certificate
	ClusterPermissionGroups = "system:nodes"
	// KarmadaAgentBootstrapKubeConfigFileName defines the file name for the kubeconfig that the karmada-agent will use to do
	// the TLS bootstrap to get itself an unique credential
	KarmadaAgentBootstrapKubeConfigFileName = "bootstrap-karmada-agent.conf"
	// KarmadaAgentKubeConfigFileName defines the file name for the kubeconfig that the karmada-agent will use to do
	// the TLS bootstrap to get itself an unique credential
	KarmadaAgentKubeConfigFileName = "karmada-agent.conf"
	// KarmadaKubeconfigName is the name of karmada kubeconfig
	KarmadaKubeconfigName = "karmada-kubeconfig"
	// KarmadaAgentName is the name of karmada-agent
	KarmadaAgentName = "karmada-agent"
	// KarmadaAgentServiceAccountName is the name of karmada-agent serviceaccount
	KarmadaAgentServiceAccountName = "karmada-agent-sa"
	// SignerName defines the signer name for csr, 'kubernetes.io/kube-apiserver-client-kubelet' can sign the csr automatically
	SignerName = "kubernetes.io/kube-apiserver-client-kubelet"
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

var karmadaAgentLabels = map[string]string{"app": KarmadaAgentName}

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
		Short:                 registerShort,
		Long:                  registerLong,
		Example:               fmt.Sprintf(registerExample, parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
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
	flags.BoolVar(&opts.EnableCertRotation, "enable-cert-rotation", false, "Enable means controller would rotate certificate for karmada-agent when the certificate is about to expire.")
	flags.StringVar(&opts.CACertPath, "ca-cert-path", CACertPath, "The path to the SSL certificate authority used to secure communications between member cluster and karmada-control-plane.")
	flags.StringVar(&opts.BootstrapToken.Token, "token", "", "For token-based discovery, the token used to validate cluster information fetched from the API server.")
	flags.StringSliceVar(&opts.BootstrapToken.CACertHashes, "discovery-token-ca-cert-hash", []string{}, "For token-based discovery, validate that the root CA public key matches this hash (format: \"<type>:<value>\").")
	flags.BoolVar(&opts.BootstrapToken.UnsafeSkipCAVerification, "discovery-token-unsafe-skip-ca-verification", false, "For token-based discovery, allow joining without --discovery-token-ca-cert-hash pinning.")
	flags.DurationVar(&opts.Timeout, "discovery-timeout", DefaultDiscoveryTimeout, "The timeout to discovery karmada apiserver client.")
	flags.StringVar(&opts.KarmadaAgentImage, "karmada-agent-image", fmt.Sprintf("swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-agent:%s", releaseVer.PatchRelease()), "Karmada agent image.")
	flags.Int32Var(&opts.KarmadaAgentReplicas, "karmada-agent-replicas", 1, "Karmada agent replicas.")
	flags.Int32Var(&opts.CertExpirationSeconds, "cert-expiration-seconds", DefaultCertExpirationSeconds, "The expiration time of certificate.")
	flags.BoolVar(&opts.DryRun, "dry-run", false, "Don't apply any changes; just output what would be done.")

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

	// EnableCertRotation indicates if enable certificate rotation for karmada-agent.
	EnableCertRotation bool

	// CACertPath is the path to the SSL certificate authority used to
	// secure comunications between member cluster and karmada-control-plane.
	// Defaults to "/etc/karmada/pki/ca.crt".
	CACertPath string

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

	memberClusterEndpoint string
	memberClusterClient   *kubeclient.Clientset
}

// Complete ensures that options are valid and marshals them if necessary.
func (o *CommandRegisterOption) Complete(args []string) error {
	// Get karmada apiserver endpoint from the command args.
	if len(args) == 0 {
		return fmt.Errorf("karmada apiserver endpoint is required")
	}
	o.BootstrapToken.APIServerEndpoint = args[0]

	if o.KubeConfig == "" {
		env := os.Getenv("KUBECONFIG")
		if env != "" {
			o.KubeConfig = env
		} else {
			o.KubeConfig = defaultKubeConfig
		}
	}

	if !Exists(o.KubeConfig) {
		return ErrEmptyConfig
	}

	restConfig, err := utils.RestConfig(o.Context, o.KubeConfig)
	if err != nil {
		return err
	}

	if len(o.ClusterName) == 0 {
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

	o.memberClusterEndpoint = restConfig.Host

	o.memberClusterClient, err = utils.NewClientSet(restConfig)
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
		return fmt.Errorf("need to varify CACertHashes, or set --discovery-token-unsafe-skip-ca-verification=true")
	}

	if !filepath.IsAbs(o.CACertPath) || !strings.HasSuffix(o.CACertPath, ".crt") {
		return fmt.Errorf("the ca certificate path must be an absolute path: %s", o.CACertPath)
	}

	return nil
}

// Run is the implementation of the 'register' command.
func (o *CommandRegisterOption) Run(parentCommand string) error {
	klog.V(1).Infof("registering cluster. cluster name: %s", o.ClusterName)
	klog.V(1).Infof("registering cluster. cluster namespace: %s", o.ClusterNamespace)

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
	fmt.Println("[prefligt] All pre-flight checks were passed")

	if o.DryRun {
		return nil
	}

	bootstrapKubeConfigFile := filepath.Join(KarmadaDir, KarmadaAgentBootstrapKubeConfigFileName)

	// Deletes the bootstrapKubeConfigFile, so the credential used for TLS bootstrap is removed from disk
	defer os.Remove(bootstrapKubeConfigFile)

	// fetch the bootstrap client to connect to karmada apiserver temporarily
	fmt.Println("[karmada-agent-start] Waiting to perform the TLS Bootstrap")
	bootstrapClient, karmadaClusterInfo, err := o.discoveryBootstrapConfigAndClusterInfo(bootstrapKubeConfigFile, parentCommand)
	if err != nil {
		return err
	}

	// construct the final kubeconfig file used by karmada agent to connect to karmada apiserver
	fmt.Println("[karmada-agent-start] Waiting to construct karmada-agent kubeconfig")
	karmadaAgentCfg, err := o.constructKarmadaAgentConfig(bootstrapClient, karmadaClusterInfo)
	if err != nil {
		return err
	}

	// ensure namespace where the karmada-agent resources be deployed exists in the member cluster
	if _, err := karmadautil.EnsureNamespaceExist(o.memberClusterClient, o.Namespace, o.DryRun); err != nil {
		return err
	}

	// create the necessary secret and RBAC in the member cluster
	fmt.Println("[karmada-agent-start] Waiting the necessary secret and RBAC")
	if err := o.createSecretAndRBACInMemberCluster(karmadaAgentCfg); err != nil {
		return err
	}

	// create karmada-agent Deployment in the member cluster
	fmt.Println("[karmada-agent-start] Waiting karmada-agent Deployment")
	if _, err := o.memberClusterClient.AppsV1().Deployments(o.Namespace).Create(context.TODO(), o.makeKarmadaAgentDeployment(), metav1.CreateOptions{}); err != nil {
		return err
	}

	if err := check.WaitPodReady(o.memberClusterClient, o.Namespace, utils.MapToString(karmadaAgentLabels), int(o.Timeout)); err != nil {
		return err
	}

	fmt.Printf("\ncluster(%s) is joined successfully\n", o.ClusterName)

	return nil
}

// preflight checks the deployment environment of the member cluster
func (o *CommandRegisterOption) preflight() []error {
	var errlist []error

	// check if the given file already exist
	errlist = appendError(errlist, checkFileIfExist(filepath.Join(KarmadaDir, KarmadaAgentBootstrapKubeConfigFileName)))
	errlist = appendError(errlist, checkFileIfExist(filepath.Join(KarmadaDir, KarmadaAgentKubeConfigFileName)))
	errlist = appendError(errlist, checkFileIfExist(CACertPath))

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

		_, err = o.memberClusterClient.AppsV1().Deployments(o.Namespace).Get(context.TODO(), KarmadaAgentName, metav1.GetOptions{})
		if err == nil {
			errlist = append(errlist, fmt.Errorf("%s/%s Deployment already exists", o.Namespace, KarmadaAgentName))
		} else if !apierrors.IsNotFound(err) {
			errlist = append(errlist, err)
		}
	}

	return errlist
}

// appendError append err to errlist
func appendError(errlist []error, err error) []error {
	if err == nil {
		return errlist
	}
	errlist = append(errlist, err)
	return errlist
}

// checkFileIfExist validates if the given file already exist.
func checkFileIfExist(filePath string) error {
	klog.V(1).Infof("validating the existence of file %s", filePath)

	if _, err := os.Stat(filePath); err == nil {
		return fmt.Errorf("%s already exists", filePath)
	}
	return nil
}

// discoveryBootstrapConfigAndClusterInfo get bootstrap-config and cluster-info from control plane
func (o *CommandRegisterOption) discoveryBootstrapConfigAndClusterInfo(bootstrapKubeConfigFile, parentCommand string) (*kubeclient.Clientset, *clientcmdapi.Cluster, error) {
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

	// Write the TLS-Bootstrapped karmada-agent config file down to disk
	klog.V(1).Infof("[discovery] writing bootstrap karmada-agent config file at %s", bootstrapKubeConfigFile)
	if err := WriteToDisk(bootstrapKubeConfigFile, tlsBootstrapCfg); err != nil {
		return nil, nil, fmt.Errorf("couldn't save %s to disk: %w", KarmadaAgentBootstrapKubeConfigFileName, err)
	}

	// Write the ca certificate to disk so karmada-agent can use it for authentication
	cluster := tlsBootstrapCfg.Contexts[tlsBootstrapCfg.CurrentContext].Cluster
	caPath := o.CACertPath
	if _, err := os.Stat(caPath); os.IsNotExist(err) {
		klog.V(1).Infof("[discovery] writing CA certificate at %s", caPath)
		if err := certutil.WriteCert(caPath, tlsBootstrapCfg.Clusters[cluster].CertificateAuthorityData); err != nil {
			return nil, nil, fmt.Errorf("couldn't save the CA certificate to disk: %w", err)
		}
	}

	bootstrapClient, err := ClientSetFromFile(bootstrapKubeConfigFile)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create client from kubeconfig file %q", bootstrapKubeConfigFile)
	}

	return bootstrapClient, clusterinfo, nil
}

// constructKarmadaAgentConfig construct the final kubeconfig used by karmada-agent
func (o *CommandRegisterOption) constructKarmadaAgentConfig(bootstrapClient *kubeclient.Clientset, karmadaClusterInfo *clientcmdapi.Cluster) (*clientcmdapi.Config, error) {
	var cert []byte

	pk, csr, err := generatKeyAndCSR(o.ClusterName)
	if err != nil {
		return nil, err
	}

	pkData, err := keyutil.MarshalPrivateKeyToPEM(pk)
	if err != nil {
		return nil, err
	}

	csrName := o.ClusterName + "-" + k8srand.String(5)

	certificateSigningRequest := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: csrName,
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

	klog.V(1).Infof("waiting for the client certificate to be issued")
	err = wait.Poll(1*time.Second, o.Timeout, func() (done bool, err error) {
		csrOK, err := bootstrapClient.CertificatesV1().CertificateSigningRequests().Get(context.TODO(), csrName, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get the cluster csr %s. err: %v", o.ClusterName, err)
		}

		if csrOK.Status.Certificate != nil {
			klog.V(1).Infof("signing certificate successfully")
			cert = csrOK.Status.Certificate
			return true, nil
		}

		klog.V(1).Infof("waiting for the client certificate to be issued")
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	karmadaAgentCfg := CreateWithCert(
		karmadaClusterInfo.Server,
		DefaultClusterName,
		o.ClusterName,
		karmadaClusterInfo.CertificateAuthorityData,
		cert,
		pkData,
	)

	kubeConfigFile := filepath.Join(KarmadaDir, KarmadaAgentKubeConfigFileName)

	// Write the karmada-agent config file down to disk
	klog.V(1).Infof("writing bootstrap karmada-agent config file at %s", kubeConfigFile)
	if err := WriteToDisk(kubeConfigFile, karmadaAgentCfg); err != nil {
		return nil, fmt.Errorf("couldn't save %s to disk: %w", KarmadaAgentKubeConfigFileName, err)
	}

	return karmadaAgentCfg, nil
}

// createSecretAndRBACInMemberCluster create required secrets and rbac in member cluster
func (o *CommandRegisterOption) createSecretAndRBACInMemberCluster(karmadaAgentCfg *clientcmdapi.Config) error {
	configBytes, err := clientcmd.Write(*karmadaAgentCfg)
	if err != nil {
		return fmt.Errorf("failure while serializing karmada-agent kubeConfig. %w", err)
	}

	kubeConfigSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      KarmadaKubeconfigName,
			Namespace: o.Namespace,
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{KarmadaKubeconfigName: string(configBytes)},
	}

	// cerate karmada-kubeconfig secret to be used by karmada-agent component.
	if err := cmdutil.CreateOrUpdateSecret(o.memberClusterClient, kubeConfigSecret); err != nil {
		return fmt.Errorf("create secret %s failed: %v", kubeConfigSecret.Name, err)
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: KarmadaAgentName,
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
	if err := karmadautil.CreateOrUpdateClusterRole(o.memberClusterClient, clusterRole); err != nil {
		return err
	}

	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      KarmadaAgentServiceAccountName,
			Namespace: o.Namespace,
		},
	}

	// create service account for karmada-agent
	_, err = karmadautil.EnsureServiceAccountExist(o.memberClusterClient, sa, o.DryRun)
	if err != nil {
		return err
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: KarmadaAgentName,
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
	if err := karmadautil.CreateOrUpdateClusterRoleBinding(o.memberClusterClient, clusterRoleBinding); err != nil {
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
			Name:      KarmadaAgentName,
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
				Name:  KarmadaAgentName,
				Image: o.KarmadaAgentImage,
				Command: []string{
					"/bin/karmada-agent",
					"--karmada-kubeconfig=/etc/kubeconfig/karmada-kubeconfig",
					fmt.Sprintf("--cluster-name=%s", o.ClusterName),
					fmt.Sprintf("--cluster-api-endpoint=%s", o.memberClusterEndpoint),
					fmt.Sprintf("--cluster-provider=%s", o.ClusterProvider),
					fmt.Sprintf("--cluster-region=%s", o.ClusterRegion),
					fmt.Sprintf("--controllers=%s", strings.Join(controllers, ",")),
					"--cluster-status-update-frequency=10s",
					"--bind-address=0.0.0.0",
					"--secure-port=10357",
					"--v=4",
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
			Name:      KarmadaAgentName,
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

// generatKeyAndCSR generate private key and csr
func generatKeyAndCSR(clusterName string) (*rsa.PrivateKey, []byte, error) {
	pk, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   ClusterPermissionPrefix + clusterName,
			Organization: []string{ClusterPermissionGroups},
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

// WriteToDisk writes a KubeConfig object down to disk with mode 0600
func WriteToDisk(filename string, kubeconfig *clientcmdapi.Config) error {
	err := clientcmd.WriteToFile(*kubeconfig, filename)
	if err != nil {
		return err
	}

	return nil
}

// ClientSetFromFile returns a ready-to-use client from a kubeconfig file
func ClientSetFromFile(path string) (*kubeclient.Clientset, error) {
	config, err := clientcmd.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load admin kubeconfig: %w", err)
	}
	return ToClientSet(config)
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
