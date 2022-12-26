package karmada

import (
	"context"
	"fmt"
	"net"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"
	netutils "k8s.io/utils/net"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/scheme"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/certs"
)

var certList = []string{
	"ca",
	"etcd-ca",
	"etcd-server",
	"etcd-client",
	"karmada",
	"apiserver",
	"front-proxy-ca",
	"front-proxy-client",
}

func (ctrl *Controller) genCerts(karmada *operatorv1alpha1.Karmada, karmadaAPIServerIP []net.IP) error {
	notAfter := time.Now().Add(certs.Duration365d).UTC()

	var etcdServerCertDNS = []string{
		"localhost",
		fmt.Sprintf("%s.%s.svc", constants.KarmadaComponentEtcd, karmada.Namespace),
		fmt.Sprintf("%s.%s.svc.%s", constants.KarmadaComponentEtcd, karmada.Namespace, "cluster.local"),
	}
	for number := int32(0); number < 1; number++ {
		etcdServerCertDNS = append(etcdServerCertDNS, fmt.Sprintf("%s-%v.%s.%s.svc", constants.KarmadaComponentEtcd, number, constants.KarmadaComponentEtcd, karmada.Namespace))
		etcdServerCertDNS = append(etcdServerCertDNS, fmt.Sprintf("%s-%v.%s.%s.svc.%s", constants.KarmadaComponentEtcd, number, constants.KarmadaComponentEtcd, karmada.Namespace, "cluster.local"))
	}

	etcdServerAltNames := certutil.AltNames{
		DNSNames: etcdServerCertDNS,
		IPs:      []net.IP{netutils.ParseIPSloppy("127.0.0.1")},
	}
	etcdServerCertConfig := certs.NewCertConfig("karmada-etcd-server", []string{}, etcdServerAltNames, &notAfter)
	etcdClientCertCfg := certs.NewCertConfig("karmada-etcd-client", []string{}, certutil.AltNames{}, &notAfter)

	var karmadaDNS = []string{
		"localhost",
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		constants.KarmadaComponentKubeAPIServer,
		constants.KarmadaComponentWebhook,
		constants.KarmadaComponentAggregratedAPIServer,
		fmt.Sprintf("%s.%s.svc", constants.KarmadaComponentKubeAPIServer, karmada.Namespace),
		fmt.Sprintf("%s.%s.svc.%s", constants.KarmadaComponentKubeAPIServer, karmada.Namespace, "cluster.local"),
		fmt.Sprintf("%s.%s.svc.%s", constants.KarmadaComponentWebhook, karmada.Namespace, "cluster.local"),
		fmt.Sprintf("%s.%s.svc", constants.KarmadaComponentWebhook, karmada.Namespace),
		fmt.Sprintf("%s.%s.svc.%s", constants.KarmadaComponentAggregratedAPIServer, karmada.Namespace, "cluster.local"),
		fmt.Sprintf("*.%s.svc.%s", karmada.Namespace, "cluster.local"),
		fmt.Sprintf("*.%s.svc", karmada.Namespace),
	}

	karmadaIPs := []net.IP{}
	karmadaIPs = append(
		karmadaIPs,
		netutils.ParseIPSloppy("127.0.0.1"),
		netutils.ParseIPSloppy("10.254.0.1"),
	)
	if len(karmadaAPIServerIP) > 0 {
		karmadaIPs = append(karmadaIPs, karmadaAPIServerIP...)
	}

	internetIP, err := util.InternetIP()
	if err != nil {
		klog.Warningln("Failed to obtain internet IP. ", err)
	} else {
		karmadaIPs = append(karmadaIPs, internetIP)
	}

	karmadaAltNames := certutil.AltNames{
		DNSNames: karmadaDNS,
		IPs:      karmadaIPs,
	}
	karmadaCertCfg := certs.NewCertConfig("system:admin", []string{"system:masters"}, karmadaAltNames, &notAfter)

	apiserverCertCfg := certs.NewCertConfig("karmada-apiserver", []string{""}, karmadaAltNames, &notAfter)

	frontProxyClientCertCfg := certs.NewCertConfig("front-proxy-client", []string{}, certutil.AltNames{}, &notAfter)
	data, err := certs.GenCerts(etcdServerCertConfig, etcdClientCertCfg, karmadaCertCfg, apiserverCertCfg, frontProxyClientCertCfg)
	if err != nil {
		return err
	}

	// Create kubeconfig Secret
	karmadaServerURL := fmt.Sprintf("https://%s.%s.svc.%s:%v", constants.KarmadaComponentKubeAPIServer, karmada.Namespace, "cluster.local", 5443)
	config := certs.CreateWithCerts(karmadaServerURL, "karmada-admin", "karmada-admin", data["ca.crt"], data["karmada.key"], data["karmada.crt"])
	configBytes, err := clientcmd.Write(*config)
	if err != nil {
		return fmt.Errorf("failure while serializing admin kubeConfig. %v", err)
	}

	kubeConfigSecret := SecretFromSpec(karmada.Namespace, "karmada-kubeconfig", corev1.SecretTypeOpaque, map[string]string{"kubeconfig": string(configBytes)})
	controllerutil.SetOwnerReference(karmada, kubeConfigSecret, scheme.Scheme)
	err = ctrl.Create(context.TODO(), kubeConfigSecret)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Create certs Secret
	etcdCert := map[string]string{
		"etcd-ca.crt":     string(data["etcd-ca.crt"]),
		"etcd-ca.key":     string(data["etcd-ca.key"]),
		"etcd-server.crt": string(data["etcd-server.crt"]),
		"etcd-server.key": string(data["etcd-server.key"]),
	}
	etcdSecret := SecretFromSpec(karmada.Namespace, fmt.Sprintf("%s-cert", constants.KarmadaComponentEtcd), corev1.SecretTypeOpaque, etcdCert)
	controllerutil.SetOwnerReference(karmada, etcdSecret, scheme.Scheme)
	err = ctrl.Create(context.TODO(), etcdSecret)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	karmadaCert := map[string]string{}
	for _, v := range certList {
		karmadaCert[fmt.Sprintf("%s.crt", v)] = string(data[fmt.Sprintf("%s.crt", v)])
		karmadaCert[fmt.Sprintf("%s.key", v)] = string(data[fmt.Sprintf("%s.key", v)])
	}
	karmadaSecret := SecretFromSpec(karmada.Namespace, "karmada-cert", corev1.SecretTypeOpaque, karmadaCert)
	controllerutil.SetOwnerReference(karmada, karmadaSecret, scheme.Scheme)
	err = ctrl.Create(context.TODO(), karmadaSecret)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	karmadaWebhookCert := map[string]string{
		"tls.crt": string(data["karmada.crt"]),
		"tls.key": string(data["karmada.key"]),
	}
	karmadaWebhookSecret := SecretFromSpec(karmada.Namespace, fmt.Sprintf("%s-cert", constants.KarmadaComponentWebhook), corev1.SecretTypeOpaque, karmadaWebhookCert)
	controllerutil.SetOwnerReference(karmada, karmadaWebhookSecret, scheme.Scheme)
	err = ctrl.Create(context.TODO(), karmadaWebhookSecret)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func SecretFromSpec(namespace, name string, secretType corev1.SecretType, data map[string]string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    map[string]string{"karmada.io/bootstrapping": "secret-defaults"},
		},
		//Immutable:  immutable,
		Type:       secretType,
		StringData: data,
	}
}
