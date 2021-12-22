package cmdinit

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
	"github.com/karmada-io/karmada/pkg/version"
)

// NewCmdInit init karmada.
func NewCmdInit() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "bootstrap install karmada (default in kubernetes)",
		Long:  `Installation options.`,
		Run: func(cmd *cobra.Command, args []string) {
			kubernetes.Deploy()
		},
		Example: `
# This master IP is any node IP of kubernetes.
kubectl karmada init --master=192.168.1.2

# In the Intranet, CRDs resources specify local files.
kubectl karmada init --master=192.168.1.2 --crd /root/crds.tar.gz

# Private registry can be specified for all images.
kubectl karmada init --master=192.168.1.2 --etcd-image local.registry.com/library/etcd:3.5.1-0

# Deploy highly available(HA) karmada.
kubectl karmada init --master=192.168.1.2,192.168.1.3,192.168.1.4 --karmada-apiserver-replicas 3 --etcd-replicas 3

# Karmada uses external load balancing or HAip, and karmada certificate needs to join the IP.
kubectl karmada init --master=192.168.1.2,192.168.1.3,192.168.1.4 --cert-external-ip 10.235.1.2
`,
	}

	// cert
	cmd.PersistentFlags().StringVar(&options.ExternalIP, "cert-external-ip", "", "the external IP of Karmada certificate (e.g 192.168.1.2,172.16.1.2)")

	// Kubernetes
	cmd.PersistentFlags().StringVarP(&options.Namespace, "namespace", "n", "karmada-system", "Kubernetes namespace")
	cmd.PersistentFlags().StringVar(&options.StorageClassesName, "storage-classes-name", "", "Kubernetes StorageClasses Name")

	// etcd
	cmd.PersistentFlags().StringVarP(&options.EtcdStorageMode, "etcd-storage-mode", "", "emptyDir",
		"etcd data storage mode(emptyDir,hostPath,PVC). value is PVC, specify --storage-classes-name")
	cmd.PersistentFlags().StringVarP(&options.EtcdImage, "etcd-image", "", "k8s.gcr.io/etcd:3.5.1-0", "etcd image")
	cmd.PersistentFlags().StringVarP(&options.EtcdInitImage, "etcd-init-image", "", "docker.io/alpine:3.14.3", "etcd init container image")
	cmd.PersistentFlags().Int32VarP(&options.EtcdReplicas, "etcd-replicas", "", 1, "etcd replica set, cluster 3,5...singular")
	cmd.PersistentFlags().StringVarP(&options.EtcdDataPath, "etcd-data", "", "/var/lib/karmada-etcd", "etcd data path,valid in hostPath mode.")
	cmd.PersistentFlags().StringVarP(&options.EtcdStorageSize, "etcd-storage-size", "", "5Gi", "etcd data path,valid in pvc mode.")

	// karmada
	crdURL := fmt.Sprintf("https://github.com/karmada-io/karmada/releases/download/%s/crds.tar.gz", version.Get().GitVersion)
	cmd.PersistentFlags().StringVar(&options.KarmadaMasterIP, "master", "", "Karmada master ip. (e.g. --master 192.168.1.2,192.168.1.3)")
	cmd.PersistentFlags().StringVar(&options.CRDs, "crds", crdURL, "Karmada crds resource.(local file  e.g. --crds /root/crds.tar.gz)")
	cmd.PersistentFlags().Int32VarP(&options.KarmadaMasterPort, "port", "p", 5443, "Karmada apiserver port")
	cmd.PersistentFlags().StringVarP(&options.DataPath, "karmada-data", "d", "/etc/karmada", "karmada data path. kubeconfig cert and crds files")
	cmd.PersistentFlags().StringVarP(&options.APIServerImage, "karmada-apiserver-image", "", "k8s.gcr.io/kube-apiserver:v1.21.7", "Kubernetes apiserver image")
	cmd.PersistentFlags().Int32VarP(&options.APIServerReplicas, "karmada-apiserver-replicas", "", 1, "karmada apiserver replica set")
	cmd.PersistentFlags().StringVarP(&options.SchedulerImage, "karmada-scheduler-image", "", "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-scheduler:latest", "karmada scheduler image")
	cmd.PersistentFlags().Int32VarP(&options.SchedulerReplicas, "karmada-scheduler-replicas", "", 1, "karmada scheduler replica set")
	cmd.PersistentFlags().StringVarP(&options.KubeControllerManagerImage, "karmada-kube-controller-manager-image", "", "k8s.gcr.io/kube-controller-manager:v1.21.7", "Kubernetes controller manager image")
	cmd.PersistentFlags().Int32VarP(&options.KubeControllerManagerReplicas, "karmada-kube-controller-manager-replicas", "", 1, "karmada kube controller manager replica set")
	cmd.PersistentFlags().StringVarP(&options.ControllerManagerImage, "karmada-controller-manager-image", "", "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-controller-manager:latest", "karmada controller manager  image")
	cmd.PersistentFlags().Int32VarP(&options.ControllerManagerReplicas, "karmada-controller-manager-replicas", "", 1, "karmada controller manager replica set")
	cmd.PersistentFlags().StringVarP(&options.WebhookImage, "karmada-webhook-image", "", "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-webhook:latest", "karmada webhook image")
	cmd.PersistentFlags().Int32VarP(&options.WebhookReplicas, "karmada-webhook-replicas", "", 1, "karmada webhook replica set")
	cmd.PersistentFlags().StringVarP(&options.AggregatedAPIServerImage, "karmada-aggregated-apiserver-image", "", "swr.ap-southeast-1.myhuaweicloud.com/karmada/karmada-aggregated-apiserver:latest", "karmada aggregated apiserver image")
	cmd.PersistentFlags().Int32VarP(&options.AggregatedAPIServerReplicas, "karmada-aggregated-apiserver-replicas", "", 1, "karmada aggregated apiserver replica set")

	options.AddFlags(cmd.Flags())
	return cmd
}
