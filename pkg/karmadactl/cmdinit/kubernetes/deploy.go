package kubernetes

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/cert"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/karmada"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

//InstallOptions kubernetes options
type InstallOptions struct {
	Namespace          string
	KubeClientSet      *kubernetes.Clientset
	CertAndKeyFileData map[string][]byte
	RestConfig         *rest.Config
	MasterIP           []net.IP
}

//  genCerts create ca etcd karmada cert
func genCerts() error {
	notAfter := time.Now().Add(cert.Duration365d * 10).UTC()
	var etcdServerCertDNS = []string{
		"localhost",
	}

	for i := int32(0); i < options.EtcdReplicas; i++ {
		etcdServerCertDNS = append(etcdServerCertDNS, fmt.Sprintf("%s-%v.%s.%s.svc.cluster.local", etcdStatefulSetAndServiceName, i, etcdStatefulSetAndServiceName, options.Namespace))
	}
	etcdServerAltNames := certutil.AltNames{
		DNSNames: etcdServerCertDNS,
		IPs:      []net.IP{utils.StringToNetIP("127.0.0.1")},
	}
	etcdServerCertConfig := cert.NewCertConfig("karmada-etcd-server", []string{"karmada"}, etcdServerAltNames, &notAfter)

	etcdClientCertCfg := cert.NewCertConfig("karmada-etcd-client", []string{"karmada"}, certutil.AltNames{}, &notAfter)

	var karmadaDNS = []string{
		"localhost",
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		karmadaAPIServerDeploymentAndServiceName,
		webhookDeploymentAndServiceAccountAndServiceName,
		fmt.Sprintf("%s.%s.svc.cluster.local", karmadaAPIServerDeploymentAndServiceName, options.Namespace),
		fmt.Sprintf("%s.%s.svc.cluster.local", webhookDeploymentAndServiceAccountAndServiceName, options.Namespace),
		fmt.Sprintf("*.%s.svc.cluster.local", options.Namespace),
		fmt.Sprintf("*.%s.svc", options.Namespace),
	}
	if hostName, err := os.Hostname(); err != nil {
		klog.Warningln("Failed to get the current hostname.", err)
	} else {
		karmadaDNS = append(karmadaDNS, hostName)
	}

	karmadaIPs := utils.FlagsIP(options.KarmadaMasterIP)
	karmadaIPs = append(
		karmadaIPs,
		utils.StringToNetIP("127.0.0.1"),
		utils.StringToNetIP("10.254.0.1"),
	)
	karmadaIPs = append(karmadaIPs, utils.FlagsIP(options.ExternalIP)...)

	internetIP, err := utils.InternetIP()
	if err != nil {
		klog.Warningln("Failed to obtain internet IP. ", err)
	} else {
		karmadaIPs = append(karmadaIPs, internetIP)
	}

	karmadaAltNames := certutil.AltNames{
		DNSNames: karmadaDNS,
		IPs:      karmadaIPs,
	}
	karmadaCertCfg := cert.NewCertConfig("system:admin", []string{"system:masters"}, karmadaAltNames, &notAfter)

	if err = cert.GenCerts(options.DataPath, etcdServerCertConfig, etcdClientCertCfg, karmadaCertCfg); err != nil {
		return err
	}
	return nil
}

//prepareCRD download or unzip `crds.tar.gz` to `options.DataPath`
func prepareCRD() error {
	if strings.HasPrefix(options.CRDs, "http") {
		filename := options.DataPath + "/" + path.Base(options.CRDs)
		klog.Infoln("crds file name:", filename)
		if err := utils.DownloadFile(options.CRDs, filename); err != nil {
			return err
		}
		if err := utils.DeCompress(filename, options.DataPath); err != nil {
			return err
		}
		return nil
	}
	klog.Infoln("crds file name:", options.CRDs)
	return utils.DeCompress(options.CRDs, options.DataPath)
}

func (i *InstallOptions) initialization() error {
	i.CertAndKeyFileData = map[string][]byte{}

	caCert, err := utils.FileToBytes(options.DataPath, fmt.Sprintf("%s.crt", options.CaCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.crt' conversion failed. %v", options.CaCertAndKeyName, err)
	}
	i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.CaCertAndKeyName)] = caCert

	caKey, err := utils.FileToBytes(options.DataPath, fmt.Sprintf("%s.key", options.CaCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.key' conversion failed. %v", options.CaCertAndKeyName, err)
	}
	i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.CaCertAndKeyName)] = caKey

	etcdServerCert, err := utils.FileToBytes(options.DataPath, fmt.Sprintf("%s.crt", options.EtcdServerCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.crt' conversion failed. %v", options.EtcdServerCertAndKeyName, err)
	}
	i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.EtcdServerCertAndKeyName)] = etcdServerCert

	etcdServerKey, err := utils.FileToBytes(options.DataPath, fmt.Sprintf("%s.key", options.EtcdServerCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.key' conversion failed. %v", options.EtcdServerCertAndKeyName, err)
	}
	i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.EtcdServerCertAndKeyName)] = etcdServerKey

	etcdClientCert, err := utils.FileToBytes(options.DataPath, fmt.Sprintf("%s.crt", options.EtcdClientCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.crt' conversion failed. %v", options.EtcdClientCertAndKeyName, err)
	}
	i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.EtcdClientCertAndKeyName)] = etcdClientCert

	etcdClientKey, err := utils.FileToBytes(options.DataPath, fmt.Sprintf("%s.key", options.EtcdClientCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.key' conversion failed. %v", options.EtcdClientCertAndKeyName, err)
	}
	i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.EtcdClientCertAndKeyName)] = etcdClientKey

	karmadaCert, err := utils.FileToBytes(options.DataPath, fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.crt' conversion failed. %v", options.KarmadaCertAndKeyName, err)
	}
	i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName)] = karmadaCert

	karmadaKey, err := utils.FileToBytes(options.DataPath, fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName))
	if err != nil {
		return fmt.Errorf("'%s.key' conversion failed. %v", options.KarmadaCertAndKeyName, err)
	}
	i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)] = karmadaKey

	if !utils.PathIsExist(options.KubeConfig) {
		klog.Exitln("kubeconfig file does not exist.")
	}
	klog.Infof("kubeconfig file: %s", options.KubeConfig)

	i.Namespace = options.Namespace

	restConfig, err := utils.RestConfig(options.KubeConfig)
	if err != nil {
		return err
	}
	i.RestConfig = restConfig

	clientSet, err := utils.NewClientSet(restConfig)
	if err != nil {
		return err
	}
	i.KubeClientSet = clientSet

	i.MasterIP = utils.FlagsIP(options.KarmadaMasterIP)

	klog.Infof("master ip: %s", i.MasterIP)

	return nil
}

func (i *InstallOptions) createCertsSecrets() {
	//Create kubeconfig Secret
	karmadaServerURL := fmt.Sprintf("https://%s.%s.svc.cluster.local:%v", karmadaAPIServerDeploymentAndServiceName, i.Namespace, options.KarmadaMasterPort)
	config := utils.CreateWithCerts(karmadaServerURL, options.UserName, options.UserName, i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.CaCertAndKeyName)],
		i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)], i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName)])
	configBytes, err := clientcmd.Write(*config)
	if err != nil {
		klog.Exitln("failure while serializing admin kubeConfig", err)
	}

	kubeConfigSecret := i.SecretFromSpec(kubeConfigSecretAndMountName, corev1.SecretTypeOpaque, map[string]string{kubeConfigSecretAndMountName: string(configBytes)})
	if err = i.CreateSecret(kubeConfigSecret); err != nil {
		klog.Exitln(err)
	}
	// Create certs Secret
	etcdCert := map[string]string{
		fmt.Sprintf("%s.crt", options.CaCertAndKeyName):         string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.CaCertAndKeyName)]),
		fmt.Sprintf("%s.key", options.CaCertAndKeyName):         string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.CaCertAndKeyName)]),
		fmt.Sprintf("%s.crt", options.EtcdServerCertAndKeyName): string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.EtcdServerCertAndKeyName)]),
		fmt.Sprintf("%s.key", options.EtcdServerCertAndKeyName): string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.EtcdServerCertAndKeyName)]),
	}
	etcdSecret := i.SecretFromSpec(etcdCertName, corev1.SecretTypeOpaque, etcdCert)
	if err := i.CreateSecret(etcdSecret); err != nil {
		klog.Exitln(err)
	}

	karmadaCert := map[string]string{
		fmt.Sprintf("%s.crt", options.CaCertAndKeyName):         string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.CaCertAndKeyName)]),
		fmt.Sprintf("%s.key", options.CaCertAndKeyName):         string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.CaCertAndKeyName)]),
		fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName):    string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName)]),
		fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName):    string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)]),
		fmt.Sprintf("%s.crt", options.EtcdClientCertAndKeyName): string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.EtcdClientCertAndKeyName)]),
		fmt.Sprintf("%s.key", options.EtcdClientCertAndKeyName): string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.EtcdClientCertAndKeyName)]),
	}

	karmadaSecret := i.SecretFromSpec(karmadaCertsName, corev1.SecretTypeOpaque, karmadaCert)
	if err := i.CreateSecret(karmadaSecret); err != nil {
		klog.Exitln(err)
	}

	karmadaWebhookCert := map[string]string{
		"tls.crt": string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName)]),
		"tls.key": string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)]),
	}
	karmadaWebhookSecret := i.SecretFromSpec(webhookCertsName, corev1.SecretTypeOpaque, karmadaWebhookCert)
	if err := i.CreateSecret(karmadaWebhookSecret); err != nil {
		klog.Exitln(err)
	}
}

func (i *InstallOptions) initKarmadaAPIServer() {
	klog.Info("create karmada ApiServer Deployment")
	if err := i.CreateService(i.makeKarmadaAPIServerService()); err != nil {
		klog.Exitln(err)
	}

	if _, err := i.KubeClientSet.AppsV1().Deployments(i.Namespace).Create(context.TODO(), i.makeKarmadaAPIServerDeployment(), metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := WaitPodReady(i.KubeClientSet, i.Namespace, utils.MapToString(apiServerLabels), 120); err != nil {
		klog.Exitln(err)
	}
}

func (i *InstallOptions) initEtcd() {
	if err := i.CreateService(i.makeEtcdService(etcdStatefulSetAndServiceName)); err != nil {
		klog.Exitln(err)
	}
	klog.Info("create etcd StatefulSets")
	if _, err := i.KubeClientSet.AppsV1().StatefulSets(i.Namespace).Create(context.TODO(), i.makeETCDStatefulSet(), metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := WaitEtcdReplicasetInDesired(i.KubeClientSet, i.Namespace, utils.MapToString(etcdLabels), 30); err != nil {
		klog.Warning(err)
	}
	if err := WaitPodReady(i.KubeClientSet, i.Namespace, utils.MapToString(etcdLabels), 30); err != nil {
		klog.Warning(err)
	}
}

func (i *InstallOptions) initComponent() {
	// Create karmada-kube-controller-manager
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/kube-controller-manager.yaml
	klog.Info("create karmada kube controller manager Deployment")
	if err := i.CreateService(i.kubeControllerManagerService()); err != nil {
		klog.Exitln(err)
	}
	if _, err := i.KubeClientSet.AppsV1().Deployments(i.Namespace).Create(context.TODO(), i.makeKarmadaKubeControllerManagerDeployment(), metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := WaitPodReady(i.KubeClientSet, i.Namespace, utils.MapToString(kubeControllerManagerLabels), 30); err != nil {
		klog.Warning(err)
	}

	// Create karmada-scheduler
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-scheduler.yaml
	klog.Info("create karmada scheduler Deployment")
	if _, err := i.KubeClientSet.AppsV1().Deployments(i.Namespace).Create(context.TODO(), i.makeKarmadaSchedulerDeployment(), metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := WaitPodReady(i.KubeClientSet, i.Namespace, utils.MapToString(schedulerLabels), 30); err != nil {
		klog.Warning(err)
	}

	// Create karmada-controller-manager
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/controller-manager.yaml
	klog.Info("create karmada controller manager Deployment")
	if _, err := i.KubeClientSet.AppsV1().Deployments(i.Namespace).Create(context.TODO(), i.makeKarmadaControllerManagerDeployment(), metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := WaitPodReady(i.KubeClientSet, i.Namespace, utils.MapToString(controllerManagerLabels), 30); err != nil {
		klog.Warning(err)
	}

	// Create karmada-webhook
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-webhook.yaml
	klog.Info("create karmada webhook Deployment")
	if err := i.CreateService(i.karmadaWebhookService()); err != nil {
		klog.Exitln(err)
	}

	if _, err := i.KubeClientSet.AppsV1().Deployments(i.Namespace).Create(context.TODO(), i.makeKarmadaWebhookDeployment(), metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := WaitPodReady(i.KubeClientSet, i.Namespace, utils.MapToString(webhookLabels), 30); err != nil {
		klog.Warning(err)
	}
}

//Deploy karmada in kubernetes
func Deploy() {
	verify()
	//generate certificate
	if err := genCerts(); err != nil {
		klog.Exitln("certificate generation failed", err)
	}

	//prepare karmada CRD resources
	if err := prepareCRD(); err != nil {
		klog.Exitln("prepare karmada failed.", err)
	}

	i := &InstallOptions{}
	if err := i.initialization(); err != nil {
		klog.Exitln(err)
	}

	// Create karmada kubeconfig
	serverURL := fmt.Sprintf("https://%s:%v", i.MasterIP[0].String(), options.KarmadaMasterPort)
	if err := utils.WriteKubeConfigFromSpec(serverURL, options.UserName, options.ClusterName, options.DataPath, options.KarmadaKubeConfigName,
		i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.CaCertAndKeyName)], i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)],
		i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName)]); err != nil {
		klog.Exitln("Failed to create karmada kubeconfig file.", err)
	}
	klog.Info("Create karmada kubeconfig success.")

	//	create ns
	if err := i.CreateNamespace(); err != nil {
		klog.Exitln(err)
	}

	// Create sa
	if err := i.CreateServiceAccount(); err != nil {
		klog.Exitln(err)
	}

	// Create karmada-controller-manager ClusterRole and ClusterRoleBinding
	if err := i.CreateClusterRole(); err != nil {
		klog.Exitln(err)
	}

	// Create Secrets
	i.createCertsSecrets()

	// add node labels
	if err := i.AddNodeSelectorLabels(); err != nil {
		klog.Exitf("Node failed to add '%s' label. %v", NodeSelectorLabels, err)
	}

	// install etcd
	i.initEtcd()

	// install karmada-apiserver
	i.initKarmadaAPIServer()

	//Create CRDs in karmada
	caBase64 := base64.StdEncoding.EncodeToString(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.CaCertAndKeyName)])
	if err := karmada.InitKarmadaResources(caBase64); err != nil {
		klog.Exitln(err)
	}

	//install karmada Component
	i.initComponent()

	utils.GenExamples(options.DataPath)
}

func verify() {
	if options.KarmadaMasterIP == "" {
		klog.Exitln("error verifying flag, master is missing. See 'kubectl karmada init --help'.")
	}

	if options.EtcdStorageMode == "hostPath" && options.EtcdDataPath == "" {
		klog.Exitln("When etcd storage mode is hostPath, dataPath is not empty. See 'kubectl karmada init --help'.")
	}

	if options.EtcdStorageMode == "PVC" && options.StorageClassesName == "" {
		klog.Exitln("When etcd storage mode is PVC, storageClassesName is not empty. See 'kubectl karmada init --help'.")
	}
}
