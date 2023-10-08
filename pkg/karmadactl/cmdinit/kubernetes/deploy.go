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
	netutils "k8s.io/utils/net"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/cert"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/karmada"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	"github.com/karmada-io/karmada/pkg/version"
)

var (
	imageRepositories = map[string]string{
		"global": "registry.k8s.io",
		"cn":     "registry.cn-hangzhou.aliyuncs.com/google_containers",
	}

	certList = []string{
		options.CaCertAndKeyName,
		options.EtcdCaCertAndKeyName,
		options.EtcdServerCertAndKeyName,
		options.EtcdClientCertAndKeyName,
		options.KarmadaCertAndKeyName,
		options.ApiserverCertAndKeyName,
		options.FrontProxyCaCertAndKeyName,
		options.FrontProxyClientCertAndKeyName,
	}

	emptyByteSlice                 = make([]byte, 0)
	externalEtcdCertSpecialization = map[string]func(*CommandInitOption) ([]byte, []byte, error){
		options.EtcdCaCertAndKeyName: func(option *CommandInitOption) (cert, key []byte, err error) {
			cert, err = os.ReadFile(option.ExternalEtcdCACertPath)
			key = emptyByteSlice
			return
		},
		options.EtcdServerCertAndKeyName: func(_ *CommandInitOption) ([]byte, []byte, error) {
			return emptyByteSlice, emptyByteSlice, nil
		},
		options.EtcdClientCertAndKeyName: func(option *CommandInitOption) (cert, key []byte, err error) {
			if cert, err = os.ReadFile(option.ExternalEtcdClientCertPath); err != nil {
				return
			}
			key, err = os.ReadFile(option.ExternalEtcdClientKeyPath)
			return
		},
	}

	karmadaRelease string

	defaultEtcdImage = "etcd:3.5.9-0"

	// DefaultCrdURL Karmada crds resource
	DefaultCrdURL string
	// DefaultInitImage etcd init container image
	DefaultInitImage string
	// DefaultKarmadaSchedulerImage Karmada scheduler image
	DefaultKarmadaSchedulerImage string
	// DefaultKarmadaControllerManagerImage Karmada controller manager image
	DefaultKarmadaControllerManagerImage string
	// DefualtKarmadaWebhookImage Karmada webhook image
	DefualtKarmadaWebhookImage string
	// DefaultKarmadaAggregatedAPIServerImage Karmada aggregated apiserver image
	DefaultKarmadaAggregatedAPIServerImage string
)

const (
	etcdStorageModePVC      = "PVC"
	etcdStorageModeEmptyDir = "emptyDir"
	etcdStorageModeHostPath = "hostPath"
)

func init() {
	releaseVer, err := version.ParseGitVersion(version.Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", version.Get().String())
		releaseVer = &version.ReleaseVersion{} // initialize to avoid panic
	}
	karmadaRelease = releaseVer.ReleaseVersion()

	DefaultCrdURL = fmt.Sprintf("https://github.com/karmada-io/karmada/releases/download/%s/crds.tar.gz", releaseVer.ReleaseVersion())
	DefaultInitImage = "docker.io/alpine:3.15.1"
	DefaultKarmadaSchedulerImage = fmt.Sprintf("docker.io/karmada/karmada-scheduler:%s", releaseVer.ReleaseVersion())
	DefaultKarmadaControllerManagerImage = fmt.Sprintf("docker.io/karmada/karmada-controller-manager:%s", releaseVer.ReleaseVersion())
	DefualtKarmadaWebhookImage = fmt.Sprintf("docker.io/karmada/karmada-webhook:%s", releaseVer.ReleaseVersion())
	DefaultKarmadaAggregatedAPIServerImage = fmt.Sprintf("docker.io/karmada/karmada-aggregated-apiserver:%s", releaseVer.ReleaseVersion())
}

// CommandInitOption holds all flags options for init.
type CommandInitOption struct {
	ImageRegistry                      string
	KubeImageRegistry                  string
	KubeImageMirrorCountry             string
	KubeImageTag                       string
	EtcdImage                          string
	EtcdReplicas                       int32
	EtcdInitImage                      string
	EtcdStorageMode                    string
	EtcdHostDataPath                   string
	EtcdNodeSelectorLabels             string
	EtcdPersistentVolumeSize           string
	ExternalEtcdCACertPath             string
	ExternalEtcdClientCertPath         string
	ExternalEtcdClientKeyPath          string
	ExternalEtcdServers                string
	ExternalEtcdKeyPrefix              string
	KarmadaAPIServerImage              string
	KarmadaAPIServerReplicas           int32
	KarmadaAPIServerAdvertiseAddress   string
	KarmadaAPIServerNodePort           int32
	KarmadaSchedulerImage              string
	KarmadaSchedulerReplicas           int32
	KubeControllerManagerImage         string
	KubeControllerManagerReplicas      int32
	KarmadaControllerManagerImage      string
	KarmadaControllerManagerReplicas   int32
	KarmadaWebhookImage                string
	KarmadaWebhookReplicas             int32
	KarmadaAggregatedAPIServerImage    string
	KarmadaAggregatedAPIServerReplicas int32
	Namespace                          string
	KubeConfig                         string
	Context                            string
	StorageClassesName                 string
	KarmadaDataPath                    string
	KarmadaPkiPath                     string
	CRDs                               string
	ExternalIP                         string
	ExternalDNS                        string
	PullSecrets                        []string
	CertValidity                       time.Duration
	KubeClientSet                      kubernetes.Interface
	CertAndKeyFileData                 map[string][]byte
	RestConfig                         *rest.Config
	KarmadaAPIServerIP                 []net.IP
	HostClusterDomain                  string
	WaitComponentReadyTimeout          int
}

func (i *CommandInitOption) validateLocalEtcd(parentCommand string) error {
	if i.EtcdStorageMode == etcdStorageModeHostPath && i.EtcdHostDataPath == "" {
		return fmt.Errorf("when etcd storage mode is hostPath, dataPath is not empty. See '%s init --help'", parentCommand)
	}

	if i.EtcdStorageMode == etcdStorageModeHostPath && i.EtcdNodeSelectorLabels != "" && utils.StringToMap(i.EtcdNodeSelectorLabels) == nil {
		return fmt.Errorf("the label does not seem to be 'key=value'")
	}

	if i.EtcdStorageMode == etcdStorageModeHostPath && i.EtcdReplicas != 1 {
		return fmt.Errorf("for data security,when etcd storage mode is hostPath,etcd-replicas can only be 1")
	}

	if i.EtcdStorageMode == etcdStorageModePVC && i.StorageClassesName == "" {
		return fmt.Errorf("when etcd storage mode is PVC, storageClassesName is not empty. See '%s init --help'", parentCommand)
	}

	if i.WaitComponentReadyTimeout < 0 {
		return fmt.Errorf("wait-component-ready-timeout must be greater than or equal to 0")
	}

	supportedStorageMode := SupportedStorageMode()
	if i.EtcdStorageMode != "" {
		for _, mode := range supportedStorageMode {
			if i.EtcdStorageMode == mode {
				return nil
			}
		}
		return fmt.Errorf("unsupported etcd-storage-mode %s. See '%s init --help'", i.EtcdStorageMode, parentCommand)
	}
	return nil
}

func (i *CommandInitOption) validateExternalEtcd(_ string) error {
	if (i.ExternalEtcdClientCertPath == "" && i.ExternalEtcdClientKeyPath != "") ||
		(i.ExternalEtcdClientCertPath != "" && i.ExternalEtcdClientKeyPath == "") {
		return fmt.Errorf("etcd client cert and key should be specified both or none")
	}
	return nil
}

func (i *CommandInitOption) isExternalEtcdProvided() bool {
	return i.ExternalEtcdServers != ""
}

// Validate Check that there are enough flags to run the command.
//
//nolint:gocyclo
func (i *CommandInitOption) Validate(parentCommand string) error {
	if i.KarmadaAPIServerAdvertiseAddress != "" {
		if netutils.ParseIPSloppy(i.KarmadaAPIServerAdvertiseAddress) == nil {
			return fmt.Errorf("karmada apiserver advertise address is not valid")
		}
	}

	if i.isExternalEtcdProvided() {
		return i.validateExternalEtcd(parentCommand)
	} else {
		return i.validateLocalEtcd(parentCommand)
	}
}

// Complete Initialize k8s client
func (i *CommandInitOption) Complete() error {
	restConfig, err := apiclient.RestConfig(i.Context, i.KubeConfig)
	if err != nil {
		return err
	}
	i.RestConfig = restConfig

	klog.Infof("kubeconfig file: %s, kubernetes: %s", i.KubeConfig, restConfig.Host)
	clientSet, err := apiclient.NewClientSet(restConfig)
	if err != nil {
		return err
	}
	i.KubeClientSet = clientSet

	if i.isNodePortExist() {
		return fmt.Errorf("nodePort of karmada apiserver %v already exist", i.KarmadaAPIServerNodePort)
	}

	if !i.isExternalEtcdProvided() && i.EtcdStorageMode == "hostPath" && i.EtcdNodeSelectorLabels == "" {
		if err := i.AddNodeSelectorLabels(); err != nil {
			return err
		}
	}

	if err := i.getKarmadaAPIServerIP(); err != nil {
		return err
	}
	klog.Infof("karmada apiserver ip: %s", i.KarmadaAPIServerIP)

	if !i.isExternalEtcdProvided() && i.EtcdStorageMode == "hostPath" && i.EtcdNodeSelectorLabels != "" {
		if !i.isNodeExist(i.EtcdNodeSelectorLabels) {
			return fmt.Errorf("no node found by label %s", i.EtcdNodeSelectorLabels)
		}
	}

	return initializeDirectory(i.KarmadaDataPath)
}

// initializeDirectory initializes a directory and makes sure it's empty.
func initializeDirectory(path string) error {
	if utils.IsExist(path) {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(path, os.FileMode(0700)); err != nil {
		return fmt.Errorf("failed to create directory: %s, error: %v", path, err)
	}

	return nil
}

// genCerts create ca etcd karmada cert
func (i *CommandInitOption) genCerts() error {
	notAfter := time.Now().Add(i.CertValidity).UTC()

	var etcdServerCertConfig, etcdClientCertCfg *cert.CertsConfig
	if !i.isExternalEtcdProvided() {
		etcdServerCertDNS := []string{
			"localhost",
		}
		for number := int32(0); number < i.EtcdReplicas; number++ {
			etcdServerCertDNS = append(etcdServerCertDNS, fmt.Sprintf("%s-%v.%s.%s.svc.%s",
				etcdStatefulSetAndServiceName, number, etcdStatefulSetAndServiceName, i.Namespace, i.HostClusterDomain))
		}
		etcdServerAltNames := certutil.AltNames{
			DNSNames: etcdServerCertDNS,
			IPs:      []net.IP{utils.StringToNetIP("127.0.0.1")},
		}
		etcdServerCertConfig = cert.NewCertConfig("karmada-etcd-server", []string{}, etcdServerAltNames, &notAfter)
		etcdClientCertCfg = cert.NewCertConfig("karmada-etcd-client", []string{}, certutil.AltNames{}, &notAfter)
	}

	karmadaDNS := []string{
		"localhost",
		"kubernetes",
		"kubernetes.default",
		"kubernetes.default.svc",
		karmadaAPIServerDeploymentAndServiceName,
		webhookDeploymentAndServiceAccountAndServiceName,
		karmadaAggregatedAPIServerDeploymentAndServiceName,
		fmt.Sprintf("%s.%s.svc.%s", karmadaAPIServerDeploymentAndServiceName, i.Namespace, i.HostClusterDomain),
		fmt.Sprintf("%s.%s.svc.%s", webhookDeploymentAndServiceAccountAndServiceName, i.Namespace, i.HostClusterDomain),
		fmt.Sprintf("%s.%s.svc", webhookDeploymentAndServiceAccountAndServiceName, i.Namespace),
		fmt.Sprintf("%s.%s.svc.%s", karmadaAggregatedAPIServerDeploymentAndServiceName, i.Namespace, i.HostClusterDomain),
		fmt.Sprintf("*.%s.svc.%s", i.Namespace, i.HostClusterDomain),
		fmt.Sprintf("*.%s.svc", i.Namespace),
	}
	karmadaDNS = append(karmadaDNS, utils.FlagsDNS(i.ExternalDNS)...)

	karmadaIPs := utils.FlagsIP(i.ExternalIP)
	karmadaIPs = append(
		karmadaIPs,
		utils.StringToNetIP("127.0.0.1"),
		utils.StringToNetIP("10.254.0.1"),
	)
	karmadaIPs = append(karmadaIPs, i.KarmadaAPIServerIP...)

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

	apiserverCertCfg := cert.NewCertConfig("karmada-apiserver", []string{""}, karmadaAltNames, &notAfter)

	frontProxyClientCertCfg := cert.NewCertConfig("front-proxy-client", []string{}, certutil.AltNames{}, &notAfter)
	if err = cert.GenCerts(i.KarmadaPkiPath, etcdServerCertConfig, etcdClientCertCfg, karmadaCertCfg, apiserverCertCfg, frontProxyClientCertCfg); err != nil {
		return err
	}
	return nil
}

// prepareCRD download or unzip `crds.tar.gz` to `options.DataPath`
func (i *CommandInitOption) prepareCRD() error {
	if strings.HasPrefix(i.CRDs, "http") {
		filename := i.KarmadaDataPath + "/" + path.Base(i.CRDs)
		klog.Infof("download crds file:%s", i.CRDs)
		if err := utils.DownloadFile(i.CRDs, filename); err != nil {
			return err
		}
		if err := utils.DeCompress(filename, i.KarmadaDataPath); err != nil {
			return err
		}
		return nil
	}
	klog.Infoln("local crds file name:", i.CRDs)
	return utils.DeCompress(i.CRDs, i.KarmadaDataPath)
}

func (i *CommandInitOption) createCertsSecrets() error {
	// Create kubeconfig Secret
	karmadaServerURL := fmt.Sprintf("https://%s.%s.svc.%s:%v", karmadaAPIServerDeploymentAndServiceName, i.Namespace, i.HostClusterDomain, karmadaAPIServerContainerPort)
	config := utils.CreateWithCerts(karmadaServerURL, options.UserName, options.UserName, i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.CaCertAndKeyName)],
		i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)], i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName)])
	configBytes, err := clientcmd.Write(*config)
	if err != nil {
		return fmt.Errorf("failure while serializing admin kubeConfig. %v", err)
	}

	kubeConfigSecret := i.SecretFromSpec(KubeConfigSecretAndMountName, corev1.SecretTypeOpaque, map[string]string{KubeConfigSecretAndMountName: string(configBytes)})
	if err = util.CreateOrUpdateSecret(i.KubeClientSet, kubeConfigSecret); err != nil {
		return err
	}
	// Create certs Secret
	etcdCert := map[string]string{
		fmt.Sprintf("%s.crt", options.EtcdCaCertAndKeyName):     string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.EtcdCaCertAndKeyName)]),
		fmt.Sprintf("%s.key", options.EtcdCaCertAndKeyName):     string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.EtcdCaCertAndKeyName)]),
		fmt.Sprintf("%s.crt", options.EtcdServerCertAndKeyName): string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.EtcdServerCertAndKeyName)]),
		fmt.Sprintf("%s.key", options.EtcdServerCertAndKeyName): string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.EtcdServerCertAndKeyName)]),
	}
	etcdSecret := i.SecretFromSpec(etcdCertName, corev1.SecretTypeOpaque, etcdCert)
	if err := util.CreateOrUpdateSecret(i.KubeClientSet, etcdSecret); err != nil {
		return err
	}

	karmadaCert := map[string]string{}
	for _, v := range certList {
		karmadaCert[fmt.Sprintf("%s.crt", v)] = string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", v)])
		karmadaCert[fmt.Sprintf("%s.key", v)] = string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", v)])
	}
	karmadaSecret := i.SecretFromSpec(karmadaCertsName, corev1.SecretTypeOpaque, karmadaCert)
	if err := util.CreateOrUpdateSecret(i.KubeClientSet, karmadaSecret); err != nil {
		return err
	}

	karmadaWebhookCert := map[string]string{
		"tls.crt": string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName)]),
		"tls.key": string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)]),
	}
	karmadaWebhookSecret := i.SecretFromSpec(webhookCertsName, corev1.SecretTypeOpaque, karmadaWebhookCert)
	if err := util.CreateOrUpdateSecret(i.KubeClientSet, karmadaWebhookSecret); err != nil {
		return err
	}

	return nil
}

func (i *CommandInitOption) initKarmadaAPIServer() error {
	if !i.isExternalEtcdProvided() {
		if err := util.CreateOrUpdateService(i.KubeClientSet, i.makeEtcdService(etcdStatefulSetAndServiceName)); err != nil {
			return err
		}
		klog.Info("Create etcd StatefulSets")
		etcdStatefulSet := i.makeETCDStatefulSet()
		if _, err := i.KubeClientSet.AppsV1().StatefulSets(i.Namespace).Create(context.TODO(), etcdStatefulSet, metav1.CreateOptions{}); err != nil {
			klog.Warning(err)
		}
		if err := util.WaitForStatefulSetRollout(i.KubeClientSet, etcdStatefulSet, i.WaitComponentReadyTimeout); err != nil {
			klog.Warning(err)
		}
	}
	klog.Info("Create karmada ApiServer Deployment")
	if err := util.CreateOrUpdateService(i.KubeClientSet, i.makeKarmadaAPIServerService()); err != nil {
		return err
	}

	karmadaAPIServerDeployment := i.makeKarmadaAPIServerDeployment()
	if _, err := i.KubeClientSet.AppsV1().Deployments(i.Namespace).Create(context.TODO(), karmadaAPIServerDeployment, metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := util.WaitForDeploymentRollout(i.KubeClientSet, karmadaAPIServerDeployment, i.WaitComponentReadyTimeout); err != nil {
		return err
	}

	// Create karmada-aggregated-apiserver
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-aggregated-apiserver.yaml
	klog.Info("Create karmada aggregated apiserver Deployment")
	if err := util.CreateOrUpdateService(i.KubeClientSet, i.karmadaAggregatedAPIServerService()); err != nil {
		klog.Exitln(err)
	}
	karmadaAggregatedAPIServerDeployment := i.makeKarmadaAggregatedAPIServerDeployment()
	if _, err := i.KubeClientSet.AppsV1().Deployments(i.Namespace).Create(context.TODO(), karmadaAggregatedAPIServerDeployment, metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := util.WaitForDeploymentRollout(i.KubeClientSet, karmadaAggregatedAPIServerDeployment, i.WaitComponentReadyTimeout); err != nil {
		klog.Warning(err)
	}
	return nil
}

func (i *CommandInitOption) initKarmadaComponent() error {
	deploymentClient := i.KubeClientSet.AppsV1().Deployments(i.Namespace)
	// Create karmada-kube-controller-manager
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/kube-controller-manager.yaml
	klog.Info("Create karmada kube controller manager Deployment")
	if err := util.CreateOrUpdateService(i.KubeClientSet, i.kubeControllerManagerService()); err != nil {
		klog.Exitln(err)
	}
	karmadaKubeControllerManagerDeployment := i.makeKarmadaKubeControllerManagerDeployment()
	if _, err := deploymentClient.Create(context.TODO(), karmadaKubeControllerManagerDeployment, metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := util.WaitForDeploymentRollout(i.KubeClientSet, karmadaKubeControllerManagerDeployment, i.WaitComponentReadyTimeout); err != nil {
		klog.Warning(err)
	}

	// Create karmada-scheduler
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-scheduler.yaml
	klog.Info("Create karmada scheduler Deployment")
	karmadaSchedulerDeployment := i.makeKarmadaSchedulerDeployment()
	if _, err := deploymentClient.Create(context.TODO(), karmadaSchedulerDeployment, metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := util.WaitForDeploymentRollout(i.KubeClientSet, karmadaSchedulerDeployment, i.WaitComponentReadyTimeout); err != nil {
		klog.Warning(err)
	}

	// Create karmada-controller-manager
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-controller-manager.yaml
	klog.Info("Create karmada controller manager Deployment")
	karmadaControllerManagerDeployment := i.makeKarmadaControllerManagerDeployment()
	if _, err := deploymentClient.Create(context.TODO(), karmadaControllerManagerDeployment, metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := util.WaitForDeploymentRollout(i.KubeClientSet, karmadaControllerManagerDeployment, i.WaitComponentReadyTimeout); err != nil {
		klog.Warning(err)
	}

	// Create karmada-webhook
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-webhook.yaml
	klog.Info("Create karmada webhook Deployment")
	if err := util.CreateOrUpdateService(i.KubeClientSet, i.karmadaWebhookService()); err != nil {
		klog.Exitln(err)
	}
	karmadaWebhookDeployment := i.makeKarmadaWebhookDeployment()
	if _, err := deploymentClient.Create(context.TODO(), karmadaWebhookDeployment, metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	if err := util.WaitForDeploymentRollout(i.KubeClientSet, karmadaWebhookDeployment, i.WaitComponentReadyTimeout); err != nil {
		klog.Warning(err)
	}
	return nil
}

func (i *CommandInitOption) readExternalEtcdCert(name string) (isExternalEtcdCert bool, err error) {
	if !i.isExternalEtcdProvided() {
		return
	}
	var getCertAndKey func(*CommandInitOption) ([]byte, []byte, error)
	if getCertAndKey, isExternalEtcdCert = externalEtcdCertSpecialization[name]; isExternalEtcdCert {
		var certs, key []byte
		if certs, key, err = getCertAndKey(i); err != nil {
			return
		}
		i.CertAndKeyFileData[fmt.Sprintf("%s.crt", name)] = certs
		i.CertAndKeyFileData[fmt.Sprintf("%s.key", name)] = key
	}
	return
}

// RunInit Deploy karmada in kubernetes
func (i *CommandInitOption) RunInit(parentCommand string) error {
	// generate certificate
	if err := i.genCerts(); err != nil {
		return fmt.Errorf("certificate generation failed.%v", err)
	}

	i.CertAndKeyFileData = map[string][]byte{}

	for _, v := range certList {
		if isExternalEtcdCert, err := i.readExternalEtcdCert(v); err != nil {
			return fmt.Errorf("read external etcd certificate failed, %s. %v", v, err)
		} else if isExternalEtcdCert {
			continue
		}
		certs, err := utils.FileToBytes(i.KarmadaPkiPath, fmt.Sprintf("%s.crt", v))
		if err != nil {
			return fmt.Errorf("'%s.crt' conversion failed. %v", v, err)
		}
		i.CertAndKeyFileData[fmt.Sprintf("%s.crt", v)] = certs

		key, err := utils.FileToBytes(i.KarmadaPkiPath, fmt.Sprintf("%s.key", v))
		if err != nil {
			return fmt.Errorf("'%s.key' conversion failed. %v", v, err)
		}
		i.CertAndKeyFileData[fmt.Sprintf("%s.key", v)] = key
	}

	// prepare karmada CRD resources
	if err := i.prepareCRD(); err != nil {
		return fmt.Errorf("prepare karmada failed.%v", err)
	}

	// Create karmada kubeconfig
	err := i.createKarmadaConfig()
	if err != nil {
		return fmt.Errorf("create karmada kubeconfig failed.%v", err)
	}

	// Create ns
	if err := util.CreateOrUpdateNamespace(i.KubeClientSet, util.NewNamespace(i.Namespace)); err != nil {
		return fmt.Errorf("create namespace %s failed: %v", i.Namespace, err)
	}

	// Create Secrets
	if err := i.createCertsSecrets(); err != nil {
		return err
	}

	// install karmada-apiserver
	if err := i.initKarmadaAPIServer(); err != nil {
		return err
	}

	// Create CRDs in karmada
	caBase64 := base64.StdEncoding.EncodeToString(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.CaCertAndKeyName)])
	if err := karmada.InitKarmadaResources(i.KarmadaDataPath, caBase64, i.Namespace); err != nil {
		return err
	}

	// Create bootstrap token in karmada
	registerCommand, err := karmada.InitKarmadaBootstrapToken(i.KarmadaDataPath)
	if err != nil {
		return err
	}

	// install karmada Component
	if err := i.initKarmadaComponent(); err != nil {
		return err
	}

	utils.GenExamples(i.KarmadaDataPath, parentCommand, registerCommand)
	return nil
}

func (i *CommandInitOption) createKarmadaConfig() error {
	serverIP := i.KarmadaAPIServerIP[0].String()
	serverURL, err := generateServerURL(serverIP, i.KarmadaAPIServerNodePort)
	if err != nil {
		return err
	}
	if err := utils.WriteKubeConfigFromSpec(serverURL, options.UserName, options.ClusterName, i.KarmadaDataPath, options.KarmadaKubeConfigName,
		i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.CaCertAndKeyName)], i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)],
		i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName)]); err != nil {
		return fmt.Errorf("failed to create karmada kubeconfig file. %v", err)
	}
	klog.Info("Create karmada kubeconfig success.")
	return err
}

// get kube components registry
func (i *CommandInitOption) kubeRegistry() string {
	registry := i.KubeImageRegistry
	mirrorCountry := strings.ToLower(i.KubeImageMirrorCountry)

	if registry != "" {
		return registry
	}

	if mirrorCountry != "" {
		value, ok := imageRepositories[mirrorCountry]
		if ok {
			return value
		}
	}

	if i.ImageRegistry != "" {
		return i.ImageRegistry
	}
	return imageRepositories["global"]
}

// get kube-apiserver image
func (i *CommandInitOption) kubeAPIServerImage() string {
	if i.KarmadaAPIServerImage != "" {
		return i.KarmadaAPIServerImage
	}

	return i.kubeRegistry() + "/kube-apiserver:" + i.KubeImageTag
}

// get kube-controller-manager image
func (i *CommandInitOption) kubeControllerManagerImage() string {
	if i.KubeControllerManagerImage != "" {
		return i.KubeControllerManagerImage
	}

	return i.kubeRegistry() + "/kube-controller-manager:" + i.KubeImageTag
}

// get etcd-init image
func (i *CommandInitOption) etcdInitImage() string {
	if i.ImageRegistry != "" && i.EtcdInitImage == DefaultInitImage {
		return i.ImageRegistry + "/alpine:3.15.1"
	}
	return i.EtcdInitImage
}

// get etcd image
func (i *CommandInitOption) etcdImage() string {
	if i.EtcdImage != "" {
		return i.EtcdImage
	}
	return i.kubeRegistry() + "/" + defaultEtcdImage
}

// get karmada-scheduler image
func (i *CommandInitOption) karmadaSchedulerImage() string {
	if i.ImageRegistry != "" && i.KarmadaSchedulerImage == DefaultKarmadaSchedulerImage {
		return i.ImageRegistry + "/karmada-scheduler:" + karmadaRelease
	}
	return i.KarmadaSchedulerImage
}

// get karmada-controller-manager
func (i *CommandInitOption) karmadaControllerManagerImage() string {
	if i.ImageRegistry != "" && i.KarmadaControllerManagerImage == DefaultKarmadaControllerManagerImage {
		return i.ImageRegistry + "/karmada-controller-manager:" + karmadaRelease
	}
	return i.KarmadaControllerManagerImage
}

// get karmada-webhook image
func (i *CommandInitOption) karmadaWebhookImage() string {
	if i.ImageRegistry != "" && i.KarmadaWebhookImage == DefualtKarmadaWebhookImage {
		return i.ImageRegistry + "/karmada-webhook:" + karmadaRelease
	}
	return i.KarmadaWebhookImage
}

// get karmada-aggregated-apiserver image
func (i *CommandInitOption) karmadaAggregatedAPIServerImage() string {
	if i.ImageRegistry != "" && i.KarmadaAggregatedAPIServerImage == DefaultKarmadaAggregatedAPIServerImage {
		return i.ImageRegistry + "/karmada-aggregated-apiserver:" + karmadaRelease
	}
	return i.KarmadaAggregatedAPIServerImage
}

// get image pull secret
func (i *CommandInitOption) getImagePullSecrets() []corev1.LocalObjectReference {
	var imagePullSecrets []corev1.LocalObjectReference
	for _, val := range i.PullSecrets {
		secret := corev1.LocalObjectReference{
			Name: val,
		}
		imagePullSecrets = append(imagePullSecrets, secret)
	}
	return imagePullSecrets
}

func generateServerURL(serverIP string, nodePort int32) (string, error) {
	_, ipType, err := utils.ParseIP(serverIP)
	if err != nil {
		return "", err
	}
	if ipType == 4 {
		return fmt.Sprintf("https://%s:%v", serverIP, nodePort), nil
	}
	return fmt.Sprintf("https://[%s]:%v", serverIP, nodePort), nil
}

// SupportedStorageMode Return install etcd supported storage mode
func SupportedStorageMode() []string {
	return []string{etcdStorageModeEmptyDir, etcdStorageModeHostPath, etcdStorageModePVC}
}
