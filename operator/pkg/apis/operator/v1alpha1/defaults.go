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

package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/pkg/util/names"
	"github.com/karmada-io/karmada/pkg/version"
)

var (
	// DefaultKarmadaImageVersion defines the default of the karmada components image tag
	DefaultKarmadaImageVersion                string
	etcdImageRepository                       = fmt.Sprintf("%s/%s", constants.KubeDefaultRepository, constants.Etcd)
	karmadaAPIServiceImageRepository          = fmt.Sprintf("%s/%s", constants.KubeDefaultRepository, constants.KubeAPIServer)
	karmadaAggregatedAPIServerImageRepository = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, names.KarmadaAggregatedAPIServerComponentName)
	kubeControllerManagerImageRepository      = fmt.Sprintf("%s/%s", constants.KubeDefaultRepository, constants.KubeControllerManager)
	karmadaControllerManagerImageRepository   = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, names.KarmadaControllerManagerComponentName)
	karmadaSchedulerImageRepository           = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, names.KarmadaSchedulerComponentName)
	karmadaWebhookImageRepository             = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, names.KarmadaWebhookComponentName)
	karmadaDeschedulerImageRepository         = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, names.KarmadaDeschedulerComponentName)
	karmadaMetricsAdapterImageRepository      = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, names.KarmadaMetricsAdapterComponentName)
	karmadaSearchImageRepository              = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, names.KarmadaSearchComponentName)
)

func init() {
	releaseVer, err := version.ParseGitVersion(version.Get().GitVersion)
	if err != nil {
		klog.Infof("No default release version found. build version: %s", version.Get().String())
		releaseVer = &version.ReleaseVersion{} // initialize to avoid panic
	}
	DefaultKarmadaImageVersion = releaseVer.ReleaseVersion()
	klog.Infof("default Karmada Image Version: %s", DefaultKarmadaImageVersion)
}

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// RegisterDefaults adds defaulters functions to the given scheme.
// Public to allow building arbitrary schemes.
// All generated defaulters are covering - they call all nested defaulters.
func RegisterDefaults(scheme *runtime.Scheme) error {
	scheme.AddTypeDefaultingFunc(&Karmada{}, func(obj interface{}) { SetObjectDefaultsKarmada(obj.(*Karmada)) })
	return nil
}

// SetObjectDefaultsKarmada set defaults for karmada
func SetObjectDefaultsKarmada(in *Karmada) {
	setDefaultsKarmada(in)
}

func setDefaultsKarmada(obj *Karmada) {
	setDefaultsHostCluster(obj)
	setDefaultsKarmadaComponents(obj)
}

func setDefaultsKarmadaComponents(obj *Karmada) {
	if obj.Spec.Components == nil {
		obj.Spec.Components = &KarmadaComponents{}
	}

	setDefaultsEtcd(obj.Spec.Components)
	setDefaultsKarmadaAPIServer(obj.Spec.Components)
	setDefaultsKarmadaAggregatedAPIServer(obj.Spec.Components)
	setDefaultsKubeControllerManager(obj.Spec.Components)
	setDefaultsKarmadaControllerManager(obj.Spec.Components)
	setDefaultsKarmadaScheduler(obj.Spec.Components)
	setDefaultsKarmadaWebhook(obj.Spec.Components)
	setDefaultsKarmadaMetricsAdapter(obj.Spec.Components)
	setDefaultsKarmadaSearch(obj.Spec.Components)
	// set addon defaults
	setDefaultsKarmadaDescheduler(obj.Spec.Components)
}

func setDefaultsHostCluster(obj *Karmada) {
	if obj.Spec.HostCluster == nil {
		obj.Spec.HostCluster = &HostCluster{}
	}

	hc := obj.Spec.HostCluster
	if hc.Networking == nil {
		hc.Networking = &Networking{}
	}
	if hc.Networking.DNSDomain == nil {
		hc.Networking.DNSDomain = ptr.To[string](constants.KarmadaDefaultDNSDomain)
	}
}

func setDefaultsEtcd(obj *KarmadaComponents) {
	if obj.Etcd == nil {
		obj.Etcd = &Etcd{}
	}

	if obj.Etcd.External == nil {
		if obj.Etcd.Local == nil {
			obj.Etcd.Local = &LocalEtcd{}
		}

		if obj.Etcd.Local.Replicas == nil {
			obj.Etcd.Local.Replicas = ptr.To[int32](1)
		}
		if len(obj.Etcd.Local.Image.ImageRepository) == 0 {
			obj.Etcd.Local.Image.ImageRepository = etcdImageRepository
		}
		if len(obj.Etcd.Local.Image.ImageTag) == 0 {
			obj.Etcd.Local.Image.ImageTag = constants.EtcdDefaultVersion
		}
		if len(obj.Etcd.Local.ImagePullPolicy) == 0 {
			obj.Etcd.Local.ImagePullPolicy = corev1.PullIfNotPresent
		}
		if obj.Etcd.Local.VolumeData == nil {
			obj.Etcd.Local.VolumeData = &VolumeData{}
		}
		if obj.Etcd.Local.VolumeData.EmptyDir == nil && obj.Etcd.Local.VolumeData.HostPath == nil && obj.Etcd.Local.VolumeData.VolumeClaim == nil {
			obj.Etcd.Local.VolumeData.EmptyDir = &corev1.EmptyDirVolumeSource{}
		}
	}
}

func setDefaultsKarmadaAPIServer(obj *KarmadaComponents) {
	if obj.KarmadaAPIServer == nil {
		obj.KarmadaAPIServer = &KarmadaAPIServer{}
	}

	apiserver := obj.KarmadaAPIServer
	if len(apiserver.Image.ImageRepository) == 0 {
		apiserver.Image.ImageRepository = karmadaAPIServiceImageRepository
	}
	if len(apiserver.Image.ImageTag) == 0 {
		apiserver.Image.ImageTag = constants.KubeDefaultVersion
	}
	if len(apiserver.ImagePullPolicy) == 0 {
		apiserver.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if apiserver.Replicas == nil {
		apiserver.Replicas = ptr.To[int32](1)
	}
	if apiserver.ServiceSubnet == nil {
		apiserver.ServiceSubnet = ptr.To[string](constants.KarmadaDefaultServiceSubnet)
	}
	if len(apiserver.ServiceType) == 0 {
		apiserver.ServiceType = corev1.ServiceTypeClusterIP
	}
}

func setDefaultsKarmadaAggregatedAPIServer(obj *KarmadaComponents) {
	if obj.KarmadaAggregatedAPIServer == nil {
		obj.KarmadaAggregatedAPIServer = &KarmadaAggregatedAPIServer{}
	}

	aggregated := obj.KarmadaAggregatedAPIServer
	if len(aggregated.Image.ImageRepository) == 0 {
		aggregated.Image.ImageRepository = karmadaAggregatedAPIServerImageRepository
	}
	if len(aggregated.Image.ImageTag) == 0 {
		aggregated.Image.ImageTag = DefaultKarmadaImageVersion
	}
	if len(aggregated.ImagePullPolicy) == 0 {
		aggregated.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if aggregated.Replicas == nil {
		aggregated.Replicas = ptr.To[int32](1)
	}
}

func setDefaultsKubeControllerManager(obj *KarmadaComponents) {
	if obj.KubeControllerManager == nil {
		obj.KubeControllerManager = &KubeControllerManager{}
	}

	kubeControllerManager := obj.KubeControllerManager
	if len(kubeControllerManager.Image.ImageRepository) == 0 {
		kubeControllerManager.Image.ImageRepository = kubeControllerManagerImageRepository
	}
	if len(kubeControllerManager.Image.ImageTag) == 0 {
		kubeControllerManager.Image.ImageTag = constants.KubeDefaultVersion
	}
	if len(kubeControllerManager.ImagePullPolicy) == 0 {
		kubeControllerManager.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if kubeControllerManager.Replicas == nil {
		kubeControllerManager.Replicas = ptr.To[int32](1)
	}
}

func setDefaultsKarmadaControllerManager(obj *KarmadaComponents) {
	if obj.KarmadaControllerManager == nil {
		obj.KarmadaControllerManager = &KarmadaControllerManager{}
	}

	karmadaControllerManager := obj.KarmadaControllerManager
	if len(karmadaControllerManager.Image.ImageRepository) == 0 {
		karmadaControllerManager.Image.ImageRepository = karmadaControllerManagerImageRepository
	}
	if len(karmadaControllerManager.Image.ImageTag) == 0 {
		karmadaControllerManager.Image.ImageTag = DefaultKarmadaImageVersion
	}
	if len(karmadaControllerManager.ImagePullPolicy) == 0 {
		karmadaControllerManager.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if karmadaControllerManager.Replicas == nil {
		karmadaControllerManager.Replicas = ptr.To[int32](1)
	}
}

func setDefaultsKarmadaScheduler(obj *KarmadaComponents) {
	if obj.KarmadaScheduler == nil {
		obj.KarmadaScheduler = &KarmadaScheduler{}
	}

	scheduler := obj.KarmadaScheduler
	if len(scheduler.Image.ImageRepository) == 0 {
		scheduler.Image.ImageRepository = karmadaSchedulerImageRepository
	}
	if len(scheduler.Image.ImageTag) == 0 {
		scheduler.Image.ImageTag = DefaultKarmadaImageVersion
	}
	if len(scheduler.ImagePullPolicy) == 0 {
		scheduler.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if scheduler.Replicas == nil {
		scheduler.Replicas = ptr.To[int32](1)
	}
}

func setDefaultsKarmadaWebhook(obj *KarmadaComponents) {
	if obj.KarmadaWebhook == nil {
		obj.KarmadaWebhook = &KarmadaWebhook{}
	}

	webhook := obj.KarmadaWebhook
	if len(webhook.Image.ImageRepository) == 0 {
		webhook.Image.ImageRepository = karmadaWebhookImageRepository
	}
	if len(webhook.Image.ImageTag) == 0 {
		webhook.Image.ImageTag = DefaultKarmadaImageVersion
	}
	if len(webhook.ImagePullPolicy) == 0 {
		webhook.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if webhook.Replicas == nil {
		webhook.Replicas = ptr.To[int32](1)
	}
}

func setDefaultsKarmadaSearch(obj *KarmadaComponents) {
	if obj.KarmadaSearch == nil {
		return
	}

	search := obj.KarmadaSearch
	if len(search.Image.ImageRepository) == 0 {
		search.Image.ImageRepository = karmadaSearchImageRepository
	}
	if len(search.Image.ImageTag) == 0 {
		search.Image.ImageTag = DefaultKarmadaImageVersion
	}
	if len(search.ImagePullPolicy) == 0 {
		search.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if search.Replicas == nil {
		search.Replicas = ptr.To[int32](1)
	}
}

func setDefaultsKarmadaDescheduler(obj *KarmadaComponents) {
	if obj.KarmadaDescheduler == nil {
		return
	}

	descheduler := obj.KarmadaDescheduler
	if len(descheduler.Image.ImageRepository) == 0 {
		descheduler.Image.ImageRepository = karmadaDeschedulerImageRepository
	}
	if len(descheduler.Image.ImageTag) == 0 {
		descheduler.Image.ImageTag = DefaultKarmadaImageVersion
	}
	if len(descheduler.ImagePullPolicy) == 0 {
		descheduler.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if descheduler.Replicas == nil {
		descheduler.Replicas = ptr.To[int32](1)
	}
}

func setDefaultsKarmadaMetricsAdapter(obj *KarmadaComponents) {
	if obj.KarmadaMetricsAdapter == nil {
		obj.KarmadaMetricsAdapter = &KarmadaMetricsAdapter{}
	}

	metricsAdapter := obj.KarmadaMetricsAdapter
	if len(metricsAdapter.Image.ImageRepository) == 0 {
		metricsAdapter.Image.ImageRepository = karmadaMetricsAdapterImageRepository
	}
	if len(metricsAdapter.Image.ImageTag) == 0 {
		metricsAdapter.Image.ImageTag = DefaultKarmadaImageVersion
	}
	if len(metricsAdapter.ImagePullPolicy) == 0 {
		metricsAdapter.ImagePullPolicy = corev1.PullIfNotPresent
	}
	if metricsAdapter.Replicas == nil {
		metricsAdapter.Replicas = ptr.To[int32](2)
	}
}
