package init

import (
	"github.com/spf13/pflag"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	aggregator "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	"github.com/karmada-io/karmada/pkg/karmadactl/util/apiclient"
)

// GlobalCommandOptions holds the configuration shared by the all sub-commands of `karmadactl`.
type GlobalCommandOptions struct {
	// KubeConfig holds host cluster KUBECONFIG file path.
	KubeConfig string
	Context    string

	// KubeConfig holds karmada control plane KUBECONFIG file path.
	KarmadaConfig  string
	KarmadaContext string

	// Namespace holds the namespace where Karmada components installed
	Namespace string

	// Cluster holds the name of member cluster to enable or disable scheduler estimator
	Cluster string

	KubeClientSet *kubernetes.Clientset

	KarmadaRestConfig *rest.Config

	KarmadaAggregatorClientSet *aggregator.Clientset
}

// AddFlags adds flags to the specified FlagSet.
func (o *GlobalCommandOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVarP(&o.Namespace, "namespace", "n", "karmada-system", "namespace where Karmada components are installed.")
	flags.StringVar(&o.KubeConfig, "kubeconfig", "", "Path to the host cluster kubeconfig file.")
	flags.StringVar(&o.Context, "context", "", "The name of the kubeconfig context to use.")
	flags.StringVar(&o.KarmadaConfig, "karmada-kubeconfig", "/etc/karmada/karmada-apiserver.config", "Path to the karmada control plane kubeconfig file.")
	flags.StringVar(&o.KarmadaContext, "karmada-context", "", "The name of the karmada control plane kubeconfig context to use.")
	flags.StringVarP(&o.Cluster, "cluster", "C", "", "Name of the member cluster that enables or disables the scheduler estimator.")
}

// Complete the conditions required to be able to run list.
func (o *GlobalCommandOptions) Complete() error {
	restConfig, err := apiclient.RestConfig(o.Context, o.KubeConfig)
	if err != nil {
		return err
	}

	o.KubeClientSet, err = apiclient.NewClientSet(restConfig)
	if err != nil {
		return err
	}

	o.KarmadaRestConfig, err = apiclient.RestConfig(o.KarmadaContext, o.KarmadaConfig)
	if err != nil {
		return err
	}

	o.KarmadaAggregatorClientSet, err = apiclient.NewAPIRegistrationClient(o.KarmadaRestConfig)
	if err != nil {
		return err
	}

	return nil
}
