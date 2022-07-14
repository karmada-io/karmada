package cmdinit

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"k8s.io/kubectl/pkg/util/templates"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	"github.com/karmada-io/karmada/pkg/version"
)

const (
	initShort = `Install karmada in kubernetes`
	initLong  = `Install karmada in kubernetes.`
)

var (
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
		
		# Deploy highly available(HA) karmada
		%[1]s init --karmada-apiserver-replicas 3 --etcd-replicas 3 --etcd-storage-mode PVC --storage-classes-name {StorageClassesName}
				
		# Specify external IPs(load balancer or HA IP) which used to sign the certificate
		%[1]s init --cert-external-ip 10.235.1.2 --cert-external-dns www.karmada.io`)
)

// NewCmdInit install karmada on kubernetes
func NewCmdInit(parentCommand string) *cobra.Command {
	opts := kubernetes.NewCommandInitOption()
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
			if err := opts.RunInit(parentCommand); err != nil {
				return err
			}
			return nil
		},
	}
	opts.AddFlags(cmd)
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
