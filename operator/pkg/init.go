package karmada

import (
	"errors"
	"fmt"
	"net/url"
	"sync"

	corev1 "k8s.io/api/core/v1"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/certs"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	tasks "github.com/karmada-io/karmada/operator/pkg/tasks/init"
	workflow "github.com/karmada-io/karmada/operator/pkg/workflow"
)

var (
	defaultCrdURL = "https://github.com/karmada-io/karmada/releases/download/%s/crds.tar.gz"
)

// InitOptions defines all the init options.
type InitOptions struct {
	Name            string
	Namespace       string
	Kubeconfig      *rest.Config
	KarmadaVersion  string
	CrdRemoteURL    string
	KarmadaDataDir  string
	PrivateRegistry string
	HostCluster     *operatorv1alpha1.HostCluster
	Components      *operatorv1alpha1.KarmadaComponents
	FeatureGates    map[string]bool
}

// InitOpt defines a type of function to set InitOptions values.
type InitOpt func(opt *InitOptions)

var _ tasks.InitData = &initData{}

// initData defines all the runtime information used when ruing init workflow;
// this data is shared across all the tasks tha are included in the workflow.
type initData struct {
	sync.Once
	certs.CertStore
	name                string
	namespace           string
	karmadaVersion      *utilversion.Version
	controlplaneConifig *rest.Config
	remoteClient        clientset.Interface
	karmadaClient       clientset.Interface
	dnsDomain           string
	crdRemoteURL        string
	karmadaDataDir      string
	privateRegistry     string
	featureGates        map[string]bool
	components          *operatorv1alpha1.KarmadaComponents
}

// NewInitJob initializes a job with list of init sub task. and build
// init runData object.
func NewInitJob(opt *InitOptions) *workflow.Job {
	initJob := workflow.NewJob()

	// add the all of tasks to the init job workflow.
	initJob.AppendTask(tasks.NewPrepareCrdsTask())
	initJob.AppendTask(tasks.NewCertTask())
	initJob.AppendTask(tasks.NewNamespaceTask())
	initJob.AppendTask(tasks.NewUploadKubeconfigTask())
	initJob.AppendTask(tasks.NewUploadCertsTask())
	initJob.AppendTask(tasks.NewEtcdTask())
	initJob.AppendTask(tasks.NewKarmadaApiserverTask())
	initJob.AppendTask(tasks.NewWaitApiserverTask())
	initJob.AppendTask(tasks.NewKarmadaResourcesTask())
	initJob.AppendTask(tasks.NewComponentTask())
	initJob.AppendTask(tasks.NewWaitControlPlaneTask())

	initJob.SetDataInitializer(func() (workflow.RunData, error) {
		// if there is no endpoint info, we are consider that the local cluster
		// is remote cluster to install karmada.
		var remoteClient clientset.Interface
		if opt.HostCluster.SecretRef == nil && len(opt.HostCluster.APIEndpoint) == 0 {
			client, err := clientset.NewForConfig(opt.Kubeconfig)
			if err != nil {
				return nil, fmt.Errorf("error when create cluster client to install karmada, err: %w", err)
			}

			remoteClient = client
		}

		if len(opt.Name) == 0 || len(opt.Namespace) == 0 {
			return nil, errors.New("unexpected empty name or namespace")
		}

		version, err := utilversion.ParseGeneric(opt.KarmadaVersion)
		if err != nil {
			return nil, fmt.Errorf("unexpected karmada invalid version %s", opt.KarmadaVersion)
		}

		if len(opt.CrdRemoteURL) > 0 {
			if _, err := url.Parse(opt.CrdRemoteURL); err != nil {
				return nil, fmt.Errorf("unexpected invalid crds remote url %s", opt.CrdRemoteURL)
			}
		}

		// TODO: Verify whether important values of initData is valid

		return &initData{
			name:            opt.Name,
			namespace:       opt.Namespace,
			karmadaVersion:  version,
			remoteClient:    remoteClient,
			crdRemoteURL:    opt.CrdRemoteURL,
			karmadaDataDir:  opt.KarmadaDataDir,
			privateRegistry: opt.PrivateRegistry,
			components:      opt.Components,
			featureGates:    opt.FeatureGates,
			dnsDomain:       *opt.HostCluster.Networking.DNSDomain,
			CertStore:       certs.NewCertStore(),
		}, nil
	})

	return initJob
}

func (data *initData) GetName() string {
	return data.name
}

func (data *initData) GetNamespace() string {
	return data.namespace
}

func (data *initData) RemoteClient() clientset.Interface {
	return data.remoteClient
}

func (data *initData) KarmadaClient() clientset.Interface {
	if data.karmadaClient == nil {
		data.Once.Do(func() {
			client, err := clientset.NewForConfig(data.controlplaneConifig)
			if err != nil {
				klog.Errorf("error when init karmada client, err: %w", err)
			}
			data.karmadaClient = client
		})
	}

	return data.karmadaClient
}

func (data *initData) ControlplaneConifg() *rest.Config {
	return data.controlplaneConifig
}

func (data *initData) SetControlplaneConifg(config *rest.Config) {
	data.controlplaneConifig = config
}

func (data *initData) Components() *operatorv1alpha1.KarmadaComponents {
	return data.components
}

func (data *initData) DataDir() string {
	return data.karmadaDataDir
}

func (data *initData) CrdsRomoteURL() string {
	return data.crdRemoteURL
}

func (data *initData) KarmadaVersion() string {
	return data.karmadaVersion.String()
}

// NewJobOptions calls all of InitOpt func to initialize a InitOptions.
// if there is not InitOpt functions, it will return a default InitOptions.
func NewJobOptions(opts ...InitOpt) *InitOptions {
	options := defaultJobOptions()

	for _, c := range opts {
		c(options)
	}
	return options
}

func defaultJobOptions() *InitOptions {
	return &InitOptions{
		CrdRemoteURL:   fmt.Sprintf(defaultCrdURL, constants.KarmadaDefaultVersion),
		Name:           "karmada",
		Namespace:      constants.KarmadaSystemNamespace,
		KarmadaVersion: constants.KarmadaDefaultVersion,
		KarmadaDataDir: constants.KarmadaDataDir,
		Components:     defaultComponents(),
		HostCluster:    defaultHostCluster(),
	}
}

func defaultHostCluster() *operatorv1alpha1.HostCluster {
	return &operatorv1alpha1.HostCluster{
		Networking: &operatorv1alpha1.Networking{
			DNSDomain: pointer.String("cluster.local"),
		},
	}
}

func defaultComponents() *operatorv1alpha1.KarmadaComponents {
	return &operatorv1alpha1.KarmadaComponents{
		Etcd: &operatorv1alpha1.Etcd{
			Local: &operatorv1alpha1.LocalEtcd{
				Image: operatorv1alpha1.Image{
					ImageRepository: fmt.Sprintf("%s/%s", constants.KubeDefaultRepository, constants.Etcd),
					ImageTag:        constants.EtcdDefaultVersion,
				},
				VolumeData: &operatorv1alpha1.VolumeData{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
		},

		KarmadaAPIServer: &operatorv1alpha1.KarmadaAPIServer{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Replicas: pointer.Int32(1),
				Image: operatorv1alpha1.Image{
					ImageRepository: fmt.Sprintf("%s/%s", constants.KubeDefaultRepository, constants.KarmadaAPIServer),
					ImageTag:        constants.KubeDefaultVersion,
				},
			},
			ServiceSubnet: pointer.String("10.96.0.0/12"),
			ServiceType:   corev1.ServiceTypeClusterIP,
		},
		KarmadaAggregratedAPIServer: &operatorv1alpha1.KarmadaAggregratedAPIServer{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Replicas: pointer.Int32(1),
				Image: operatorv1alpha1.Image{
					ImageRepository: fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, constants.KarmadaAggregatedAPIServer),
					ImageTag:        constants.KarmadaDefaultVersion,
				},
			},
		},
		KubeControllerManager: &operatorv1alpha1.KubeControllerManager{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Replicas: pointer.Int32(1),
				Image: operatorv1alpha1.Image{
					ImageRepository: fmt.Sprintf("%s/%s", constants.KubeDefaultRepository, constants.KubeControllerManager),
					ImageTag:        constants.KubeDefaultVersion,
				},
			},
		},
		KarmadaControllerManager: &operatorv1alpha1.KarmadaControllerManager{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Replicas: pointer.Int32(1),
				Image: operatorv1alpha1.Image{
					ImageRepository: fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, constants.KarmadaControllerManager),
					ImageTag:        constants.KarmadaDefaultVersion,
				},
			},
		},
		KarmadaScheduler: &operatorv1alpha1.KarmadaScheduler{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Replicas: pointer.Int32(1),
				Image: operatorv1alpha1.Image{
					ImageRepository: fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, constants.KarmadaScheduler),
					ImageTag:        constants.KarmadaDefaultVersion,
				},
			},
		},
		KarmadaWebhook: &operatorv1alpha1.KarmadaWebhook{
			CommonSettings: operatorv1alpha1.CommonSettings{
				Replicas: pointer.Int32(1),
				Image: operatorv1alpha1.Image{
					ImageRepository: fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, constants.KarmadaWebhook),
					ImageTag:        constants.KarmadaDefaultVersion,
				},
			},
		},
	}
}
