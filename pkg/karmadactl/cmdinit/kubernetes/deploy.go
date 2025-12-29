/*
Copyright 2021 The Karmada Authors.

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

package kubernetes

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
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

	certconst "github.com/karmada-io/karmada/pkg/cert"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/cert"
	initConfig "github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/config"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/karmada"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
	globaloptions "github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/util/validation"
	"github.com/karmada-io/karmada/pkg/version"
)

var (
	imageRepositories = map[string]string{
		"global": "registry.k8s.io",
		"cn":     "registry.cn-hangzhou.aliyuncs.com/google_containers",
	}

	// Legacy minimal certificate set (old layout)
	certListLegacy = []string{
		globaloptions.CaCertAndKeyName,
		options.EtcdCaCertAndKeyName,
		options.EtcdServerCertAndKeyName,
		options.EtcdClientCertAndKeyName,
		options.KarmadaCertAndKeyName,
		options.ApiserverCertAndKeyName,
		options.FrontProxyCaCertAndKeyName,
		options.FrontProxyClientCertAndKeyName,
	}

	// Full certificate set for split layout (independent CN per component)
	certListSplit = []string{
		// CAs
		globaloptions.CaCertAndKeyName,
		options.EtcdCaCertAndKeyName,
		options.FrontProxyCaCertAndKeyName,

		// Servers
		options.KarmadaAPIServerCertAndKeyName,
		options.KarmadaAggregatedAPIServerCertAndKeyName,
		options.KarmadaWebhookCertAndKeyName,
		options.KarmadaSearchCertAndKeyName,
		options.KarmadaMetricsAdapterCertAndKeyName,
		options.KarmadaSchedulerEstimatorCertAndKeyName,
		options.EtcdServerCertAndKeyName,

		// Clients
		options.KarmadaAPIServerClientCertAndKeyName,
		options.KarmadaAggregatedAPIServerClientCertAndKeyName,
		options.KarmadaWebhookClientCertAndKeyName,
		options.KarmadaSearchClientCertAndKeyName,
		options.KarmadaMetricsAdapterClientCertAndKeyName,
		options.KarmadaSchedulerEstimatorClientCertAndKeyName,
		options.KarmadaControllerManagerClientCertAndKeyName,
		options.KubeControllerManagerClientCertAndKeyName,
		options.KarmadaSchedulerClientCertAndKeyName,
		options.KarmadaDeschedulerClientCertAndKeyName,

		// ETCD clients
		options.KarmadaAPIServerEtcdClientCertAndKeyName,
		options.KarmadaAggregatedAPIServerEtcdClientCertAndKeyName,
		options.KarmadaSearchEtcdClientCertAndKeyName,
		options.EtcdClientCertAndKeyName,

		// GRPC clients
		options.KarmadaSchedulerGrpcCertAndKeyName,
		options.KarmadaDeschedulerGrpcCertAndKeyName,

		// front proxy
		options.FrontProxyClientCertAndKeyName,

		// Admin kubeconfig client
		options.KarmadaCertAndKeyName,
	}

	karmadaConfigList = []string{
		util.KarmadaConfigName(names.KarmadaAggregatedAPIServerComponentName),
		util.KarmadaConfigName(names.KarmadaControllerManagerComponentName),
		util.KarmadaConfigName(names.KubeControllerManagerComponentName),
		util.KarmadaConfigName(names.KarmadaSchedulerComponentName),
		util.KarmadaConfigName(names.KarmadaDeschedulerComponentName),
		util.KarmadaConfigName(names.KarmadaMetricsAdapterComponentName),
		util.KarmadaConfigName(names.KarmadaSearchComponentName),
		util.KarmadaConfigName(names.KarmadaWebhookComponentName),
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
		// map split-layout etcd client variants to external etcd client credentials
		options.KarmadaAPIServerEtcdClientCertAndKeyName: func(option *CommandInitOption) (cert, key []byte, err error) {
			if cert, err = os.ReadFile(option.ExternalEtcdClientCertPath); err != nil {
				return
			}
			key, err = os.ReadFile(option.ExternalEtcdClientKeyPath)
			return
		},
		options.KarmadaAggregatedAPIServerEtcdClientCertAndKeyName: func(option *CommandInitOption) (cert, key []byte, err error) {
			if cert, err = os.ReadFile(option.ExternalEtcdClientCertPath); err != nil {
				return
			}
			key, err = os.ReadFile(option.ExternalEtcdClientKeyPath)
			return
		},
		options.KarmadaSearchEtcdClientCertAndKeyName: func(option *CommandInitOption) (cert, key []byte, err error) {
			if cert, err = os.ReadFile(option.ExternalEtcdClientCertPath); err != nil {
				return
			}
			key, err = os.ReadFile(option.ExternalEtcdClientKeyPath)
			return
		},
	}

	karmadaRelease string

	defaultEtcdImage = "etcd:3.6.0-0"

	// DefaultCrdURL Karmada crds resource
	DefaultCrdURL string
	// DefaultKarmadaSchedulerImage Karmada scheduler image
	DefaultKarmadaSchedulerImage string
	// DefaultKarmadaControllerManagerImage Karmada controller manager image
	DefaultKarmadaControllerManagerImage string
	// DefaultKarmadaWebhookImage Karmada webhook image
	DefaultKarmadaWebhookImage string
	// DefaultKarmadaAggregatedAPIServerImage Karmada aggregated apiserver image
	DefaultKarmadaAggregatedAPIServerImage string
)

const (
	etcdStorageModePVC      = "PVC"
	etcdStorageModeEmptyDir = "emptyDir"
	etcdStorageModeHostPath = "hostPath"

	// secret layout values
	secretLayoutLegacy = "legacy"
	secretLayoutSplit  = "split"
)

func init() {
	releaseVer, err := version.ParseGitVersion(version.Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", version.Get().String())
		releaseVer = &version.ReleaseVersion{} // initialize to avoid panic
	}
	karmadaRelease = releaseVer.ReleaseVersion()

	DefaultCrdURL = fmt.Sprintf("https://github.com/karmada-io/karmada/releases/download/%s/crds.tar.gz", releaseVer.ReleaseVersion())
	DefaultKarmadaSchedulerImage = fmt.Sprintf("docker.io/karmada/karmada-scheduler:%s", releaseVer.ReleaseVersion())
	DefaultKarmadaControllerManagerImage = fmt.Sprintf("docker.io/karmada/karmada-controller-manager:%s", releaseVer.ReleaseVersion())
	DefaultKarmadaWebhookImage = fmt.Sprintf("docker.io/karmada/karmada-webhook:%s", releaseVer.ReleaseVersion())
	DefaultKarmadaAggregatedAPIServerImage = fmt.Sprintf("docker.io/karmada/karmada-aggregated-apiserver:%s", releaseVer.ReleaseVersion())
}

// CommandInitOption holds all flags options for init.
type CommandInitOption struct {
	ImageRegistry          string
	ImagePullPolicy        string
	KubeImageRegistry      string
	KubeImageMirrorCountry string
	KubeImageTag           string

	// internal etcd
	EtcdImage                 string
	EtcdReplicas              int32
	EtcdStorageMode           string
	EtcdHostDataPath          string
	EtcdNodeSelectorLabels    string
	EtcdNodeSelectorLabelsMap map[string]string
	EtcdPersistentVolumeSize  string
	EtcdPriorityClass         string
	EtcdExtraArgs             []string
	EtcdContainerCmd          []string // The command for containers in a Pod.

	// external etcd
	ExternalEtcdCACertPath     string
	ExternalEtcdClientCertPath string
	ExternalEtcdClientKeyPath  string
	ExternalEtcdServers        string
	ExternalEtcdKeyPrefix      string

	// karmada-apiserver
	KarmadaAPIServerImage            string
	KarmadaAPIServerReplicas         int32
	KarmadaAPIServerAdvertiseAddress string
	KarmadaAPIServerNodePort         int32
	KarmadaAPIServerIP               []net.IP
	KarmadaAPIServerPriorityClass    string
	KarmadaAPIServerExtraArgs        []string
	KarmadaAPIServerContainerCmd     []string

	// karmada-scheduler
	KarmadaSchedulerImage         string
	KarmadaSchedulerReplicas      int32
	KarmadaSchedulerPriorityClass string
	KarmadaSchedulerExtraArgs     []string
	KarmadaSchedulerContainerCmd  []string

	// kube-controller-manager
	KubeControllerManagerImage         string
	KubeControllerManagerReplicas      int32
	KubeControllerManagerPriorityClass string
	KubeControllerManagerExtraArgs     []string
	KubeControllerManagerContainerCmd  []string

	// karmada-controller-manager
	KarmadaControllerManagerImage         string
	KarmadaControllerManagerReplicas      int32
	KarmadaControllerManagerPriorityClass string
	KarmadaControllerManagerExtraArgs     []string
	KarmadaControllerManagerContainerCmd  []string

	// karmada-webhook
	KarmadaWebhookImage         string
	KarmadaWebhookReplicas      int32
	KarmadaWebhookPriorityClass string
	KarmadaWebhookExtraArgs     []string
	KarmadaWebhookContainerCmd  []string

	// karamda-aggregated-apiserver
	KarmadaAggregatedAPIServerImage         string
	KarmadaAggregatedAPIServerReplicas      int32
	KarmadaAggregatedAPIServerPriorityClass string
	KarmadaAggregatedAPIServerExtraArgs     []string
	KarmadaAggregatedAPIServerContainerCmd  []string

	Namespace          string
	KubeConfig         string
	Context            string
	StorageClassesName string
	KarmadaDataPath    string
	KarmadaPkiPath     string
	CRDs               string
	ExternalIP         string
	ExternalDNS        string
	PullSecrets        []string
	CertValidity       time.Duration
	KubeClientSet      kubernetes.Interface
	CertAndKeyFileData map[string][]byte
	RestConfig         *rest.Config

	HostClusterDomain         string
	WaitComponentReadyTimeout int
	CaCertFile                string
	CaKeyFile                 string
	KarmadaInitFilePath       string

	// SecretLayout controls how cert secrets are generated and mounted.
	// One of: "legacy" (aggregate secret) or "split" (per-component TLS secrets)
	SecretLayout string
}

func (i *CommandInitOption) validateLocalEtcd(parentCommand string) error {
	if i.EtcdStorageMode == etcdStorageModeHostPath && i.EtcdHostDataPath == "" {
		return fmt.Errorf("when etcd storage mode is hostPath, dataPath is not empty. See '%s init --help'", parentCommand)
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

// validateCommandLineArgs The parameters that are successfully validated will be reassigned to the corresponding xxxExtraArgs.
func (i *CommandInitOption) validateCommandLineArgs() error {
	type validateCommandLine struct {
		name                string
		additionCommandLine *[]string // addition
	}

	validateCommandLines := []validateCommandLine{
		{names.KarmadaEtcdComponentName, &i.EtcdExtraArgs},
		{names.KarmadaAPIServerComponentName, &i.KarmadaAPIServerExtraArgs},
		{names.KarmadaSchedulerComponentName, &i.KarmadaSchedulerExtraArgs},
		{names.KubeControllerManagerComponentName, &i.KubeControllerManagerExtraArgs},
		{names.KarmadaControllerManagerComponentName, &i.KarmadaControllerManagerExtraArgs},
		{names.KarmadaWebhookComponentName, &i.KarmadaWebhookExtraArgs},
		{names.KarmadaAggregatedAPIServerComponentName, &i.KarmadaAggregatedAPIServerExtraArgs},
	}
	// validate case -> vc
	for _, vc := range validateCommandLines {
		if *vc.additionCommandLine != nil {
			var err error
			*vc.additionCommandLine, err = utils.ValidateExtraArgs(*vc.additionCommandLine)
			if err != nil {
				klog.Errorf("validate %s extra args failed: %v", vc.name, err)
				return err
			}
		}
	}
	return nil
}

// Validate Check that there are enough flags to run the command.
func (i *CommandInitOption) Validate(parentCommand string) error {
	// validate command line args
	err := i.validateCommandLineArgs()
	if err != nil {
		return err
	}

	// validate secret layout
	if i.SecretLayout != secretLayoutSplit {
		i.SecretLayout = secretLayoutLegacy
	}
	if i.KarmadaInitFilePath != "" {
		cfg, err := initConfig.LoadInitConfiguration(i.KarmadaInitFilePath)
		if err != nil {
			return fmt.Errorf("failed to load karmada init configuration: %v", err)
		}
		if err := i.parseInitConfig(cfg); err != nil {
			return fmt.Errorf("failed to parse karmada init configuration: %v", err)
		}
	}

	if i.KarmadaAPIServerAdvertiseAddress != "" {
		if netutils.ParseIPSloppy(i.KarmadaAPIServerAdvertiseAddress) == nil {
			return fmt.Errorf("karmada apiserver advertise address is not valid")
		}
	}
	if (i.CaCertFile != "") != (i.CaKeyFile != "") {
		return fmt.Errorf("ca-cert-file and ca-key-file must be used together")
	}

	switch i.ImagePullPolicy {
	case string(corev1.PullAlways), string(corev1.PullIfNotPresent), string(corev1.PullNever):
		// continue
	default:
		return fmt.Errorf("invalid image pull policy: %s", i.ImagePullPolicy)
	}

	if i.isExternalEtcdProvided() {
		return i.validateExternalEtcd(parentCommand)
	}
	return i.validateLocalEtcd(parentCommand)
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

	if err := i.handleEtcdNodeSelectorLabels(); err != nil {
		return err
	}

	if !i.isExternalEtcdProvided() && i.EtcdStorageMode == "hostPath" && i.EtcdNodeSelectorLabels != "" {
		labels := strings.Split(i.EtcdNodeSelectorLabels, ",")
		for _, label := range labels {
			if !i.isNodeExist(label) {
				return fmt.Errorf("no node found by label %s", label)
			}
		}
	}

	i.initializeCommandLineArgs()

	return initializeDirectory(i.KarmadaDataPath)
}

// initializeCommandLineArgs Merge default parameters and validated user parameters.
func (i *CommandInitOption) initializeCommandLineArgs() {
	type mergeCommandLine struct {
		name                string
		finalCommandLine    *[]string
		defaultCommandLine  func() []string
		additionCommandLine []string
	}

	mergeCommandLines := []mergeCommandLine{
		{names.KarmadaEtcdComponentName, &i.EtcdContainerCmd, i.defaultEtcdContainerCommand, i.EtcdExtraArgs},
		{names.KarmadaAPIServerComponentName, &i.KarmadaAPIServerContainerCmd, i.defaultKarmadaAPIServerContainerCommand, i.KarmadaAPIServerExtraArgs},
		{names.KarmadaSchedulerComponentName, &i.KarmadaSchedulerContainerCmd, i.defaultKarmadaSchedulerContainerCommand, i.KarmadaSchedulerExtraArgs},
		{names.KubeControllerManagerComponentName, &i.KubeControllerManagerContainerCmd, i.defaultKarmadaKubeControllerManagerContainerCommand, i.KubeControllerManagerExtraArgs},
		{names.KarmadaControllerManagerComponentName, &i.KarmadaControllerManagerContainerCmd, i.defaultKarmadaControllerManagerContainerCommand, i.KarmadaControllerManagerExtraArgs},
		{names.KarmadaWebhookComponentName, &i.KarmadaWebhookContainerCmd, i.defaultKarmadaWebhookContainerCommand, i.KarmadaWebhookExtraArgs},
		{names.KarmadaAggregatedAPIServerComponentName, &i.KarmadaAggregatedAPIServerContainerCmd, i.defaultKarmadaAggregatedAPIServerContainerCommand, i.KarmadaAggregatedAPIServerExtraArgs},
	}

	for _, mc := range mergeCommandLines {
		if mc.additionCommandLine != nil {
			*mc.finalCommandLine = utils.MergeCommandArgs(mc.defaultCommandLine(), mc.additionCommandLine)
		} else {
			*mc.finalCommandLine = mc.defaultCommandLine()
		}
	}
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
	if err = cert.GenCerts(i.KarmadaPkiPath, i.CaCertFile, i.CaKeyFile, etcdServerCertConfig, etcdClientCertCfg, karmadaCertCfg, apiserverCertCfg, frontProxyClientCertCfg); err != nil {
		return err
	}
	return nil
}

// buildDefaultAltNames builds a standard AltNames set for in-cluster service endpoints
// of a given component, plus localhost and optional external DNS/IPs.
func (i *CommandInitOption) buildDefaultAltNames(componentName string) certutil.AltNames {
	defaultDNS := []string{
		fmt.Sprintf("%s.karmada-system.svc", componentName),
		fmt.Sprintf("%s.karmada-system.svc.cluster.local", componentName),
		fmt.Sprintf("%s.%s.svc.%s", componentName, i.Namespace, i.HostClusterDomain),
		"localhost",
	}
	defaultDNS = append(defaultDNS, utils.FlagsDNS(i.ExternalDNS)...)

	defaultIPs := []net.IP{utils.StringToNetIP("127.0.0.1")}
	defaultIPs = append(
		defaultIPs,
		utils.FlagsIP(i.ExternalIP)...,
	)
	defaultIPs = append(defaultIPs, i.KarmadaAPIServerIP...)
	if ip, err := utils.InternetIP(); err == nil {
		defaultIPs = append(defaultIPs, ip)
	} else {
		klog.Warningln("Failed to obtain internet IP. ", err)
	}

	return certutil.AltNames{DNSNames: defaultDNS, IPs: defaultIPs}
}

// buildSchedulerEstimatorAltNames returns AltNames for scheduler-estimator server
// covering wildcard services in karmada-system and namespace-scoped services.
func (i *CommandInitOption) buildSchedulerEstimatorAltNames() certutil.AltNames {
	dns := []string{
		"*.karmada-system.svc.cluster.local",
		"*.karmada-system.svc",
		fmt.Sprintf("*.%s.svc.%s", i.Namespace, i.HostClusterDomain),
		"localhost",
	}
	ips := []net.IP{utils.StringToNetIP("127.0.0.1")}
	dns = append(dns, utils.FlagsDNS(i.ExternalDNS)...)
	ips = append(ips, utils.FlagsIP(i.ExternalIP)...)
	ips = append(ips, i.KarmadaAPIServerIP...)
	if ip, err := utils.InternetIP(); err == nil {
		ips = append(ips, ip)
	} else {
		klog.Warningln("Failed to obtain internet IP. ", err)
	}
	return certutil.AltNames{DNSNames: dns, IPs: ips}
}

// genCertsSplitFull builds full set of component certificates (independent CN + SAN)
// and generates them using cert.NewGenCerts with proper CA selection.
func (i *CommandInitOption) genCertsSplitFull() error {
	notAfter := time.Now().Add(i.CertValidity).UTC()

	cfg := map[string]*cert.CertsConfig{}

	// Admin kubeconfig client (for users)
	adminAlt := i.buildDefaultAltNames("karmada-apiserver")
	cfg[options.KarmadaCertAndKeyName] = cert.NewCertConfig("system:admin", []string{"system:masters"}, adminAlt, &notAfter)

	// Servers
	cfg[options.KarmadaAPIServerCertAndKeyName] = cert.NewCertConfig(options.KarmadaAPIServerCN, []string{}, i.buildDefaultAltNames("karmada-apiserver"), &notAfter)
	cfg[options.KarmadaAggregatedAPIServerCertAndKeyName] = cert.NewCertConfig(options.KarmadaAggregatedAPIServerCN, []string{}, i.buildDefaultAltNames("karmada-aggregated-apiserver"), &notAfter)
	cfg[options.KarmadaWebhookCertAndKeyName] = cert.NewCertConfig(options.KarmadaWebhookCN, []string{}, i.buildDefaultAltNames("karmada-webhook"), &notAfter)
	cfg[options.KarmadaSearchCertAndKeyName] = cert.NewCertConfig(options.KarmadaSearchCN, []string{}, i.buildDefaultAltNames(names.KarmadaSearchComponentName), &notAfter)
	cfg[options.KarmadaMetricsAdapterCertAndKeyName] = cert.NewCertConfig(options.KarmadaMetricsAdapterCN, []string{}, i.buildDefaultAltNames(names.KarmadaMetricsAdapterComponentName), &notAfter)
	cfg[options.KarmadaSchedulerEstimatorCertAndKeyName] = cert.NewCertConfig(options.KarmadaSchedulerEstimatorCN, []string{}, i.buildSchedulerEstimatorAltNames(), &notAfter)

	if !i.isExternalEtcdProvided() {
		// etcd server
		etcdDNS := []string{"localhost"}
		for number := int32(0); number < i.EtcdReplicas; number++ {
			etcdDNS = append(etcdDNS, fmt.Sprintf("%s-%v.%s.%s.svc.%s", etcdStatefulSetAndServiceName, number, etcdStatefulSetAndServiceName, i.Namespace, i.HostClusterDomain))
		}
		cfg[options.EtcdServerCertAndKeyName] = cert.NewCertConfig(options.KarmadaEtcdServerCN, []string{}, certutil.AltNames{DNSNames: etcdDNS, IPs: []net.IP{utils.StringToNetIP("127.0.0.1")}}, &notAfter)
	}

	// Clients (Org: system:masters)
	cfg[options.KarmadaAPIServerClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaAPIServerCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaAggregatedAPIServerClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaAggregatedAPIServerCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaWebhookClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaWebhookCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaSearchClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaSearchCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaMetricsAdapterClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaMetricsAdapterCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaSchedulerEstimatorClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaSchedulerEstimatorCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaControllerManagerClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaControllerManagerCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KubeControllerManagerClientCertAndKeyName] = cert.NewCertConfig(options.KubeControllerManagerCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaSchedulerClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaSchedulerCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaDeschedulerClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaDeschedulerCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)

	// ETCD clients
	cfg[options.KarmadaAPIServerEtcdClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaAPIServerEtcdClientCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaAggregatedAPIServerEtcdClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaAggregatedAPIServerEtcdClientCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaSearchEtcdClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaSearchEtcdClientCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.EtcdClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaEtcdClientCN, []string{""}, certutil.AltNames{}, &notAfter)

	// GRPC clients
	cfg[options.KarmadaSchedulerGrpcCertAndKeyName] = cert.NewCertConfig(options.KarmadaSchedulerGrpcCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)
	cfg[options.KarmadaDeschedulerGrpcCertAndKeyName] = cert.NewCertConfig(options.KarmadaDeschedulerGrpcCN, []string{"system:masters"}, certutil.AltNames{}, &notAfter)

	// front-proxy client
	cfg[options.FrontProxyClientCertAndKeyName] = cert.NewCertConfig(options.KarmadaFrontProxyClientCN, []string{}, certutil.AltNames{}, &notAfter)

	if err := cert.NewGenCerts(i.KarmadaPkiPath, i.CaCertFile, i.CaKeyFile, cfg); err != nil {
		return err
	}
	return nil
}

// prepareCRD download or unzip `crds.tar.gz` to `options.DataPath`
func (i *CommandInitOption) prepareCRD() error {
	var filename string
	if strings.HasPrefix(i.CRDs, "http") {
		filename = i.KarmadaDataPath + "/" + path.Base(i.CRDs)
		klog.Infof("download crds file:%s", i.CRDs)
		if err := utils.DownloadFile(i.CRDs, filename); err != nil {
			return err
		}
	} else {
		filename = i.CRDs
		klog.Infoln("local crds file name:", i.CRDs)
	}

	if err := validation.ValidateTarball(filename, validation.ValidateCrdsTarBall); err != nil {
		return fmt.Errorf("inValid crd tar, err: %w", err)
	}

	if err := utils.DeCompress(filename, i.KarmadaDataPath); err != nil {
		return err
	}

	for _, archive := range validation.CrdsArchive {
		expectedDir := filepath.Join(i.KarmadaDataPath, archive)
		exist, _ := utils.PathExists(expectedDir)
		if !exist {
			return fmt.Errorf("lacking the necessary file path: %s", expectedDir)
		}
	}
	return nil
}

func (i *CommandInitOption) createCertsSecrets() error {
	if strings.ToLower(i.SecretLayout) == secretLayoutSplit {
		return i.createSplitCertsSecrets()
	}
	// Create karmada-config Secret
	karmadaServerURL := fmt.Sprintf("https://%s.%s.svc.%s:%v", karmadaAPIServerDeploymentAndServiceName, i.Namespace, i.HostClusterDomain, karmadaAPIServerContainerPort)
	config := utils.CreateWithCerts(karmadaServerURL, options.UserName, options.UserName, i.CertAndKeyFileData[fmt.Sprintf("%s.crt", globaloptions.CaCertAndKeyName)],
		i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)], i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName)])
	configBytes, err := clientcmd.Write(*config)
	if err != nil {
		return fmt.Errorf("failure while serializing admin kubeConfig. %v", err)
	}

	for _, karmadaConfigSecretName := range karmadaConfigList {
		karmadaConfigSecret := i.SecretFromSpec(karmadaConfigSecretName, corev1.SecretTypeOpaque, map[string]string{util.KarmadaConfigFieldName: string(configBytes)})
		if err = util.CreateOrUpdateSecret(i.KubeClientSet, karmadaConfigSecret); err != nil {
			return err
		}
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
	for _, v := range certListLegacy {
		karmadaCert[fmt.Sprintf("%s.crt", v)] = string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", v)])
		karmadaCert[fmt.Sprintf("%s.key", v)] = string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", v)])
	}
	karmadaSecret := i.SecretFromSpec(globaloptions.KarmadaCertsName, corev1.SecretTypeOpaque, karmadaCert)
	if err := util.CreateOrUpdateSecret(i.KubeClientSet, karmadaSecret); err != nil {
		return err
	}

	karmadaWebhookCert := map[string]string{
		certconst.KeyTLSCrt: string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaCertAndKeyName)]),
		certconst.KeyTLSKey: string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)]),
	}
	karmadaWebhookSecret := i.SecretFromSpec(certconst.SecretWebhook, corev1.SecretTypeOpaque, karmadaWebhookCert)
	if err := util.CreateOrUpdateSecret(i.KubeClientSet, karmadaWebhookSecret); err != nil {
		return err
	}

	return nil
}

// createSplitCertsSecrets creates per-component TLS secrets following the deploy-karmada.sh layout
func (i *CommandInitOption) createSplitCertsSecrets() error {
	// Kubeconfig for components (each uses its own client cert)
	karmadaServerURL := fmt.Sprintf("https://%s.%s.svc.%s:%v", "karmada-apiserver", i.Namespace, i.HostClusterDomain, karmadaAPIServerContainerPort)

	// helper to create kubeconfig secret by client cert name
	createCfg := func(secretName, clientCertName string) error {
		cfg := utils.CreateWithCerts(
			karmadaServerURL,
			options.UserName,
			options.UserName,
			i.CertAndKeyFileData[fmt.Sprintf("%s.crt", globaloptions.CaCertAndKeyName)],
			i.CertAndKeyFileData[fmt.Sprintf("%s.key", clientCertName)],
			i.CertAndKeyFileData[fmt.Sprintf("%s.crt", clientCertName)],
		)
		b, err := clientcmd.Write(*cfg)
		if err != nil {
			return fmt.Errorf("failure while serializing component kubeConfig for %s: %v", secretName, err)
		}
		sec := i.SecretFromSpec(secretName, corev1.SecretTypeOpaque, map[string]string{util.KarmadaConfigFieldName: string(b)})
		return util.CreateOrUpdateSecret(i.KubeClientSet, sec)
	}

	// mapping from component config secret to its client cert name
	cfgMapping := map[string]string{
		util.KarmadaConfigName(names.KarmadaAggregatedAPIServerComponentName): options.KarmadaAggregatedAPIServerClientCertAndKeyName,
		util.KarmadaConfigName(names.KarmadaControllerManagerComponentName):   options.KarmadaControllerManagerClientCertAndKeyName,
		util.KarmadaConfigName(names.KubeControllerManagerComponentName):      options.KubeControllerManagerClientCertAndKeyName,
		util.KarmadaConfigName(names.KarmadaSchedulerComponentName):           options.KarmadaSchedulerClientCertAndKeyName,
		util.KarmadaConfigName(names.KarmadaDeschedulerComponentName):         options.KarmadaDeschedulerClientCertAndKeyName,
		util.KarmadaConfigName(names.KarmadaMetricsAdapterComponentName):      options.KarmadaMetricsAdapterClientCertAndKeyName,
		util.KarmadaConfigName(names.KarmadaSearchComponentName):              options.KarmadaSearchClientCertAndKeyName,
		util.KarmadaConfigName(names.KarmadaWebhookComponentName):             options.KarmadaWebhookClientCertAndKeyName,
	}

	for secretName, client := range cfgMapping {
		if err := createCfg(secretName, client); err != nil {
			return err
		}
	}

	caCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", globaloptions.CaCertAndKeyName)])
	caKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", globaloptions.CaCertAndKeyName)])
	apiserverCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaAPIServerCertAndKeyName)])
	apiserverKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaAPIServerCertAndKeyName)])
	aggregatedCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaAggregatedAPIServerCertAndKeyName)])
	aggregatedKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaAggregatedAPIServerCertAndKeyName)])
	webhookServerCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaWebhookCertAndKeyName)])
	webhookServerKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaWebhookCertAndKeyName)])
	frontProxyCaCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.FrontProxyCaCertAndKeyName)])
	frontProxyClientCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.FrontProxyClientCertAndKeyName)])
	frontProxyClientKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.FrontProxyClientCertAndKeyName)])
	etcdCaCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.EtcdCaCertAndKeyName)])
	etcdServerCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.EtcdServerCertAndKeyName)])
	etcdServerKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.EtcdServerCertAndKeyName)])
	// Component-specific etcd client certs
	etcdClientCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.EtcdClientCertAndKeyName)])
	etcdClientKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.EtcdClientCertAndKeyName)])
	apiserverEtcdClientCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaAPIServerEtcdClientCertAndKeyName)])
	apiserverEtcdClientKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaAPIServerEtcdClientCertAndKeyName)])
	aggregatedEtcdClientCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaAggregatedAPIServerEtcdClientCertAndKeyName)])
	aggregatedEtcdClientKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaAggregatedAPIServerEtcdClientCertAndKeyName)])

	saPriv, saPub, err := generateServiceAccountKeyPairPEM()
	if err != nil {
		return fmt.Errorf("generate service account key pair failed: %v", err)
	}
	saPrivStr := string(saPriv)
	saPubStr := string(saPub)

	err = i.createSecret(caCrt, caKey,
		aggregatedCrt, aggregatedKey,
		apiserverCrt, apiserverKey,
		frontProxyCaCrt, frontProxyClientCrt, frontProxyClientKey,
		etcdCaCrt, etcdServerCrt, etcdServerKey, etcdClientCrt, etcdClientKey,
		apiserverEtcdClientCrt, apiserverEtcdClientKey,
		aggregatedEtcdClientCrt, aggregatedEtcdClientKey,
		saPrivStr, saPubStr,
		webhookServerCrt, webhookServerKey)
	if err != nil {
		return err
	}

	// CA-only compatibility secret for addons to read CABundle
	compat := i.SecretFromSpec(globaloptions.KarmadaCertsName, corev1.SecretTypeOpaque, map[string]string{certconst.KeyCACrt: caCrt})
	if err := util.CreateOrUpdateSecret(i.KubeClientSet, compat); err != nil {
		return err
	}

	return nil
}

func (i *CommandInitOption) createSecret(caCrt, caKey string,
	aggregatedCrt, aggregatedKey string,
	apiserverCrt, apiserverKey string,
	frontProxyCaCrt, frontProxyClientCrt, frontProxyClientKey string,
	etcdCaCrt, etcdServerCrt, etcdServerKey, etcdClientCrt, etcdClientKey string,
	apiserverEtcdClientCrt, apiserverEtcdClientKey string,
	aggregatedEtcdClientCrt, aggregatedEtcdClientKey string,
	saPriv, saPub string,
	webhookServerCrt, webhookServerKey string) error {
	// Delegate secret creation to helper functions
	if err := i.createAPIServerSecret(caCrt, apiserverCrt, apiserverKey, etcdCaCrt, apiserverEtcdClientCrt, apiserverEtcdClientKey, frontProxyCaCrt, frontProxyClientCrt, frontProxyClientKey, saPriv, saPub); err != nil {
		return err
	}
	if err := i.createAggregatedAPIServerSecret(caCrt, aggregatedCrt, aggregatedKey, etcdCaCrt, aggregatedEtcdClientCrt, aggregatedEtcdClientKey); err != nil {
		return err
	}
	if err := i.createEtcdSecret(etcdCaCrt, etcdServerCrt, etcdServerKey, etcdClientCrt, etcdClientKey); err != nil {
		return err
	}
	if err := i.createKubeControllerManagerSecret(caCrt, caKey, saPriv, saPub); err != nil {
		return err
	}
	if err := i.createKarmadaWebhookSecret(caCrt, webhookServerCrt, webhookServerKey); err != nil {
		return err
	}
	// scheduler estimator client
	schedEstCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaSchedulerEstimatorClientCertAndKeyName)])
	schedEstKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaSchedulerEstimatorClientCertAndKeyName)])
	if err := i.createKarmadaSchedulerEstimatorSecret(caCrt, schedEstCrt, schedEstKey); err != nil {
		return err
	}
	// descheduler estimator client (reuse descheduler client cert)
	deschedEstCrt := string(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", options.KarmadaDeschedulerClientCertAndKeyName)])
	deschedEstKey := string(i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaDeschedulerClientCertAndKeyName)])
	if err := i.createKarmadaDeschedulerEstimatorSecret(caCrt, deschedEstCrt, deschedEstKey); err != nil {
		return err
	}

	return nil
}

func (i *CommandInitOption) createAPIServerSecret(caCrt, apiserverCrt, apiserverKey, etcdCaCrt,
	etcdClientCrt, etcdClientKey, frontProxyCaCrt, frontProxyClientCrt, frontProxyClientKey, saPriv, saPub string) error {
	if err := i.createTLSSecret(certconst.SecretApiserverServer, caCrt, apiserverCrt, apiserverKey); err != nil {
		return err
	}
	if err := i.createTLSSecret(certconst.SecretApiserverEtcdClient, etcdCaCrt, etcdClientCrt, etcdClientKey); err != nil {
		return err
	}
	if err := i.createTLSSecret(certconst.SecretApiserverFrontProxyClient, frontProxyCaCrt, frontProxyClientCrt, frontProxyClientKey); err != nil {
		return err
	}
	saSec := i.SecretFromSpec(certconst.SecretApiserverServiceAccountKeys, corev1.SecretTypeOpaque, map[string]string{certconst.KeySAPrivate: saPriv, certconst.KeySAPublic: saPub})
	if err := util.CreateOrUpdateSecret(i.KubeClientSet, saSec); err != nil {
		return err
	}

	return nil
}

func (i *CommandInitOption) createAggregatedAPIServerSecret(caCrt, karmadaCrt, karmadaKey, etcdCaCrt,
	etcdClientCrt, etcdClientKey string) error {
	if err := i.createTLSSecret(certconst.SecretAggregatedAPIServerServer, caCrt, karmadaCrt, karmadaKey); err != nil {
		return err
	}
	if err := i.createTLSSecret(certconst.SecretAggregatedAPIServerEtcdClient, etcdCaCrt, etcdClientCrt, etcdClientKey); err != nil {
		return err
	}

	return nil
}

func (i *CommandInitOption) createEtcdSecret(etcdCaCrt, etcdServerCrt, etcdServerKey,
	etcdClientCrt, etcdClientKey string) error {
	// etcd server & etcd-client (for internal usage like probes/clients)
	if !i.isExternalEtcdProvided() {
		if err := i.createTLSSecret(certconst.SecretEtcdServer, etcdCaCrt, etcdServerCrt, etcdServerKey); err != nil {
			return err
		}
		if err := i.createTLSSecret(certconst.SecretEtcdClient, etcdCaCrt, etcdClientCrt, etcdClientKey); err != nil {
			return err
		}
	}

	return nil
}

func (i *CommandInitOption) createKubeControllerManagerSecret(caCrt, caKey, saPriv, saPub string) error {
	kcmCA := i.SecretFromSpec(certconst.SecretKubeControllerManagerCA, corev1.SecretTypeTLS, map[string]string{certconst.KeyTLSCrt: caCrt, certconst.KeyTLSKey: caKey})
	if err := util.CreateOrUpdateSecret(i.KubeClientSet, kcmCA); err != nil {
		return err
	}
	kcmSA := i.SecretFromSpec(certconst.SecretKubeControllerManagerSAKeys, corev1.SecretTypeOpaque, map[string]string{certconst.KeySAPrivate: string(saPriv), certconst.KeySAPublic: string(saPub)})
	if err := util.CreateOrUpdateSecret(i.KubeClientSet, kcmSA); err != nil {
		return err
	}

	return nil
}

func (i *CommandInitOption) createKarmadaWebhookSecret(caCrt, karmadaCrt, karmadaKey string) error {
	if err := i.createTLSSecret(certconst.SecretWebhook, caCrt, karmadaCrt, karmadaKey); err != nil {
		return err
	}

	return nil
}

func (i *CommandInitOption) createKarmadaSchedulerEstimatorSecret(caCrt, karmadaCrt, karmadaKey string) error {
	if err := i.createTLSSecret(certconst.SecretSchedulerEstimatorClient, caCrt, karmadaCrt, karmadaKey); err != nil {
		return err
	}

	return nil
}

func (i *CommandInitOption) createKarmadaDeschedulerEstimatorSecret(caCrt, karmadaCrt, karmadaKey string) error {
	if err := i.createTLSSecret(certconst.SecretDeschedulerEstimatorClient, caCrt, karmadaCrt, karmadaKey); err != nil {
		return err
	}

	return nil
}

// helper to create TLS secret with optional ca.crt
func (i *CommandInitOption) createTLSSecret(name string, ca, crt, key string) error {
	data := map[string]string{certconst.KeyTLSCrt: crt, certconst.KeyTLSKey: key}
	if ca != "" {
		data[certconst.KeyCACrt] = ca
	}
	sec := i.SecretFromSpec(name, corev1.SecretTypeTLS, data)
	return util.CreateOrUpdateSecret(i.KubeClientSet, sec)
}

// generateServiceAccountKeyPairPEM returns PEM encoded RSA private key and public key
func generateServiceAccountKeyPairPEM() ([]byte, []byte, error) {
	// 3072-bit RSA
	priv, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return nil, nil, err
	}
	// marshal private key
	privDer := x509.MarshalPKCS1PrivateKey(priv)
	privPem := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDer})
	// marshal public key in PKIX
	pubDer, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	pubPem := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDer})
	return privPem, pubPem, nil
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
	if err := utils.CreateDeployAndWait(i.KubeClientSet, karmadaAPIServerDeployment, i.WaitComponentReadyTimeout); err != nil {
		return err
	}

	// Create karmada-aggregated-apiserver
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-aggregated-apiserver.yaml
	klog.Info("Create karmada aggregated apiserver Deployment")
	if err := util.CreateOrUpdateService(i.KubeClientSet, i.karmadaAggregatedAPIServerService()); err != nil {
		klog.Exitln(err)
	}
	karmadaAggregatedAPIServerDeployment := i.makeKarmadaAggregatedAPIServerDeployment()
	if err := utils.CreateDeployAndWait(i.KubeClientSet, karmadaAggregatedAPIServerDeployment, i.WaitComponentReadyTimeout); err != nil {
		klog.Warning(err)
	}
	return nil
}

func (i *CommandInitOption) initKarmadaComponent() error {
	// Create karmada-kube-controller-manager
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/kube-controller-manager.yaml
	klog.Info("Create karmada kube controller manager Deployment")
	if err := util.CreateOrUpdateService(i.KubeClientSet, i.kubeControllerManagerService()); err != nil {
		klog.Exitln(err)
	}

	karmadaKubeControllerManagerDeployment := i.makeKarmadaKubeControllerManagerDeployment()
	if err := utils.CreateDeployAndWait(i.KubeClientSet, karmadaKubeControllerManagerDeployment, i.WaitComponentReadyTimeout); err != nil {
		klog.Warning(err)
	}

	// Create karmada-scheduler
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-scheduler.yaml
	klog.Info("Create karmada scheduler Deployment")
	karmadaSchedulerDeployment := i.makeKarmadaSchedulerDeployment()
	if err := utils.CreateDeployAndWait(i.KubeClientSet, karmadaSchedulerDeployment, i.WaitComponentReadyTimeout); err != nil {
		klog.Warning(err)
	}

	// Create karmada-controller-manager
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-controller-manager.yaml
	klog.Info("Create karmada controller manager Deployment")
	karmadaControllerManagerDeployment := i.makeKarmadaControllerManagerDeployment()
	if err := utils.CreateDeployAndWait(i.KubeClientSet, karmadaControllerManagerDeployment, i.WaitComponentReadyTimeout); err != nil {
		klog.Warning(err)
	}

	// Create karmada-webhook
	// https://github.com/karmada-io/karmada/blob/master/artifacts/deploy/karmada-webhook.yaml
	klog.Info("Create karmada webhook Deployment")
	if err := util.CreateOrUpdateService(i.KubeClientSet, i.karmadaWebhookService()); err != nil {
		klog.Exitln(err)
	}
	karmadaWebhookDeployment := i.makeKarmadaWebhookDeployment()
	if err := utils.CreateDeployAndWait(i.KubeClientSet, karmadaWebhookDeployment, i.WaitComponentReadyTimeout); err != nil {
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

// generateCertsForLayout selects and generates certificates according to secret layout.
func (i *CommandInitOption) generateCertsForLayout() error {
	if strings.ToLower(i.SecretLayout) == secretLayoutSplit {
		return i.genCertsSplitFull()
	}
	return i.genCerts()
}

// loadCertsIntoMap populates CertAndKeyFileData either from disk or external etcd inputs.
func (i *CommandInitOption) loadCertsIntoMap() error {
	i.CertAndKeyFileData = map[string][]byte{}

	list := certListLegacy
	if strings.ToLower(i.SecretLayout) == secretLayoutSplit {
		list = certListSplit
	}
	for _, v := range list {
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
	return nil
}

// createNamespaceAndSecrets ensures target namespace exists and uploads certs as secrets.
func (i *CommandInitOption) createNamespaceAndSecrets() error {
	if err := util.CreateOrUpdateNamespace(i.KubeClientSet, util.NewNamespace(i.Namespace)); err != nil {
		return fmt.Errorf("create namespace %s failed: %v", i.Namespace, err)
	}
	return i.createCertsSecrets()
}

// RunInit Deploy karmada in kubernetes
func (i *CommandInitOption) RunInit(parentCommand string) error {
	// generate certificates
	if err := i.generateCertsForLayout(); err != nil {
		return fmt.Errorf("certificate generation failed.%v", err)
	}

	if err := i.loadCertsIntoMap(); err != nil {
		return err
	}

	// prepare karmada CRD resources
	if err := i.prepareCRD(); err != nil {
		return fmt.Errorf("prepare karmada failed.%v", err)
	}

	// Create karmada kubeconfig
	if err := i.createKarmadaConfig(); err != nil {
		return fmt.Errorf("create karmada kubeconfig failed.%v", err)
	}

	// Create namespace and secrets
	if err := i.createNamespaceAndSecrets(); err != nil {
		return err
	}

	// install karmada-apiserver
	if err := i.initKarmadaAPIServer(); err != nil {
		return err
	}

	// Create CRDs in karmada
	caBase64 := base64.StdEncoding.EncodeToString(i.CertAndKeyFileData[fmt.Sprintf("%s.crt", globaloptions.CaCertAndKeyName)])
	if err := karmada.InitKarmadaResources(i.KarmadaDataPath, caBase64, i.Namespace); err != nil {
		return err
	}

	// install karmada Component
	if err := i.initKarmadaComponent(); err != nil {
		return err
	}

	utils.GenExamples(i.KarmadaDataPath, parentCommand)
	return nil
}

func (i *CommandInitOption) createKarmadaConfig() error {
	serverIP := i.KarmadaAPIServerIP[0].String()
	serverURL, err := generateServerURL(serverIP, i.KarmadaAPIServerNodePort)
	if err != nil {
		return err
	}
	if err := utils.WriteKubeConfigFromSpec(serverURL, options.UserName, options.ClusterName, i.KarmadaDataPath, options.KarmadaKubeConfigName,
		i.CertAndKeyFileData[fmt.Sprintf("%s.crt", globaloptions.CaCertAndKeyName)], i.CertAndKeyFileData[fmt.Sprintf("%s.key", options.KarmadaCertAndKeyName)],
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
	if i.ImageRegistry != "" && i.KarmadaWebhookImage == DefaultKarmadaWebhookImage {
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

func (i *CommandInitOption) handleEtcdNodeSelectorLabels() error {
	if i.EtcdStorageMode == etcdStorageModeHostPath && i.EtcdNodeSelectorLabels != "" {
		selector, err := metav1.ParseToLabelSelector(i.EtcdNodeSelectorLabels)
		if err != nil {
			return fmt.Errorf("the etcdNodeSelector format is incorrect: %s", err)
		}
		labelMap, err := metav1.LabelSelectorAsMap(selector)
		if err != nil {
			return fmt.Errorf("failed to convert etcdNodeSelector labels to map: %v", err)
		}
		i.EtcdNodeSelectorLabelsMap = labelMap
	}
	return nil
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

// parseEtcdNodeSelectorLabelsMap parse etcd node selector labels
func (i *CommandInitOption) parseEtcdNodeSelectorLabelsMap() error {
	if i.EtcdNodeSelectorLabels == "" {
		return nil
	}
	// Parse the label selector string into a LabelSelector object
	selector, err := metav1.ParseToLabelSelector(i.EtcdNodeSelectorLabels)
	if err != nil {
		return fmt.Errorf("the etcdNodeSelector format is incorrect: %s", err)
	}
	// Convert the LabelSelector object into a map[string]string
	labelMap, err := metav1.LabelSelectorAsMap(selector)
	if err != nil {
		return fmt.Errorf("failed to convert etcdNodeSelector labels to map: %v", err)
	}
	i.EtcdNodeSelectorLabelsMap = labelMap
	return nil
}

// parseInitConfig parses fields from KarmadaInitConfig into CommandInitOption.
// It is responsible for delegating the parsing of various configuration sections,
// such as certificates, etcd, and control plane components.
func (i *CommandInitOption) parseInitConfig(cfg *initConfig.KarmadaInitConfig) error {
	spec := cfg.Spec

	i.parseGeneralConfig(spec)
	i.parseCertificateConfig(spec.Certificates)
	err := i.parseEtcdConfig(spec.Etcd)
	if err != nil {
		return err
	}
	err = i.parseControlPlaneConfig(spec.Components)
	if err != nil {
		return err
	}

	setIfNotEmpty(&i.KarmadaDataPath, spec.KarmadaDataPath)
	setIfNotEmpty(&i.KarmadaPkiPath, spec.KarmadaPKIPath)
	setIfNotEmpty(&i.HostClusterDomain, spec.HostCluster.Domain)
	setIfNotEmpty(&i.CRDs, spec.KarmadaCRDs)

	return nil
}

// parseGeneralConfig parses basic configuration related to the host cluster,
// such as namespace, kubeconfig, and image settings from the KarmadaInitConfigSpec.
func (i *CommandInitOption) parseGeneralConfig(spec initConfig.KarmadaInitSpec) {
	setIfNotEmpty(&i.KubeConfig, spec.HostCluster.Kubeconfig)
	setIfNotEmpty(&i.KubeImageTag, spec.Images.KubeImageTag)
	setIfNotEmpty(&i.KubeImageRegistry, spec.Images.KubeImageRegistry)
	setIfNotEmpty(&i.KubeImageMirrorCountry, spec.Images.KubeImageMirrorCountry)

	if spec.Images.PrivateRegistry != nil {
		setIfNotEmpty(&i.ImageRegistry, spec.Images.PrivateRegistry.Registry)
	}
	setIfNotEmpty(&i.ImagePullPolicy, string(spec.Images.ImagePullPolicy))
	setIfNotEmpty(&i.Context, spec.HostCluster.Context)

	if len(spec.Images.ImagePullSecrets) != 0 {
		i.PullSecrets = spec.Images.ImagePullSecrets
	}
	setIfNotZero(&i.WaitComponentReadyTimeout, spec.WaitComponentReadyTimeout)
	setIfNotEmpty(&i.SecretLayout, spec.SecretLayout)
}

// parseCertificateConfig parses certificate-related configuration, including CA files,
// external DNS, and external IP from the Certificates configuration block.
func (i *CommandInitOption) parseCertificateConfig(certificates initConfig.Certificates) {
	setIfNotEmpty(&i.CaKeyFile, certificates.CAKeyFile)
	setIfNotEmpty(&i.CaCertFile, certificates.CACertFile)

	if len(certificates.ExternalDNS) > 0 {
		i.ExternalDNS = joinStringSlice(certificates.ExternalDNS)
	}

	if len(certificates.ExternalIP) > 0 {
		i.ExternalIP = joinStringSlice(certificates.ExternalIP)
	}

	if certificates.ValidityPeriod.Duration != 0 {
		i.CertValidity = certificates.ValidityPeriod.Duration
	}
}

// parseEtcdConfig handles the parsing of both local and external Etcd configurations.
func (i *CommandInitOption) parseEtcdConfig(etcd initConfig.Etcd) error {
	if etcd.Local != nil {
		return i.parseLocalEtcdConfig(etcd.Local)
	}
	if etcd.External != nil {
		i.parseExternalEtcdConfig(etcd.External)
	}
	return nil
}

// parseLocalEtcdConfig parses the local Etcd settings, including image information,
// data path, PVC size, and node selector labels.
func (i *CommandInitOption) parseLocalEtcdConfig(localEtcd *initConfig.LocalEtcd) error {
	setIfNotEmpty(&i.EtcdImage, localEtcd.CommonSettings.Image.GetImage())
	setIfNotEmpty(&i.EtcdHostDataPath, localEtcd.DataPath)
	setIfNotEmpty(&i.EtcdPersistentVolumeSize, localEtcd.PVCSize)

	if len(localEtcd.NodeSelectorLabels) != 0 {
		i.EtcdNodeSelectorLabels = mapToString(localEtcd.NodeSelectorLabels)
	}

	setIfNotEmpty(&i.EtcdStorageMode, localEtcd.StorageMode)
	setIfNotEmpty(&i.StorageClassesName, localEtcd.StorageClassesName)
	setIfNotZeroInt32(&i.EtcdReplicas, localEtcd.Replicas)

	if localEtcd.ExtraArgs != nil {
		var err error
		i.EtcdExtraArgs, err = setComponentArgs(i.EtcdExtraArgs, localEtcd.ExtraArgs)
		return err
	}
	return nil
}

// parseExternalEtcdConfig parses the external Etcd configuration, including CA file,
// client certificates, and endpoints.
func (i *CommandInitOption) parseExternalEtcdConfig(externalEtcd *initConfig.ExternalEtcd) {
	setIfNotEmpty(&i.ExternalEtcdCACertPath, externalEtcd.CAFile)
	setIfNotEmpty(&i.ExternalEtcdClientCertPath, externalEtcd.CertFile)
	setIfNotEmpty(&i.ExternalEtcdClientKeyPath, externalEtcd.KeyFile)

	if len(externalEtcd.Endpoints) > 0 {
		i.ExternalEtcdServers = strings.Join(externalEtcd.Endpoints, ",")
	}
	setIfNotEmpty(&i.ExternalEtcdKeyPrefix, externalEtcd.KeyPrefix)
}

// parseControlPlaneConfig parses the configuration for various control plane components,
// including API Server, Controller Manager, Scheduler, and Webhook.
func (i *CommandInitOption) parseControlPlaneConfig(components initConfig.KarmadaComponents) error {
	steps := []func() error{
		func() error { return i.parseKarmadaAPIServerConfig(components.KarmadaAPIServer) },
		func() error { return i.parseKarmadaControllerManagerConfig(components.KarmadaControllerManager) },
		func() error { return i.parseKarmadaSchedulerConfig(components.KarmadaScheduler) },
		func() error { return i.parseKarmadaWebhookConfig(components.KarmadaWebhook) },
		func() error { return i.parseKarmadaAggregatedAPIServerConfig(components.KarmadaAggregatedAPIServer) },
		func() error { return i.parseKubeControllerManagerConfig(components.KubeControllerManager) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

// parseKarmadaAPIServerConfig parses the configuration for the Karmada API Server component,
// including image and replica settings, as well as advertise address.
func (i *CommandInitOption) parseKarmadaAPIServerConfig(apiServer *initConfig.KarmadaAPIServer) error {
	if apiServer != nil {
		setIfNotZeroInt32(&i.KarmadaAPIServerNodePort, apiServer.Networking.Port)
		setIfNotEmpty(&i.Namespace, apiServer.Networking.Namespace)
		setIfNotEmpty(&i.KarmadaAPIServerImage, apiServer.CommonSettings.Image.GetImage())
		setIfNotZeroInt32(&i.KarmadaAPIServerReplicas, apiServer.CommonSettings.Replicas)
		setIfNotEmpty(&i.KarmadaAPIServerAdvertiseAddress, apiServer.AdvertiseAddress)

		if apiServer.ExtraArgs != nil {
			var err error
			i.KarmadaAPIServerExtraArgs, err = setComponentArgs(i.KarmadaAPIServerExtraArgs, apiServer.ExtraArgs)
			return err
		}
	}
	return nil
}

// parseKarmadaControllerManagerConfig parses the configuration for the Karmada Controller Manager,
// including image and replica settings.
func (i *CommandInitOption) parseKarmadaControllerManagerConfig(manager *initConfig.KarmadaControllerManager) error {
	if manager != nil {
		setIfNotEmpty(&i.KarmadaControllerManagerImage, manager.CommonSettings.Image.GetImage())
		setIfNotZeroInt32(&i.KarmadaControllerManagerReplicas, manager.CommonSettings.Replicas)

		if manager.ExtraArgs != nil {
			var err error
			i.KarmadaControllerManagerExtraArgs, err = setComponentArgs(i.KarmadaControllerManagerExtraArgs, manager.ExtraArgs)
			return err
		}
	}
	return nil
}

// parseKarmadaSchedulerConfig parses the configuration for the Karmada Scheduler,
// including image and replica settings.
func (i *CommandInitOption) parseKarmadaSchedulerConfig(scheduler *initConfig.KarmadaScheduler) error {
	if scheduler != nil {
		setIfNotEmpty(&i.KarmadaSchedulerImage, scheduler.CommonSettings.Image.GetImage())
		setIfNotZeroInt32(&i.KarmadaSchedulerReplicas, scheduler.CommonSettings.Replicas)

		if scheduler.ExtraArgs != nil {
			var err error
			i.KarmadaSchedulerExtraArgs, err = setComponentArgs(i.KarmadaSchedulerExtraArgs, scheduler.ExtraArgs)
			return err
		}
	}
	return nil
}

// parseKarmadaWebhookConfig parses the configuration for the Karmada Webhook,
// including image and replica settings.
func (i *CommandInitOption) parseKarmadaWebhookConfig(webhook *initConfig.KarmadaWebhook) error {
	if webhook != nil {
		setIfNotEmpty(&i.KarmadaWebhookImage, webhook.CommonSettings.Image.GetImage())
		setIfNotZeroInt32(&i.KarmadaWebhookReplicas, webhook.CommonSettings.Replicas)

		if webhook.ExtraArgs != nil {
			var err error
			i.KarmadaWebhookExtraArgs, err = setComponentArgs(i.KarmadaWebhookExtraArgs, webhook.ExtraArgs)
			return err
		}
	}
	return nil
}

// parseKarmadaAggregatedAPIServerConfig parses the configuration for the Karmada Aggregated API Server,
// including image and replica settings.
func (i *CommandInitOption) parseKarmadaAggregatedAPIServerConfig(aggregatedAPIServer *initConfig.KarmadaAggregatedAPIServer) error {
	if aggregatedAPIServer != nil {
		setIfNotEmpty(&i.KarmadaAggregatedAPIServerImage, aggregatedAPIServer.CommonSettings.Image.GetImage())
		setIfNotZeroInt32(&i.KarmadaAggregatedAPIServerReplicas, aggregatedAPIServer.CommonSettings.Replicas)

		if aggregatedAPIServer.ExtraArgs != nil {
			var err error
			i.KarmadaAggregatedAPIServerExtraArgs, err = setComponentArgs(i.KarmadaAggregatedAPIServerExtraArgs, aggregatedAPIServer.ExtraArgs)
			return err
		}
	}
	return nil
}

// parseKubeControllerManagerConfig parses the configuration for the Kube Controller Manager,
// including image and replica settings.
func (i *CommandInitOption) parseKubeControllerManagerConfig(manager *initConfig.KubeControllerManager) error {
	if manager != nil {
		setIfNotEmpty(&i.KubeControllerManagerImage, manager.CommonSettings.Image.GetImage())
		setIfNotZeroInt32(&i.KubeControllerManagerReplicas, manager.CommonSettings.Replicas)

		if manager.ExtraArgs != nil {
			var err error
			i.KubeControllerManagerExtraArgs, err = setComponentArgs(i.KubeControllerManagerExtraArgs, manager.ExtraArgs)
			return err
		}
	}
	return nil
}

// mapToString converts a map to a comma-separated key=value string.
func mapToString(m map[string]string) string {
	var builder strings.Builder
	for k, v := range m {
		if builder.Len() > 0 {
			builder.WriteString(",")
		}
		builder.WriteString(fmt.Sprintf("%s=%s", k, v))
	}
	return builder.String()
}

// setIfNotEmpty checks if the source string is not empty, and if so, assigns its value to the destination string.
func setIfNotEmpty(dest *string, src string) {
	if src != "" {
		*dest = src
	}
}

// setIfNotZero checks if the source integer is not zero, and if so, assigns its value to the destination integer.
func setIfNotZero(dest *int, src int) {
	if src != 0 {
		*dest = src
	}
}

// setIfNotZeroInt32 checks if the source int32 is not zero, and if so, assigns its value to the destination int32.
func setIfNotZeroInt32(dest *int32, src int32) {
	if src != 0 {
		*dest = src
	}
}

// joinStringSlice joins a slice of strings into a single string separated by commas.
func joinStringSlice(slice []string) string {
	return strings.Join(slice, ",")
}
