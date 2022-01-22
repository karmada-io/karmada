package cmdinit

import (
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/docker"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/options"
)

const (
	initShort       = `install karmada.`
	initLong        = `install karmada in kubernetes by default.`
	dockerInitShort = `install karmada in local docker.`
	dockerInitLong  = `install karmada in local docker.`
)

// NewCmdInit install karmada on kubernetes
func NewCmdInit(cmdOut io.Writer, parentCommand string) *cobra.Command {
	opts := kubernetes.CommandInitOption{}
	cmd := &cobra.Command{
		Use:          "init",
		Short:        initShort,
		Long:         initLong,
		Example:      getInitExample(parentCommand),
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
	flags := cmd.Flags()
	options.AddFlags(flags)
	// Kubernetes
	flags.StringVarP(&opts.Namespace, "namespace", "n", "karmada-system", "Kubernetes namespace")
	flags.StringVar(&opts.StorageClassesName, "storage-classes-name", "", "Kubernetes StorageClasses Name")
	flags.StringVar(&opts.KubeConfig, "kubeconfig", filepath.Join(homeDir(), ".kube", "config"), "absolute path to the kubeconfig file")
	// etcd
	flags.StringVarP(&opts.EtcdInitImage, "etcd-init-image", "", "docker.io/alpine:3.14.3", "etcd init container image")
	flags.StringVarP(&opts.EtcdStorageMode, "etcd-storage-mode", "", "emptyDir", "etcd data storage mode(emptyDir,hostPath,PVC). value is PVC, specify --storage-classes-name")
	flags.Int32VarP(&opts.EtcdReplicas, "etcd-replicas", "", 1, "etcd replica set, cluster 3,5...singular")
	flags.StringVarP(&opts.EtcdNodeSelectorLabels, "etcd-node-selector-labels", "", "", "etcd pod select the labels of the node. valid in hostPath mode ( e.g. --etcd-node-selector-labels karmada.io/etcd=true)")
	flags.StringVarP(&opts.EtcdPersistentVolumeSize, "etcd-pvc-size", "", "5Gi", "etcd data path,valid in pvc mode.")
	// karmada
	flags.Int32VarP(&opts.KarmadaAPIServerNodePort, "port", "p", 32443, "Karmada apiserver service node port")
	flags.Int32VarP(&opts.KarmadaAPIServerReplicas, "karmada-apiserver-replicas", "", 1, "karmada apiserver replica set")
	flags.Int32VarP(&opts.KarmadaSchedulerReplicas, "karmada-scheduler-replicas", "", 1, "karmada scheduler replica set")
	flags.Int32VarP(&opts.KubeControllerManagerReplicas, "karmada-kube-controller-manager-replicas", "", 1, "karmada kube controller manager replica set")
	flags.Int32VarP(&opts.KarmadaControllerManagerReplicas, "karmada-controller-manager-replicas", "", 1, "karmada controller manager replica set")
	flags.Int32VarP(&opts.KarmadaWebhookReplicas, "karmada-webhook-replicas", "", 1, "karmada webhook replica set")
	flags.Int32VarP(&opts.KarmadaAggregatedAPIServerReplicas, "karmada-aggregated-apiserver-replicas", "", 1, "karmada aggregated apiserver replica set")

	// add subcommand
	cmd.AddCommand(newCmdDockerInit(cmdOut, parentCommand))
	return cmd
}

// newCmdDockerInit install karmada on docker
func newCmdDockerInit(cmdOut io.Writer, parentCommand string) *cobra.Command {
	opts := docker.CommandInitDockerOption{}
	cmd := &cobra.Command{
		Use:          "docker",
		Short:        dockerInitShort,
		Long:         dockerInitLong,
		Example:      getDockerInitExample(parentCommand),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(parentCommand); err != nil {
				return err
			}
			if err := opts.Complete(); err != nil {
				return err
			}
			if err := opts.RunDockerInit(cmdOut, parentCommand); err != nil {
				return err
			}
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.KarmadaAPIServerHostPort, "port", "p", "32443", "Karmada apiserver service host port")
	flags.StringVarP(&opts.DockerNetwork, "docker-network", "", "karmada", "docker network name")
	flags.StringVarP(&opts.DockerNetworkSubnet, "docker-network-subnet", "", "182.16.0.0/16", "docker network subnet")
	flags.StringVarP(&opts.DockerNetworkGateway, "docker-network-gateway", "", "182.16.0.1", "docker network gateway")
	options.AddFlags(flags)
	return cmd
}

func getInitExample(parentCommand string) string {
	initExample := `
# Install Karmada in Kubernetes cluster.
# The karmada-apiserver binds the master node's IP by default.
` + parentCommand + ` init 

# Specify the URL to download CRD tarball.
` + parentCommand + ` init --crds https://github.com/karmada-io/karmada/releases/download/v0.10.1/crds.tar.gz

# Specify the local CRD tarball.
` + parentCommand + ` init --crds /root/crds.tar.gz

# Use PVC to persistent storage etcd data.
` + parentCommand + ` init --etcd-storage-mode PVC --storage-classes-name {StorageClassesName}

# Use hostPath to persistent storage etcd data. For data security, only 1 etcd pod can run in hostPath mode.
` + parentCommand + ` init --etcd-storage-mode hostPath  --etcd-replicas 1

# Use hostPath to persistent storage etcd data but select nodes by labels.
` + parentCommand + ` init --etcd-storage-mode hostPath --etcd-node-selector-labels karmada.io/etcd=true

# Private registry can be specified for all images.
` + parentCommand + ` init --etcd-image local.registry.com/library/etcd:3.5.1-0

# Deploy highly available(HA) karmada.
` + parentCommand + ` init --karmada-apiserver-replicas 3 --etcd-replicas 3 --storage-classes-name PVC --storage-classes-name {StorageClassesName}

# Specify external IPs(load balancer or HA IP) which used to sign the certificate.
` + parentCommand + ` init --cert-external-ip 10.235.1.2 --cert-external-dns www.karmada.io
`
	return initExample
}

func getDockerInitExample(parentCommand string) string {
	dockerInitExample := `
# Install Karmada in local Docker.
# The karmada-apiserver binds the local ip by default.
` + parentCommand + ` init docker 

# Specify the URL to download CRD tarball.
` + parentCommand + ` init docker --crds https://github.com/karmada-io/karmada/releases/download/v0.10.1/crds.tar.gz

# Specify the local CRD tarball.
` + parentCommand + ` init docker --crds /root/crds.tar.gz

# Private registry can be specified for all images.
` + parentCommand + ` init docker --etcd-image local.registry.com/library/etcd:3.5.1-0`

	return dockerInitExample
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}
