/*
   Copyright 2018 The Kubernetes Authors.
   Copy From: https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/kubectl/pkg/cmd/get/get_flags.go
   Change:  ConfigFlags struct add CaBundle fields, toRawKubeConfigLoader method modify new loadRules and overrides, remove AddFlags function.
*/

package karmadactl

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

var (
	defaultKubeConfig = filepath.Join(homedir.HomeDir(), ".kube", "config")
	defaultCacheDir   = filepath.Join(homedir.HomeDir(), ".kube", "cache")
	// ErrEmptyConfig is the error message to be displayed if the configuration info is missing or incomplete
	ErrEmptyConfig = clientcmd.NewEmptyConfigError(
		`Missing or incomplete configuration info.  Please point to an existing, complete config file:
  1. Via the command-line flag --kubeconfig
  2. Via the KUBECONFIG environment variable
  3. In your home directory as ~/.kube/config

To view or setup config directly use the 'config' command.`)
)

// RESTClientGetter is an interface that the ConfigFlags describe to provide an easier way to mock for commands
// and eliminate the direct coupling to a struct type.  Users may wish to duplicate this type in their own packages
// as per the golang type overlapping.
type RESTClientGetter interface {
	// ToRESTConfig returns restconfig
	ToRESTConfig() (*rest.Config, error)
	// ToDiscoveryClient returns discovery client
	ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error)
	// ToRESTMapper returns a restmapper
	ToRESTMapper() (meta.RESTMapper, error)
	// ToRawKubeConfigLoader return kubeconfig loader as-is
	ToRawKubeConfigLoader() clientcmd.ClientConfig
}

type clientConfig struct {
	defaultClientConfig clientcmd.ClientConfig
}

func (c *clientConfig) RawConfig() (clientcmdapi.Config, error) {
	config, err := c.defaultClientConfig.RawConfig()
	// replace client-go's ErrEmptyConfig error with our custom, more verbose version
	if clientcmd.IsEmptyConfig(err) {
		return config, ErrEmptyConfig
	}
	return config, err
}

func (c *clientConfig) ClientConfig() (*rest.Config, error) {
	config, err := c.defaultClientConfig.ClientConfig()
	// replace client-go's ErrEmptyConfig error with our custom, more verbose version
	if clientcmd.IsEmptyConfig(err) {
		return config, ErrEmptyConfig
	}
	return config, err
}

func (c *clientConfig) Namespace() (string, bool, error) {
	namespace, ok, err := c.defaultClientConfig.Namespace()
	// replace client-go's ErrEmptyConfig error with our custom, more verbose version
	if clientcmd.IsEmptyConfig(err) {
		return namespace, ok, ErrEmptyConfig
	}
	return namespace, ok, err
}

func (c *clientConfig) ConfigAccess() clientcmd.ConfigAccess {
	return c.defaultClientConfig.ConfigAccess()
}

var _ RESTClientGetter = &ConfigFlags{}

// ConfigFlags composes the set of values necessary
// for obtaining a REST client config
type ConfigFlags struct {
	CaBundle *string

	// Default cache director
	CacheDir *string
	// Path to the kubeconfig file to use for CLI requests.
	KubeConfig *string

	// The name of the kubeconfig cluster to use
	ClusterName *string
	// The name of the kubeconfig user to use
	AuthInfoName *string
	// The name of the kubeconfig context to use
	Context *string
	// If present, the namespace scope for this CLI request
	Namespace *string
	// The address and port of the Kubernetes API server
	APIServer *string
	// Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
	TLSServerName *string
	// If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
	Insecure *bool
	// Path to a client certificate file for TLS
	CertFile *string
	// Path to a client key file for TLS
	KeyFile *string
	// Path to a cert file for the certificate authority
	CAFile *string
	// Bearer token for authentication to the API server
	BearerToken      *string
	Impersonate      *string
	ImpersonateGroup *[]string
	Username         *string
	Password         *string
	// The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests.
	Timeout *string
	// If non-nil, wrap config function can transform the Config
	// before it is returned in ToRESTConfig function.
	WrapConfigFn func(*rest.Config) *rest.Config

	clientConfig clientcmd.ClientConfig
	lock         sync.Mutex
	// If set to true, will use persistent client config and
	// propagate the config to the places that need it, rather than
	// loading the config multiple times
	usePersistentConfig bool
	// Allows increasing burst used for discovery, this is useful
	// in clusters with many registered resources
	discoveryBurst int
}

// ToRESTConfig implements RESTClientGetter.
// Returns a REST client configuration based on a provided path
// to a .kubeconfig file, loading rules, and config flag overrides.
// Expects the AddFlags method to have been called. If WrapConfigFn
// is non-nil this function can transform config before return.
func (f *ConfigFlags) ToRESTConfig() (*rest.Config, error) {
	c, err := f.ToRawKubeConfigLoader().ClientConfig()
	if err != nil {
		return nil, err
	}
	if f.WrapConfigFn != nil {
		return f.WrapConfigFn(c), nil
	}
	return c, nil
}

// ToRawKubeConfigLoader binds config flag values to config overrides
// Returns an interactive clientConfig if the password flag is enabled,
// or a non-interactive clientConfig otherwise.
func (f *ConfigFlags) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	if f.usePersistentConfig {
		return f.toRawKubePersistentConfigLoader()
	}
	return f.toRawKubeConfigLoader()
}

//nolint:gocyclo
func (f *ConfigFlags) toRawKubeConfigLoader() clientcmd.ClientConfig {
	//loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules := &clientcmd.ClientConfigLoadingRules{}
	// use the standard defaults for this client command
	// DEPRECATED: remove and replace with something more accurate
	loadingRules.DefaultClientConfig = &clientcmd.DefaultClientConfig

	if f.KubeConfig != nil {
		loadingRules.ExplicitPath = *f.KubeConfig
	}

	clusterOverrides := clientcmd.ClusterDefaults
	if f.CaBundle != nil {
		clusterOverrides.CertificateAuthorityData = []byte(*f.CaBundle)
	}

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clusterOverrides}

	// bind auth info flag values to overrides
	if f.CertFile != nil {
		overrides.AuthInfo.ClientCertificate = *f.CertFile
	}
	if f.KeyFile != nil {
		overrides.AuthInfo.ClientKey = *f.KeyFile
	}
	if f.BearerToken != nil {
		overrides.AuthInfo.Token = *f.BearerToken
	}

	// bind cluster flags
	if f.APIServer != nil {
		overrides.ClusterInfo.Server = *f.APIServer
	}
	if f.TLSServerName != nil {
		overrides.ClusterInfo.TLSServerName = *f.TLSServerName
	}
	if f.CAFile != nil {
		overrides.ClusterInfo.CertificateAuthority = *f.CAFile
	}
	if f.Insecure != nil {
		overrides.ClusterInfo.InsecureSkipTLSVerify = *f.Insecure
	}

	// bind context flags
	if f.Context != nil {
		overrides.CurrentContext = *f.Context
	}
	if f.ClusterName != nil {
		overrides.Context.Cluster = *f.ClusterName
	}
	if f.AuthInfoName != nil {
		overrides.Context.AuthInfo = *f.AuthInfoName
	}
	if f.Namespace != nil {
		overrides.Context.Namespace = *f.Namespace
	}

	if f.Timeout != nil {
		overrides.Timeout = *f.Timeout
	}

	// we only have an interactive prompt when a password is allowed
	if f.Password == nil {
		return &clientConfig{clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)}
	}
	return &clientConfig{clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, overrides, os.Stdin)}
}

// toRawKubePersistentConfigLoader binds config flag values to config overrides
// Returns a persistent clientConfig for propagation.
func (f *ConfigFlags) toRawKubePersistentConfigLoader() clientcmd.ClientConfig {
	f.lock.Lock()
	defer f.lock.Unlock()

	if f.clientConfig == nil {
		f.clientConfig = f.toRawKubeConfigLoader()
	}

	return f.clientConfig
}

// ToDiscoveryClient implements RESTClientGetter.
// Expects the AddFlags method to have been called.
// Returns a CachedDiscoveryInterface using a computed RESTConfig.
func (f *ConfigFlags) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	// The more groups you have, the more discovery requests you need to make.
	// given 25 groups (our groups + a few custom resources) with one-ish version each, discovery needs to make 50 requests
	// double it just so we don't end up here again for a while.  This config is only used for discovery.
	config.Burst = f.discoveryBurst

	cacheDir := defaultCacheDir

	// retrieve a user-provided value for the "cache-dir"
	// override httpCacheDir and discoveryCacheDir if user-value is given.
	if f.CacheDir != nil {
		cacheDir = *f.CacheDir
	}
	httpCacheDir := filepath.Join(cacheDir, "http")
	discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(cacheDir, "discovery"), config.Host)

	return diskcached.NewCachedDiscoveryClientForConfig(config, discoveryCacheDir, httpCacheDir, time.Duration(10*time.Minute))
}

// ToRESTMapper returns a mapper.
func (f *ConfigFlags) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

// WithDeprecatedPasswordFlag enables the username and password config flags
func (f *ConfigFlags) WithDeprecatedPasswordFlag() *ConfigFlags {
	f.Username = stringptr("")
	f.Password = stringptr("")
	return f
}

// WithDiscoveryBurst sets the RESTClient burst for discovery.
func (f *ConfigFlags) WithDiscoveryBurst(discoveryBurst int) *ConfigFlags {
	f.discoveryBurst = discoveryBurst
	return f
}

// NewConfigFlags returns ConfigFlags with default values set
func NewConfigFlags(usePersistentConfig bool) *ConfigFlags {
	impersonateGroup := []string{}
	insecure := false

	return &ConfigFlags{
		Insecure:   &insecure,
		Timeout:    stringptr("0"),
		KubeConfig: stringptr(""),

		CacheDir:         stringptr(defaultCacheDir),
		ClusterName:      stringptr(""),
		AuthInfoName:     stringptr(""),
		Context:          stringptr(""),
		Namespace:        stringptr(""),
		APIServer:        stringptr(""),
		TLSServerName:    stringptr(""),
		CertFile:         stringptr(""),
		KeyFile:          stringptr(""),
		CAFile:           stringptr(""),
		BearerToken:      stringptr(""),
		Impersonate:      stringptr(""),
		ImpersonateGroup: &impersonateGroup,

		usePersistentConfig: usePersistentConfig,
		// The more groups you have, the more discovery requests you need to make.
		// given 25 groups (our groups + a few custom resources) with one-ish version each, discovery needs to make 50 requests
		// double it just so we don't end up here again for a while.  This config is only used for discovery.
		discoveryBurst: 100,
	}
}

func stringptr(val string) *string {
	return &val
}

// overlyCautiousIllegalFileCharacters matches characters that *might* not be supported.  Windows is really restrictive, so this is really restrictive
var overlyCautiousIllegalFileCharacters = regexp.MustCompile(`[^(\w/\.)]`)

// computeDiscoverCacheDir takes the parentDir and the host and comes up with a "usually non-colliding" name.
func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if its there:
	schemelessHost := strings.Replace(strings.Replace(host, "https://", "", 1), "http://", "", 1)
	// now do a simple collapse of non-AZ09 characters.  Collisions are possible but unlikely.  Even if we do collide the problem is short lived
	safeHost := overlyCautiousIllegalFileCharacters.ReplaceAllString(schemelessHost, "_")
	return filepath.Join(parentDir, safeHost)
}
