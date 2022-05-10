package cmdinit

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	"github.com/karmada-io/karmada/pkg/version"
)

const (
	initShort = `install karmada in kubernetes.`
	initLong  = `install karmada in kubernetes.`
)

// NewCmdInit install karmada on kubernetes
func NewCmdInit(cmdOut io.Writer, parentCommand string) *cobra.Command {
	opts := kubernetes.CommandInitOption{}
	cmd := &cobra.Command{
		Use:          "init",
		Short:        initShort,
		Long:         initLong,
		Example:      initExample(parentCommand),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(parentCommand); err != nil {
				return err
			}
			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.RunInit(cmdOut, parentCommand); err != nil {
				return err
			}
			return nil
		},
	}
	flags := cmd.PersistentFlags()

	releaseVer, err := version.ParseGitVersion(version.Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", version.Get().String())
		releaseVer = &version.ReleaseVersion{} // initialize to avoid panic
	}

	// cert
	flags.StringVar(&opts.ExternalIP, "cert-external-ip", "", "the external IP of Karmada certificate (e.g 192.168.1.2,172.16.1.2)")
	flags.StringVar(&opts.ExternalDNS, "cert-external-dns", "", "the external DNS of Karmada certificate (e.g localhost,localhost.com)")
	// Kubernetes
	flags.StringVarP(&opts.Namespace, "namespace", "n", "karmada-system", "Kubernetes namespace")
	flags.StringVar(&opts.StorageClassesName, "storage-classes-name", "", "Kubernetes StorageClasses Name")
	flags.StringVar(&opts.KubeConfig, "kubeconfig", "", "absolute path to the kubeconfig file")
	flags.StringVar(&opts.Context, "context", "", "The name of the kubeconfig context to use")
	// etcd
	flags.StringVarP(&opts.EtcdStorageMode, "etcd-storage-mode", "", "emptyDir",
		"etcd data storage mode(emptyDir,hostPath,PVC). value is PVC, specify --storage-classes-name")
	flags.StringVarP(&opts.EtcdImage, "etcd-image", "", "k8s.gcr.io/etcd:3.5.1-0", "etcd image")
	flags.StringVarP(&opts.EtcdInitImage, "etcd-init-image", "", "docker.io/alpine:3.15.1", "etcd init container image")
	flags.Int32VarP(&opts.EtcdReplicas, "etcd-replicas", "", 1, "etcd replica set, cluster 3,5...singular")
	flags.StringVarP(&opts.EtcdHostDataPath, "etcd-data", "", "/var/lib/karmada-etcd", "etcd data path,valid in hostPath mode.")
	flags.StringVarP(&opts.EtcdNodeSelectorLabels, "etcd-node-selector-labels", "", "", "etcd pod select the labels of the node. valid in hostPath mode ( e.g. --etcd-node-selector-labels karmada.io/etcd=true)")
	flags.StringVarP(&opts.EtcdPersistentVolumeSize, "etcd-pvc-size", "", "5Gi", "etcd data path,valid in pvc mode.")
	// karmada
	crdURL := fmt.Sprintf("https://github.com/karmada-io/karmada/releases/download/%s/crds.tar.gz", releaseVer.FirstMinorRelease())
	flags.StringVar(&opts.CRDs, "crds", crdURL, "Karmada crds resource.(local file e.g. --crds /root/crds.tar.gz)")
	flags.Int32VarP(&opts.KarmadaAPIServerNodePort, "port", "p", 32443, "Karmada apiserver service node port")
	flags.StringVarP(&opts.KarmadaDataPath, "karmada-data", "d", "/etc/karmada", "karmada data path. kubeconfig cert and crds files")
	flags.StringVarP(&opts.KarmadaAPIServerImage, "karmada-apiserver-image", "", "k8s.gcr.io/kube-apiserver:v1.21.7", "Kubernetes apiserver image")
	flags.Int32VarP(&opts.KarmadaAPIServerReplicas, "karmada-apiserver-replicas", "", 1, "karmada apiserver replica set")
	flags.StringVarP(&opts.KarmadaSchedulerImage, "karmada-scheduler-image", "", fmt.Sprintf("swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-scheduler:%s", releaseVer.PatchRelease()), "karmada scheduler image")
	flags.Int32VarP(&opts.KarmadaSchedulerReplicas, "karmada-scheduler-replicas", "", 1, "karmada scheduler replica set")
	flags.StringVarP(&opts.KubeControllerManagerImage, "karmada-kube-controller-manager-image", "", "k8s.gcr.io/kube-controller-manager:v1.21.7", "Kubernetes controller manager image")
	flags.Int32VarP(&opts.KubeControllerManagerReplicas, "karmada-kube-controller-manager-replicas", "", 1, "karmada kube controller manager replica set")
	flags.StringVarP(&opts.KarmadaControllerManagerImage, "karmada-controller-manager-image", "", fmt.Sprintf("swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-controller-manager:%s", releaseVer.PatchRelease()), "karmada controller manager  image")
	flags.Int32VarP(&opts.KarmadaControllerManagerReplicas, "karmada-controller-manager-replicas", "", 1, "karmada controller manager replica set")
	flags.StringVarP(&opts.KarmadaWebhookImage, "karmada-webhook-image", "", fmt.Sprintf("swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-webhook:%s", releaseVer.PatchRelease()), "karmada webhook image")
	flags.Int32VarP(&opts.KarmadaWebhookReplicas, "karmada-webhook-replicas", "", 1, "karmada webhook replica set")
	flags.StringVarP(&opts.KarmadaAggregatedAPIServerImage, "karmada-aggregated-apiserver-image", "", fmt.Sprintf("swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-aggregated-apiserver:%s", releaseVer.PatchRelease()), "karmada aggregated apiserver image")
	flags.Int32VarP(&opts.KarmadaAggregatedAPIServerReplicas, "karmada-aggregated-apiserver-replicas", "", 1, "karmada aggregated apiserver replica set")

	return cmd
}

func initExample(parentCommand string) string {
	example := `
# Install Karmada in Kubernetes cluster
# The karmada-apiserver binds the master node's IP by default` + "\n" +
		fmt.Sprintf("%s init", parentCommand) + `

# Specify the URL to download CRD tarball` + "\n" +
		fmt.Sprintf("%s init --crds https://github.com/karmada-io/karmada/releases/download/v1.1.0/crds.tar.gz", parentCommand) + `

# Specify the local CRD tarball` + "\n" +
		fmt.Sprintf("%s init --crds /root/crds.tar.gz", parentCommand) + `

# Use PVC to persistent storage etcd data` + "\n" +
		fmt.Sprintf("%s init --etcd-storage-mode PVC --storage-classes-name {StorageClassesName}", parentCommand) + `

# Use hostPath to persistent storage etcd data. For data security, only 1 etcd pod can run in hostPath mode` + "\n" +
		fmt.Sprintf("%s init --etcd-storage-mode hostPath  --etcd-replicas 1", parentCommand) + `

# Use hostPath to persistent storage etcd data but select nodes by labels` + "\n" +
		fmt.Sprintf("%s init --etcd-storage-mode hostPath --etcd-node-selector-labels karmada.io/etcd=true", parentCommand) + `

# Private registry can be specified for all images` + "\n" +
		fmt.Sprintf("%s init --etcd-image local.registry.com/library/etcd:3.5.1-0", parentCommand) + `

# Deploy highly available(HA) karmada` + "\n" +
		fmt.Sprintf("%s init --karmada-apiserver-replicas 3 --etcd-replicas 3 --storage-classes-name PVC --storage-classes-name {StorageClassesName}", parentCommand) + `
		
# Specify external IPs(load balancer or HA IP) which used to sign the certificate` + "\n" +
		fmt.Sprintf("%s init --cert-external-ip 10.235.1.2 --cert-external-dns www.karmada.io", parentCommand)
	return example
}
