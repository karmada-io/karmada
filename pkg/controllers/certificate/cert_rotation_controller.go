package certificate

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/record"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	"github.com/karmada-io/karmada/pkg/sharedcli/ratelimiterflag"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
)

const (
	// CertRotationControllerName is the controller name that will be used when reporting events.
	CertRotationControllerName = "cert-rotation-controller"

	// SignerName defines the signer name for csr, 'kubernetes.io/kube-apiserver-client-kubelet' can sign the csr automatically
	SignerName = "kubernetes.io/kube-apiserver-client-kubelet"

	// KarmadaKubeconfigName is the name of the secret containing karmada-agent certificate.
	KarmadaKubeconfigName = "karmada-kubeconfig"
)

// CertRotationController is to rotate certificates.
type CertRotationController struct {
	client.Client        // used to operate cluster resources in the control plane.
	KubeClient           clientset.Interface
	EventRecorder        record.EventRecorder
	RESTMapper           meta.RESTMapper
	ClusterClient        *util.ClusterClient
	ClusterClientSetFunc func(string, client.Client, *util.ClientOption) (*util.ClusterClient, error)
	// ClusterClientOption holds the attributes that should be injected to a Kubernetes client.
	ClusterClientOption *util.ClientOption
	PredicateFunc       predicate.Predicate
	InformerManager     genericmanager.MultiClusterInformerManager
	RatelimiterOptions  ratelimiterflag.Options

	// CertRotationCheckingInterval defines the interval of checking if the certificate need to be rotated.
	CertRotationCheckingInterval time.Duration
	// KarmadaKubeconfigNamespace is the namespace of the secret containing karmada-agent certificate.
	KarmadaKubeconfigNamespace string
	// CertRotationRemainingTimeThreshold defines the threshold of remaining time of the valid certificate.
	// If the ratio of remaining time to total time is less than or equal to this threshold, the certificate rotation starts.
	CertRotationRemainingTimeThreshold float64
}

// Reconcile performs a full reconciliation for the object referred to by the Request.
// The Controller will requeue the Request to be processed again if an error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (c *CertRotationController) Reconcile(ctx context.Context, req controllerruntime.Request) (controllerruntime.Result, error) {
	klog.V(4).Infof("Rotating the certificate of karmada-agent for the member cluster: %s", req.NamespacedName.Name)

	var err error

	cluster := &clusterv1alpha1.Cluster{}
	if err := c.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		// The resource may no longer exist, in which case we stop processing.
		if apierrors.IsNotFound(err) {
			return controllerruntime.Result{}, nil
		}

		return controllerruntime.Result{Requeue: true}, err
	}

	if !cluster.DeletionTimestamp.IsZero() {
		return controllerruntime.Result{}, nil
	}

	// create a ClusterClient for the given member cluster
	c.ClusterClient, err = c.ClusterClientSetFunc(cluster.Name, c.Client, c.ClusterClientOption)
	if err != nil {
		klog.Errorf("Failed to create a ClusterClient for the given member cluster: %s, err is: %v", cluster.Name, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	secret, err := c.ClusterClient.KubeClient.CoreV1().Secrets(c.KarmadaKubeconfigNamespace).Get(ctx, KarmadaKubeconfigName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get karmada kubeconfig secret: %v", err)
		return controllerruntime.Result{Requeue: true}, err
	}

	if err = c.syncCertRotation(secret); err != nil {
		klog.Errorf("Failed to rotate the certificate of karmada-agent for the given member cluster: %s, err is: %v", cluster.Name, err)
		return controllerruntime.Result{Requeue: true}, err
	}

	return controllerruntime.Result{RequeueAfter: c.CertRotationCheckingInterval}, nil
}

// SetupWithManager creates a controller and register to controller manager.
func (c *CertRotationController) SetupWithManager(mgr controllerruntime.Manager) error {
	return controllerruntime.NewControllerManagedBy(mgr).
		For(&clusterv1alpha1.Cluster{}, builder.WithPredicates(c.PredicateFunc)).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controller.Options{
			RateLimiter: ratelimiterflag.DefaultControllerRateLimiter(c.RatelimiterOptions),
		}).
		Complete(c)
}

func (c *CertRotationController) syncCertRotation(secret *corev1.Secret) error {
	karmadaKubeconfig, err := getKubeconfigFromSecret(secret)
	if err != nil {
		return err
	}

	clusterName := karmadaKubeconfig.Contexts[karmadaKubeconfig.CurrentContext].AuthInfo
	if clusterName == "" {
		return fmt.Errorf("failed to get cluster name, the current context is %s", karmadaKubeconfig.CurrentContext)
	}

	oldCertData := karmadaKubeconfig.AuthInfos[clusterName].ClientCertificateData

	shouldRotate, err := c.shouldRotateCert(oldCertData)
	if err != nil {
		return err
	}
	if !shouldRotate {
		return nil
	}

	oldCert, err := certutil.ParseCertsPEM(oldCertData)
	if err != nil {
		return fmt.Errorf("unable to parse old certificate: %v", err)
	}

	// create a new private key
	keyData, err := keyutil.MakeEllipticPrivateKeyPEM()
	if err != nil {
		return err
	}

	privateKey, err := keyutil.ParsePrivateKeyPEM(keyData)
	if err != nil {
		return fmt.Errorf("invalid private key for certificate request: %v", err)
	}

	csr, err := c.createCSRInControlPlane(clusterName, privateKey, oldCert)
	if err != nil {
		return fmt.Errorf("failed to create csr in control plane, err is: %v", err)
	}

	var newCertData []byte
	klog.V(1).Infof("Waiting for the client certificate to be issued")
	err = wait.Poll(1*time.Second, 5*time.Minute, func() (done bool, err error) {
		csr, err := c.KubeClient.CertificatesV1().CertificateSigningRequests().Get(context.TODO(), csr, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get the cluster csr %s. err: %v", clusterName, err)
		}

		if csr.Status.Certificate != nil {
			klog.V(1).Infof("Signing certificate successfully")
			newCertData = csr.Status.Certificate
			return true, nil
		}

		klog.V(1).Infof("Waiting for the client certificate to be issued")
		return false, nil
	})
	if err != nil {
		return err
	}

	karmadaKubeconfig.AuthInfos[clusterName].ClientCertificateData = newCertData
	karmadaKubeconfig.AuthInfos[clusterName].ClientKeyData = keyData

	karmadaKubeconfigBytes, err := clientcmd.Write(*karmadaKubeconfig)
	if err != nil {
		return fmt.Errorf("failed to serialize karmada-agent kubeConfig. %v", err)
	}

	secret.Data["karmada-kubeconfig"] = karmadaKubeconfigBytes
	// Update the karmada-kubeconfig secret in the member cluster.
	if _, err := c.ClusterClient.KubeClient.CoreV1().Secrets(secret.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("Unable to update secret, err: %w", err)
	}

	newCert, err := certutil.ParseCertsPEM(newCertData)
	if err != nil {
		klog.Errorf("Unable to parse new certificate: %v", err)
		return err
	}

	klog.V(4).Infof("The certificate has been rotated successfully, new expiration time is %v", newCert[0].NotAfter)

	return nil
}

func (c *CertRotationController) createCSRInControlPlane(clusterName string, privateKey interface{}, oldCert []*x509.Certificate) (string, error) {
	csrData, err := certutil.MakeCSR(privateKey, &oldCert[0].Subject, nil, nil)
	if err != nil {
		return "", fmt.Errorf("unable to generate certificate request: %v", err)
	}

	csrName := clusterName + "-" + k8srand.String(5)

	// Expiration of the new certificate is same with the old certificate
	certExpirationSeconds := int32((oldCert[0].NotAfter.Sub(oldCert[0].NotBefore)).Seconds())

	certificateSigningRequest := &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: csrName,
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Request:           csrData,
			SignerName:        SignerName,
			ExpirationSeconds: &certExpirationSeconds,
			Usages: []certificatesv1.KeyUsage{
				certificatesv1.UsageDigitalSignature,
				certificatesv1.UsageKeyEncipherment,
				certificatesv1.UsageClientAuth,
			},
		},
	}

	_, err = c.KubeClient.CertificatesV1().CertificateSigningRequests().Create(context.TODO(), certificateSigningRequest, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to create certificate request in control plane: %v", err)
	}

	return csrName, nil
}

func getKubeconfigFromSecret(secret *corev1.Secret) (*clientcmdapi.Config, error) {
	if secret.Data == nil {
		return nil, fmt.Errorf("no client certificate found in secret %q", secret.Namespace+"/"+secret.Name)
	}

	karmadaKubeconfigData, ok := secret.Data[KarmadaKubeconfigName]
	if !ok {
		return nil, fmt.Errorf("no karmada kubeconfig found in secret %q", secret.Namespace+"/"+secret.Name)
	}

	karmadaKubeconfig, err := clientcmd.Load(karmadaKubeconfigData)
	if err != nil {
		return nil, err
	}

	return karmadaKubeconfig, nil
}

func (c *CertRotationController) shouldRotateCert(certData []byte) (bool, error) {
	notBefore, notAfter, err := getCertValidityPeriod(certData)
	if err != nil {
		return false, err
	}

	total := notAfter.Sub(*notBefore)
	remaining := time.Until(*notAfter)
	klog.V(4).Infof("The certificate of karmada-agent: time total=%v, remaining=%v, remaining/total=%v", total, remaining, remaining.Seconds()/total.Seconds())

	if remaining.Seconds()/total.Seconds() > c.CertRotationRemainingTimeThreshold {
		// Do nothing if the certificate of karmada-agent is valid and has more than a valid threshold of its life remaining
		klog.V(4).Infof("The certificate of karmada-agent is valid and has more than %.2f%% of its life remaining", c.CertRotationRemainingTimeThreshold*100)
		return false, nil
	}

	klog.V(4).Infof("The certificate of karmada-agent has less than or equal %.2f%% of its life remaining and need to be rotated", c.CertRotationRemainingTimeThreshold*100)
	return true, nil
}

// getCertValidityPeriod returns the validity period of the certificate
func getCertValidityPeriod(certData []byte) (*time.Time, *time.Time, error) {
	certs, err := certutil.ParseCertsPEM(certData)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to parse TLS certificates: %v", err)
	}

	if len(certs) == 0 {
		return nil, nil, errors.New("no cert found in certificate")
	}

	// find out the validity period for all certs in the certificate chain
	var notBefore, notAfter *time.Time
	for index, cert := range certs {
		if index == 0 {
			notBefore = &cert.NotBefore
			notAfter = &cert.NotAfter
			continue
		}

		if notBefore.Before(cert.NotBefore) {
			notBefore = &cert.NotBefore
		}

		if notAfter.After(cert.NotAfter) {
			notAfter = &cert.NotAfter
		}
	}

	return notBefore, notAfter, nil
}
