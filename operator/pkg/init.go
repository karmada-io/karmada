/*
Copyright 2023 The Karmada Authors.

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

package karmada

import (
	"errors"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/certs"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	operatorscheme "github.com/karmada-io/karmada/operator/pkg/scheme"
	tasks "github.com/karmada-io/karmada/operator/pkg/tasks/init"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/workflow"
)

var (
	defaultCrdURL = "https://github.com/karmada-io/karmada/releases/download/%s/crds.tar.gz"
)

// InitOptions defines all the init workflow options.
type InitOptions struct {
	Name                    string
	Namespace               string
	Kubeconfig              *rest.Config
	KarmadaVersion          string
	CRDTarball              operatorv1alpha1.CRDTarball
	CustomCertificateConfig operatorv1alpha1.CustomCertificate
	KarmadaDataDir          string
	Karmada                 *operatorv1alpha1.Karmada
}

// Validate is used to validate the initOptions before creating initJob.
func (opt *InitOptions) Validate() error {
	if len(opt.Name) == 0 || len(opt.Namespace) == 0 {
		return errors.New("unexpected empty name or namespace")
	}

	_, err := utilversion.ParseGeneric(opt.KarmadaVersion)
	if err != nil {
		return fmt.Errorf("unexpected karmada invalid version %s", opt.KarmadaVersion)
	}

	return nil
}

// InitOpt defines a type of function to set InitOptions values.
type InitOpt func(o *InitOptions)

var _ tasks.InitData = &initData{}

// initData defines all the runtime information used when ruing init workflow;
// this data is shared across all the tasks that are included in the workflow.
type initData struct {
	sync.Once
	certs.CertStore
	name                    string
	namespace               string
	karmadaVersion          *utilversion.Version
	controlplaneConfig      *rest.Config
	controlplaneAddress     string
	remoteClient            clientset.Interface
	karmadaClient           clientset.Interface
	dnsDomain               string
	CRDTarball              operatorv1alpha1.CRDTarball
	CustomCertificateConfig operatorv1alpha1.CustomCertificate
	karmadaDataDir          string
	privateRegistry         string
	featureGates            map[string]bool
	components              *operatorv1alpha1.KarmadaComponents
}

// NewInitJob initializes a job with list of init sub-task. and build
// init runData object.
func NewInitJob(opt *InitOptions) *workflow.Job {
	initJob := workflow.NewJob()

	// add the all tasks to the init job workflow.
	initJob.AppendTask(tasks.NewPrepareCrdsTask())
	initJob.AppendTask(tasks.NewCertTask(opt.Karmada))
	initJob.AppendTask(tasks.NewNamespaceTask())
	initJob.AppendTask(tasks.NewUploadCertsTask(opt.Karmada))

	etcdConfig := opt.Karmada.Spec.Components.Etcd
	// Only required if local etcd is configured
	if etcdConfig.Local != nil {
		initJob.AppendTask(tasks.NewEtcdTask())
	}

	initJob.AppendTask(tasks.NewKarmadaApiserverTask())
	initJob.AppendTask(tasks.NewUploadKubeconfigTask())
	initJob.AppendTask(tasks.NewKarmadaAggregatedApiserverTask())
	initJob.AppendTask(tasks.NewCheckApiserverHealthTask())
	initJob.AppendTask(tasks.NewKarmadaResourcesTask())
	initJob.AppendTask(tasks.NewRBACTask())
	initJob.AppendTask(tasks.NewComponentTask())
	initJob.AppendTask(tasks.NewWaitControlPlaneTask())

	initJob.SetDataInitializer(func() (workflow.RunData, error) {
		return newRunData(opt)
	})

	return initJob
}

func newRunData(opt *InitOptions) (*initData, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	localClusterClient, err := clientset.NewForConfig(opt.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("error when creating local cluster client, err: %w", err)
	}

	// if there is no endpoint message, we are consider that the local cluster
	// is remote cluster to install karmada.
	var remoteClient clientset.Interface
	if util.IsInCluster(opt.Karmada.Spec.HostCluster) {
		remoteClient = localClusterClient
	} else {
		remoteClient, err = util.BuildClientFromSecretRef(localClusterClient, opt.Karmada.Spec.HostCluster.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("error when creating cluster client to install karmada, err: %w", err)
		}
	}

	var privateRegistry string
	if opt.Karmada.Spec.PrivateRegistry != nil {
		privateRegistry = opt.Karmada.Spec.PrivateRegistry.Registry
	}

	version, err := utilversion.ParseGeneric(opt.KarmadaVersion)
	if err != nil {
		return nil, fmt.Errorf("unexpected karmada invalid version %s", opt.KarmadaVersion)
	}

	// TODO: Verify whether important values of initData is valid
	var address string
	if opt.Karmada.Spec.Components.KarmadaAPIServer.ServiceType == corev1.ServiceTypeNodePort {
		address, err = util.GetAPIServiceIP(remoteClient)
		if err != nil {
			return nil, fmt.Errorf("failed to get a valid node IP for APIServer, err: %w", err)
		}
	}

	return &initData{
		name:                    opt.Name,
		namespace:               opt.Namespace,
		karmadaVersion:          version,
		controlplaneAddress:     address,
		remoteClient:            remoteClient,
		CRDTarball:              opt.CRDTarball,
		CustomCertificateConfig: opt.CustomCertificateConfig,
		karmadaDataDir:          opt.KarmadaDataDir,
		privateRegistry:         privateRegistry,
		components:              opt.Karmada.Spec.Components,
		featureGates:            opt.Karmada.Spec.FeatureGates,
		dnsDomain:               *opt.Karmada.Spec.HostCluster.Networking.DNSDomain,
		CertStore:               certs.NewCertStore(),
	}, nil
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
			client, err := clientset.NewForConfig(data.controlplaneConfig)
			if err != nil {
				klog.Errorf("error when init karmada client, err: %v", err)
			}
			data.karmadaClient = client
		})
	}

	return data.karmadaClient
}

func (data *initData) ControlplaneConfig() *rest.Config {
	return data.controlplaneConfig
}

func (data *initData) SetControlplaneConfig(config *rest.Config) {
	data.controlplaneConfig = config
}

func (data *initData) Components() *operatorv1alpha1.KarmadaComponents {
	return data.components
}

func (data *initData) DataDir() string {
	return data.karmadaDataDir
}

func (data *initData) CrdTarball() operatorv1alpha1.CRDTarball {
	return data.CRDTarball
}

func (data *initData) CustomCertificate() operatorv1alpha1.CustomCertificate {
	return data.CustomCertificateConfig
}

func (data *initData) KarmadaVersion() string {
	return data.karmadaVersion.String()
}

func (data *initData) ControlplaneAddress() string {
	return data.controlplaneAddress
}

func (data *initData) FeatureGates() map[string]bool {
	return data.featureGates
}

// NewJobInitOptions calls all of InitOpt func to initialize a InitOptions.
// if there is not InitOpt functions, it will return a default InitOptions.
func NewJobInitOptions(opts ...InitOpt) *InitOptions {
	options := defaultJobInitOptions()

	for _, c := range opts {
		c(options)
	}
	return options
}

func defaultJobInitOptions() *InitOptions {
	karmada := &operatorv1alpha1.Karmada{}

	// set defaults for karmada.
	operatorscheme.Scheme.Default(karmada)

	defaultDownloadPolicy := operatorv1alpha1.DownloadIfNotPresent
	return &InitOptions{
		CRDTarball: operatorv1alpha1.CRDTarball{
			CRDDownloadPolicy: &defaultDownloadPolicy,
			HTTPSource: &operatorv1alpha1.HTTPSource{
				URL: fmt.Sprintf(defaultCrdURL, operatorv1alpha1.DefaultKarmadaImageVersion),
			},
		},
		KarmadaVersion: operatorv1alpha1.DefaultKarmadaImageVersion,
		KarmadaDataDir: constants.KarmadaDataDir,
		Karmada:        karmada,
	}
}

// NewInitOptWithKarmada returns a InitOpt function to initialize InitOptions with karmada resource
func NewInitOptWithKarmada(karmada *operatorv1alpha1.Karmada) InitOpt {
	return func(o *InitOptions) {
		o.Karmada = karmada
		o.Name = karmada.GetName()
		o.Namespace = karmada.GetNamespace()
		if karmada.Spec.CRDTarball != nil {
			o.CRDTarball = *karmada.Spec.CRDTarball
		}
		if karmada.Spec.CustomCertificate != nil {
			o.CustomCertificateConfig = *karmada.Spec.CustomCertificate
		}
	}
}

// NewInitOptWithKubeconfig returns a InitOpt function to set kubeconfig to InitOptions with rest config
func NewInitOptWithKubeconfig(config *rest.Config) InitOpt {
	return func(o *InitOptions) {
		o.Kubeconfig = config
	}
}
