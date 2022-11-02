package bootstraptoken

import (
	"context"
	"crypto/x509"
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcertutil "k8s.io/client-go/util/cert"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	bootstraputil "k8s.io/cluster-bootstrap/token/util"
	bootstrapsecretutil "k8s.io/cluster-bootstrap/util/secrets"
	"k8s.io/klog/v2"

	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util/lifted/pubkeypin"
)

const (
	// When a token is matched with 'BootstrapTokenPattern', the size of validated substrings returned by
	// regexp functions which contains 'Submatch' in their names will be 3.
	// Submatch 0 is the match of the entire expression, submatch 1 is
	// the match of the first parenthesized subexpression, and so on.
	// e.g.:
	// result := bootstraputil.BootstrapTokenRegexp.FindStringSubmatch("abcdef.1234567890123456")
	// result == []string{"abcdef.1234567890123456","abcdef","1234567890123456"}
	// len(result) == 3
	validatedSubstringsSize = 3

	// DefaultTokenDuration specifies the default amount of time that a bootstrap token will be valid
	// Default behaviour is 24 hours
	DefaultTokenDuration = 24 * time.Hour
)

var (
	// DefaultUsages is the default usages of bootstrap token
	DefaultUsages = bootstrapapi.KnownTokenUsages
	// DefaultGroups is the default groups of bootstrap token
	DefaultGroups = []string{"system:bootstrappers:karmada:default-cluster-token"}
)

// BootstrapToken describes one bootstrap token, stored as a Secret in the cluster
type BootstrapToken struct {
	// Token is used for establishing bidirectional trust between clusters and karmada-control-plane.
	// Used for joining clusters to the karmada-control-plane.
	Token *Token
	// Description sets a human-friendly message why this token exists and what it's used
	// for, so other administrators can know its purpose.
	// +optional
	Description string
	// TTL defines the time to live for this token. Defaults to 24h.
	// Expires and TTL are mutually exclusive.
	// +optional
	TTL *metav1.Duration
	// Expires specifies the timestamp when this token expires. Defaults to being set
	// dynamically at runtime based on the TTL. Expires and TTL are mutually exclusive.
	// +optional
	Expires *metav1.Time
	// Usages describes the ways in which this token can be used. Can by default be used
	// for establishing bidirectional trust, but that can be changed here.
	// +optional
	Usages []string
	// Groups specifies the extra groups that this token will authenticate as when/if
	// used for authentication
	// +optional
	Groups []string
}

// Token is a token of the format abcdef.abcdef0123456789 that is used
// for both validation of the practically of the API server from a joining cluster's point
// of view and as an authentication method for the cluster in the bootstrap phase of
// "karmadactl join". This token is and should be short-lived
type Token struct {
	ID     string
	Secret string
}

// GenerateRegisterCommand generate register command that will be printed
func GenerateRegisterCommand(kubeConfig, parentCommand, token string) (string, error) {
	klog.V(1).Info("print register command")
	// load the kubeconfig file to get the CA certificate and endpoint
	config, err := clientcmd.LoadFromFile(kubeConfig)
	if err != nil {
		return "", fmt.Errorf("failed to load kubeconfig, err: %w", err)
	}

	// load the default cluster config
	clusterConfig := GetClusterFromKubeConfig(config)
	if clusterConfig == nil {
		return "", fmt.Errorf("failed to get default cluster config")
	}

	// load CA certificates from the kubeconfig (either from PEM data or by file path)
	var caCerts []*x509.Certificate
	if clusterConfig.CertificateAuthorityData != nil {
		caCerts, err = clientcertutil.ParseCertsPEM(clusterConfig.CertificateAuthorityData)
		if err != nil {
			return "", fmt.Errorf("failed to parse CA certificate from kubeconfig, err: %w", err)
		}
	} else if clusterConfig.CertificateAuthority != "" {
		caCerts, err = clientcertutil.CertsFromFile(clusterConfig.CertificateAuthority)
		if err != nil {
			return "", fmt.Errorf("failed to load CA certificate referenced by kubeconfig, err: %w", err)
		}
	} else {
		return "", fmt.Errorf("no CA certificates found in kubeconfig")
	}

	// hash all the CA certs and include their public key pins as trusted values
	publicKeyPins := make([]string, 0, len(caCerts))
	for _, caCert := range caCerts {
		publicKeyPins = append(publicKeyPins, pubkeypin.Hash(caCert))
	}

	return fmt.Sprintf("%s register %s --token %s --discovery-token-ca-cert-hash %s",
		parentCommand, strings.Replace(clusterConfig.Server, "https://", "", -1),
		token, strings.Join(publicKeyPins, ",")), nil
}

// GetClusterFromKubeConfig returns the default Cluster of the specified KubeConfig
func GetClusterFromKubeConfig(config *clientcmdapi.Config) *clientcmdapi.Cluster {
	// If there is an unnamed cluster object, use it
	if config.Clusters[""] != nil {
		return config.Clusters[""]
	}
	if config.Contexts[config.CurrentContext] != nil {
		return config.Clusters[config.Contexts[config.CurrentContext].Cluster]
	}
	return nil
}

// GenerateRandomBootstrapToken generate random bootstrap token
func GenerateRandomBootstrapToken(ttl *metav1.Duration, description string, groups, usages []string) (*BootstrapToken, error) {
	tokenStr, err := bootstraputil.GenerateBootstrapToken()
	if err != nil {
		return nil, fmt.Errorf("couldn't generate random token, err: %w", err)
	}

	token, err := NewToken(tokenStr)
	if err != nil {
		return nil, err
	}

	bt := &BootstrapToken{
		Token:       token,
		TTL:         ttl,
		Description: description,
		Groups:      groups,
		Usages:      usages,
	}

	return bt, nil
}

// NewToken converts the given Bootstrap Token as a string
// to the Token object used for serialization/deserialization
// and internal usage. It also automatically validates that the given token
// is of the right format
func NewToken(token string) (*Token, error) {
	substrs := bootstraputil.BootstrapTokenRegexp.FindStringSubmatch(token)
	if len(substrs) != validatedSubstringsSize {
		return nil, fmt.Errorf("the bootstrap token %q was not of the form %q", token, bootstrapapi.BootstrapTokenPattern)
	}

	return &Token{ID: substrs[1], Secret: substrs[2]}, nil
}

// ConvertBootstrapTokenToSecret converts the given BootstrapToken object to its Secret representation that
// may be submitted to the API Server in order to be stored.
func ConvertBootstrapTokenToSecret(bt *BootstrapToken) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bootstraputil.BootstrapTokenSecretName(bt.Token.ID),
			Namespace: metav1.NamespaceSystem,
		},
		Type: corev1.SecretType(bootstrapapi.SecretTypeBootstrapToken),
		Data: encodeTokenSecretData(bt, time.Now()),
	}
}

// encodeTokenSecretData takes the token discovery object and an optional duration and returns the .Data for the Secret
// now is passed in order to be able to used in unit testing
func encodeTokenSecretData(token *BootstrapToken, now time.Time) map[string][]byte {
	data := map[string][]byte{
		bootstrapapi.BootstrapTokenIDKey:     []byte(token.Token.ID),
		bootstrapapi.BootstrapTokenSecretKey: []byte(token.Token.Secret),
	}

	if len(token.Description) > 0 {
		data[bootstrapapi.BootstrapTokenDescriptionKey] = []byte(token.Description)
	}

	// If for some strange reason both token.TTL and token.Expires would be set
	// (they are mutually exclusive in validation so this shouldn't be the case),
	// token.Expires has higher priority, as can be seen in the logic here.
	if token.Expires != nil {
		// Format the expiration date accordingly
		// TODO: This maybe should be a helper function in bootstraputil?
		expirationString := token.Expires.Time.UTC().Format(time.RFC3339)
		data[bootstrapapi.BootstrapTokenExpirationKey] = []byte(expirationString)
	} else if token.TTL != nil && token.TTL.Duration > 0 {
		// Only if .Expires is unset, TTL might have an effect
		// Get the current time, add the specified duration, and format it accordingly
		expirationString := now.Add(token.TTL.Duration).UTC().Format(time.RFC3339)
		data[bootstrapapi.BootstrapTokenExpirationKey] = []byte(expirationString)
	}

	for _, usage := range token.Usages {
		data[bootstrapapi.BootstrapTokenUsagePrefix+usage] = []byte("true")
	}

	if len(token.Groups) > 0 {
		data[bootstrapapi.BootstrapTokenExtraGroupsKey] = []byte(strings.Join(token.Groups, ","))
	}
	return data
}

// NewTokenFromIDAndSecret is a wrapper around NewToken
// that allows the caller to specify the ID and Secret separately
func NewTokenFromIDAndSecret(id, secret string) (*Token, error) {
	return NewToken(bootstraputil.TokenFromIDAndSecret(id, secret))
}

// GetBootstrapTokenFromSecret returns a BootstrapToken object from the given Secret
func GetBootstrapTokenFromSecret(secret *corev1.Secret) (*BootstrapToken, error) {
	// Get the Token ID field from the Secret data
	tokenID := bootstrapsecretutil.GetData(secret, bootstrapapi.BootstrapTokenIDKey)
	if len(tokenID) == 0 {
		return nil, fmt.Errorf("bootstrap Token Secret has no token-id data: %s", secret.Name)
	}

	// Enforce the right naming convention
	if secret.Name != bootstraputil.BootstrapTokenSecretName(tokenID) {
		return nil, fmt.Errorf("bootstrap token name is not of the form '%s(token-id)'. Actual: %q. Expected: %q",
			bootstrapapi.BootstrapTokenSecretPrefix, secret.Name, bootstraputil.BootstrapTokenSecretName(tokenID))
	}

	tokenSecret := bootstrapsecretutil.GetData(secret, bootstrapapi.BootstrapTokenSecretKey)
	if len(tokenSecret) == 0 {
		return nil, fmt.Errorf("bootstrap Token Secret has no token-secret data: %s", secret.Name)
	}

	// Create the Token object based on the ID and Secret
	bts, err := NewTokenFromIDAndSecret(tokenID, tokenSecret)
	if err != nil {
		return nil, fmt.Errorf("bootstrap Token Secret is invalid and couldn't be parsed, err: %w", err)
	}

	// Get the description (if any) from the Secret
	description := bootstrapsecretutil.GetData(secret, bootstrapapi.BootstrapTokenDescriptionKey)

	// Expiration time is optional, if not specified this implies the token
	// never expires.
	secretExpiration := bootstrapsecretutil.GetData(secret, bootstrapapi.BootstrapTokenExpirationKey)
	var expires *metav1.Time
	if len(secretExpiration) > 0 {
		expTime, err := time.Parse(time.RFC3339, secretExpiration)
		if err != nil {
			return nil, fmt.Errorf("can't parse expiration time of bootstrap token %q, err: %w", secret.Name, err)
		}
		expires = &metav1.Time{Time: expTime}
	}

	// Build an usages string slice from the Secret data
	var usages []string
	for k, v := range secret.Data {
		// Skip all fields that don't include this prefix
		if !strings.HasPrefix(k, bootstrapapi.BootstrapTokenUsagePrefix) {
			continue
		}
		// Skip those that don't have this usage set to true
		if string(v) != "true" {
			continue
		}
		usages = append(usages, strings.TrimPrefix(k, bootstrapapi.BootstrapTokenUsagePrefix))
	}
	// Only sort the slice if defined
	if usages != nil {
		sort.Strings(usages)
	}

	// Get the extra groups information from the Secret
	// It's done this way to make .Groups be nil in case there is no items, rather than an
	// empty slice or an empty slice with a "" string only
	var groups []string
	groupsString := bootstrapsecretutil.GetData(secret, bootstrapapi.BootstrapTokenExtraGroupsKey)
	g := strings.Split(groupsString, ",")
	if len(g) > 0 && len(g[0]) > 0 {
		groups = g
	}

	return &BootstrapToken{
		Token:       bts,
		Description: description,
		Expires:     expires,
		Usages:      usages,
		Groups:      groups,
	}, nil
}

// CreateNewToken tries to create a token and fails if one with the same ID already exists
func CreateNewToken(client kubeclient.Interface, token *BootstrapToken) error {
	return UpdateOrCreateToken(client, true, token)
}

// UpdateOrCreateToken attempts to update a token with the given ID, or create if it does not already exist.
func UpdateOrCreateToken(client kubeclient.Interface, failIfExists bool, token *BootstrapToken) error {
	secretName := bootstraputil.BootstrapTokenSecretName(token.Token.ID)
	secret, err := client.CoreV1().Secrets(metav1.NamespaceSystem).Get(context.TODO(), secretName, metav1.GetOptions{})
	if secret != nil && err == nil && failIfExists {
		return fmt.Errorf("a token with id %q already exists", token.Token.ID)
	}

	updatedOrNewSecret := ConvertBootstrapTokenToSecret(token)
	// Try to create or update the token with an exponential backoff
	err = TryRunCommand(func() error {
		if err := cmdutil.CreateOrUpdateSecret(client, updatedOrNewSecret); err != nil {
			return fmt.Errorf("failed to create or update bootstrap token with name %s, err: %w", secretName, err)
		}
		return nil
	}, 5)
	if err != nil {
		return err
	}

	return nil
}

// TryRunCommand runs a function a maximum of failureThreshold times, and retries on error. If failureThreshold is hit; the last error is returned
func TryRunCommand(f func() error, failureThreshold int) error {
	backoff := wait.Backoff{
		Duration: 5 * time.Second,
		Factor:   2, // double the timeout for every failure
		Steps:    failureThreshold,
	}
	return wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := f()
		if err != nil {
			// Retry until the timeout
			return false, nil
		}
		// The last f() call was a success, return cleanly
		return true, nil
	})
}
