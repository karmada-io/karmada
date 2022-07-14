package kubernetes

import (
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/version"
)

// NewCommandInitOption returns a new instance of CommandInitOption with default values.
func NewCommandInitOption() *CommandInitOption {
	releaseVer, err := version.ParseGitVersion(version.Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", version.Get().String())
		releaseVer = &version.ReleaseVersion{} // initialize to avoid panic
	}
	return &CommandInitOption{
		Namespace:                          "karmada-system",
		EtcdStorageMode:                    "hostPath",
		EtcdInitImage:                      "docker.io/alpine:3.15.1",
		EtcdReplicas:                       1,
		EtcdHostDataPath:                   "/var/lib/karmada-etcd",
		EtcdPersistentVolumeSize:           "5Gi",
		CRDs:                               fmt.Sprintf("https://github.com/karmada-io/karmada/releases/download/%s/crds.tar.gz", releaseVer.FirstMinorRelease()),
		KarmadaAPIServerNodePort:           32443,
		KarmadaDataPath:                    "/etc/karmada",
		KarmadaAPIServerReplicas:           1,
		KarmadaSchedulerImage:              fmt.Sprintf("swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-scheduler:%s", releaseVer.PatchRelease()),
		KarmadaSchedulerReplicas:           1,
		KubeControllerManagerReplicas:      1,
		KarmadaControllerManagerImage:      fmt.Sprintf("swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-controller-manager:%s", releaseVer.PatchRelease()),
		KarmadaControllerManagerReplicas:   1,
		KarmadaWebhookImage:                fmt.Sprintf("swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-webhook:%s", releaseVer.PatchRelease()),
		KarmadaWebhookReplicas:             1,
		KarmadaAggregatedAPIServerImage:    fmt.Sprintf("swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-aggregated-apiserver:%s", releaseVer.PatchRelease()),
		KarmadaAggregatedAPIServerReplicas: 1,
	}
}

// CommandInitOption holds all flags options for init.
type CommandInitOption struct {
	KubeImageRegistry      string
	KubeImageMirrorCountry string

	EtcdImage                string
	EtcdReplicas             int32
	EtcdInitImage            string
	EtcdStorageMode          string
	EtcdHostDataPath         string
	EtcdNodeSelectorLabels   string
	EtcdPersistentVolumeSize string

	KarmadaAPIServerImage              string
	KarmadaAPIServerReplicas           int32
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

	Namespace          string
	KubeConfig         string
	Context            string
	StorageClassesName string
	KarmadaDataPath    string
	CRDs               string
	ExternalIP         string
	ExternalDNS        string
	KubeClientSet      *kubernetes.Clientset
	CertAndKeyFileData map[string][]byte
	RestConfig         *rest.Config
	KarmadaAPIServerIP []net.IP
}

// AddFlags adds flags for command.
func (i *CommandInitOption) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()

	// kube image registry
	flags.StringVarP(&i.KubeImageMirrorCountry, "kube-image-mirror-country", "", i.KubeImageMirrorCountry, "Country code of the kube image registry to be used. For Chinese mainland users, set it to cn")
	flags.StringVarP(&i.KubeImageRegistry, "kube-image-registry", "", i.KubeImageRegistry, "Kube image registry. For Chinese mainland users, you may use local gcr.io mirrors such as registry.cn-hangzhou.aliyuncs.com/google_containers to override default kube image registry")
	// cert
	flags.StringVar(&i.ExternalIP, "cert-external-ip", i.ExternalIP, "the external IP of Karmada certificate (e.g 192.168.1.2,172.16.1.2)")
	flags.StringVar(&i.ExternalDNS, "cert-external-dns", i.ExternalDNS, "the external DNS of Karmada certificate (e.g localhost,localhost.com)")
	// Kubernetes
	flags.StringVarP(&i.Namespace, "namespace", "n", i.Namespace, "Kubernetes namespace")
	flags.StringVar(&i.StorageClassesName, "storage-classes-name", i.StorageClassesName, "Kubernetes StorageClasses Name")
	flags.StringVar(&i.KubeConfig, "kubeconfig", i.KubeConfig, "absolute path to the kubeconfig file")
	flags.StringVar(&i.Context, "context", i.Context, "The name of the kubeconfig context to use")
	// etcd
	flags.StringVarP(&i.EtcdStorageMode, "etcd-storage-mode", "", i.EtcdStorageMode,
		fmt.Sprintf("etcd data storage mode(%s). value is PVC, specify --storage-classes-name", strings.Join(SupportedStorageMode(), ",")))
	flags.StringVarP(&i.EtcdImage, "etcd-image", "", i.EtcdImage, "etcd image")
	flags.StringVarP(&i.EtcdInitImage, "etcd-init-image", "", i.EtcdInitImage, "etcd init container image")
	flags.Int32VarP(&i.EtcdReplicas, "etcd-replicas", "", i.EtcdReplicas, "etcd replica set, cluster 3,5...singular")
	flags.StringVarP(&i.EtcdHostDataPath, "etcd-data", "", i.EtcdHostDataPath, "etcd data path,valid in hostPath mode.")
	flags.StringVarP(&i.EtcdNodeSelectorLabels, "etcd-node-selector-labels", "", i.EtcdNodeSelectorLabels, "etcd pod select the labels of the node. valid in hostPath mode ( e.g. --etcd-node-selector-labels karmada.io/etcd=true)")
	flags.StringVarP(&i.EtcdPersistentVolumeSize, "etcd-pvc-size", "", i.EtcdPersistentVolumeSize, "etcd data path,valid in pvc mode.")
	// karmada
	flags.StringVar(&i.CRDs, "crds", i.CRDs, "Karmada crds resource.(local file e.g. --crds /root/crds.tar.gz)")
	flags.Int32VarP(&i.KarmadaAPIServerNodePort, "port", "p", i.KarmadaAPIServerNodePort, "Karmada apiserver service node port")
	flags.StringVarP(&i.KarmadaDataPath, "karmada-data", "d", i.KarmadaDataPath, "karmada data path. kubeconfig cert and crds files")
	flags.StringVarP(&i.KarmadaAPIServerImage, "karmada-apiserver-image", "", i.KarmadaAPIServerImage, "Kubernetes apiserver image")
	flags.Int32VarP(&i.KarmadaAPIServerReplicas, "karmada-apiserver-replicas", "", i.KarmadaAPIServerReplicas, "karmada apiserver replica set")
	flags.StringVarP(&i.KarmadaSchedulerImage, "karmada-scheduler-image", "", i.KarmadaSchedulerImage, "karmada scheduler image")
	flags.Int32VarP(&i.KarmadaSchedulerReplicas, "karmada-scheduler-replicas", "", i.KarmadaSchedulerReplicas, "karmada scheduler replica set")
	flags.StringVarP(&i.KubeControllerManagerImage, "karmada-kube-controller-manager-image", "", i.KubeControllerManagerImage, "Kubernetes controller manager image")
	flags.Int32VarP(&i.KubeControllerManagerReplicas, "karmada-kube-controller-manager-replicas", "", i.KubeControllerManagerReplicas, "karmada kube controller manager replica set")
	flags.StringVarP(&i.KarmadaControllerManagerImage, "karmada-controller-manager-image", "", i.KarmadaControllerManagerImage, "karmada controller manager image")
	flags.Int32VarP(&i.KarmadaControllerManagerReplicas, "karmada-controller-manager-replicas", "", i.KarmadaControllerManagerReplicas, "karmada controller manager replica set")
	flags.StringVarP(&i.KarmadaWebhookImage, "karmada-webhook-image", "", i.KarmadaWebhookImage, "karmada webhook image")
	flags.Int32VarP(&i.KarmadaWebhookReplicas, "karmada-webhook-replicas", "", i.KarmadaWebhookReplicas, "karmada webhook replica set")
	flags.StringVarP(&i.KarmadaAggregatedAPIServerImage, "karmada-aggregated-apiserver-image", "", i.KarmadaAggregatedAPIServerImage, "karmada aggregated apiserver image")
	flags.Int32VarP(&i.KarmadaAggregatedAPIServerReplicas, "karmada-aggregated-apiserver-replicas", "", i.KarmadaAggregatedAPIServerReplicas, "karmada aggregated apiserver replica set")
}
