package cmdinit

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/version"
)

var (
	initLong = templates.LongDesc(`
		Install the Karmada control plane in a Kubernetes cluster.

		By default, the images and CRD tarball are downloaded remotely.
		For offline installation, you can set '--private-image-registry' and '--crds'.`)

	initExamples = templates.Examples(`
		# Install Karmada in Kubernetes cluster
		# The karmada-apiserver binds the master node's IP by default
		%[1]s init

		# China mainland registry mirror can be specified by using kube-image-mirror-country
		%[1]s init --kube-image-mirror-country=cn

		# Kube registry can be specified by using kube-image-registry
		%[1]s init --kube-image-registry=registry.cn-hangzhou.aliyuncs.com/google_containers

		# Specify the URL to download CRD tarball
		%[1]s init --crds https://github.com/karmada-io/karmada/releases/download/%[2]s/crds.tar.gz

		# Specify the local CRD tarball
		%[1]s init --crds /root/crds.tar.gz

		# Use PVC to persistent storage etcd data
		%[1]s init --etcd-storage-mode PVC --storage-classes-name {StorageClassesName}

		# Use hostPath to persistent storage etcd data. For data security, only 1 etcd pod can run in hostPath mode
		%[1]s init --etcd-storage-mode hostPath  --etcd-replicas 1

		# Use hostPath to persistent storage etcd data but select nodes by labels
		%[1]s init --etcd-storage-mode hostPath --etcd-node-selector-labels karmada.io/etcd=true

		# Private registry can be specified for all images
		%[1]s init --etcd-image local.registry.com/library/etcd:3.5.3-0

		# Specify Karmada API Server IP address. If not set, the address on the master node will be used.
		%[1]s init --karmada-apiserver-advertise-address 192.168.1.2

		# Deploy highly available(HA) karmada
		%[1]s init --karmada-apiserver-replicas 3 --etcd-replicas 3 --etcd-storage-mode PVC --storage-classes-name {StorageClassesName}

		# Specify external IPs(load balancer or HA IP) which used to sign the certificate
		%[1]s init --cert-external-ip 10.235.1.2 --cert-external-dns www.karmada.io`)
)

// NewCmdInit install Karmada on Kubernetes
func NewCmdInit(parentCommand string) *cobra.Command {
	opts := kubernetes.CommandInitOption{}
	cmd := &cobra.Command{
		Use:                   "init",
		Short:                 "Install the Karmada control plane in a Kubernetes cluster",
		Long:                  initLong,
		Example:               initExample(parentCommand),
		SilenceUsage:          true,
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(parentCommand); err != nil {
				return err
			}
			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.RunInit(parentCommand); err != nil {
				return err
			}
			return nil
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
		Annotations: map[string]string{
			util.TagCommandGroup: util.GroupClusterRegistration,
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.ImageRegistry, "private-image-registry", "", "", "Private image registry where pull images from. If set, all required images will be downloaded from it, it would be useful in offline installation scenarios.  In addition, you still can use --kube-image-registry to specify the registry for Kubernetes's images.")
	// kube image registry
	flags.StringVarP(&opts.KubeImageMirrorCountry, "kube-image-mirror-country", "", "", "Country code of the kube image registry to be used. For Chinese mainland users, set it to cn")
	flags.StringVarP(&opts.KubeImageRegistry, "kube-image-registry", "", "", "Kube image registry. For Chinese mainland users, you may use local gcr.io mirrors such as registry.cn-hangzhou.aliyuncs.com/google_containers to override default kube image registry")
	// cert
	flags.StringVar(&opts.ExternalIP, "cert-external-ip", "", "the external IP of Karmada certificate (e.g 192.168.1.2,172.16.1.2)")
	flags.StringVar(&opts.ExternalDNS, "cert-external-dns", "", "the external DNS of Karmada certificate (e.g localhost,localhost.com)")
	// Kubernetes
	flags.StringVarP(&opts.Namespace, "namespace", "n", "karmada-system", "Kubernetes namespace")
	flags.StringVar(&opts.StorageClassesName, "storage-classes-name", "", "Kubernetes StorageClasses Name")
	flags.StringVar(&opts.KubeConfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flags.StringVar(&opts.Context, "context", "", "The name of the kubeconfig context to use")
	// etcd
	flags.StringVarP(&opts.EtcdStorageMode, "etcd-storage-mode", "", "hostPath",
		fmt.Sprintf("etcd data storage mode(%s). value is PVC, specify --storage-classes-name", strings.Join(kubernetes.SupportedStorageMode(), ",")))
	flags.StringVarP(&opts.EtcdImage, "etcd-image", "", "", "etcd image")
	flags.StringVarP(&opts.EtcdInitImage, "etcd-init-image", "", kubernetes.DefaultInitImage, "etcd init container image")
	flags.Int32VarP(&opts.EtcdReplicas, "etcd-replicas", "", 1, "etcd replica set, cluster 3,5...singular")
	flags.StringVarP(&opts.EtcdHostDataPath, "etcd-data", "", "/var/lib/karmada-etcd", "etcd data path,valid in hostPath mode.")
	flags.StringVarP(&opts.EtcdNodeSelectorLabels, "etcd-node-selector-labels", "", "", "etcd pod select the labels of the node. valid in hostPath mode ( e.g. --etcd-node-selector-labels karmada.io/etcd=true)")
	flags.StringVarP(&opts.EtcdPersistentVolumeSize, "etcd-pvc-size", "", "5Gi", "etcd data path,valid in pvc mode.")
	// karmada
	flags.StringVar(&opts.CRDs, "crds", kubernetes.DefaultCrdURL, "Karmada crds resource.(local file e.g. --crds /root/crds.tar.gz)")
	flags.StringVarP(&opts.KarmadaAPIServerAdvertiseAddress, "karmada-apiserver-advertise-address", "", "", "The IP address the Karmada API Server will advertise it's listening on. If not set, the address on the master node will be used.")
	flags.Int32VarP(&opts.KarmadaAPIServerNodePort, "port", "p", 32443, "Karmada apiserver service node port")
	flags.StringVarP(&opts.KarmadaDataPath, "karmada-data", "d", "/etc/karmada", "Karmada data path. kubeconfig cert and crds files")
	flags.StringVarP(&opts.KarmadaPkiPath, "karmada-pki", "", "/etc/karmada/pki", "Karmada pki path. Karmada cert files")
	flags.StringVarP(&opts.KarmadaAPIServerImage, "karmada-apiserver-image", "", "", "Kubernetes apiserver image")
	flags.Int32VarP(&opts.KarmadaAPIServerReplicas, "karmada-apiserver-replicas", "", 1, "Karmada apiserver replica set")
	flags.StringVarP(&opts.KarmadaSchedulerImage, "karmada-scheduler-image", "", kubernetes.DefaultKarmadaSchedulerImage, "Karmada scheduler image")
	flags.Int32VarP(&opts.KarmadaSchedulerReplicas, "karmada-scheduler-replicas", "", 1, "Karmada scheduler replica set")
	flags.StringVarP(&opts.KubeControllerManagerImage, "karmada-kube-controller-manager-image", "", "", "Kubernetes controller manager image")
	flags.Int32VarP(&opts.KubeControllerManagerReplicas, "karmada-kube-controller-manager-replicas", "", 1, "Karmada kube controller manager replica set")
	flags.StringVarP(&opts.KarmadaControllerManagerImage, "karmada-controller-manager-image", "", kubernetes.DefaultKarmadaControllerManagerImage, "Karmada controller manager image")
	flags.Int32VarP(&opts.KarmadaControllerManagerReplicas, "karmada-controller-manager-replicas", "", 1, "Karmada controller manager replica set")
	flags.StringVarP(&opts.KarmadaWebhookImage, "karmada-webhook-image", "", kubernetes.DefualtKarmadaWebhookImage, "Karmada webhook image")
	flags.Int32VarP(&opts.KarmadaWebhookReplicas, "karmada-webhook-replicas", "", 1, "Karmada webhook replica set")
	flags.StringVarP(&opts.KarmadaAggregatedAPIServerImage, "karmada-aggregated-apiserver-image", "", kubernetes.DefaultKarmadaAggregatedAPIServerImage, "Karmada aggregated apiserver image")
	flags.Int32VarP(&opts.KarmadaAggregatedAPIServerReplicas, "karmada-aggregated-apiserver-replicas", "", 1, "Karmada aggregated apiserver replica set")

	return cmd
}

func initExample(parentCommand string) string {
	releaseVer, err := version.ParseGitVersion(version.Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", version.Get().String())
		releaseVer = &version.ReleaseVersion{}
	}
	return fmt.Sprintf(initExamples, parentCommand, releaseVer.FirstMinorRelease())
}
