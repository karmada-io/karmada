package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	"github.com/karmada-io/karmada/operator/pkg/constants"
)

var (
	etcdImageRepository                       = fmt.Sprintf("%s/%s", constants.KubeDefaultRepository, constants.Etcd)
	karmadaAPIServiceImageRepository          = fmt.Sprintf("%s/%s", constants.KubeDefaultRepository, constants.KubeAPIServer)
	karmadaAggregatedAPIServerImageRepository = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, constants.KarmadaAggregatedAPIServer)
	kubeControllerManagerImageRepository      = fmt.Sprintf("%s/%s", constants.KubeDefaultRepository, constants.KubeControllerManager)
	karmadaControllerManagerImageRepository   = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, constants.KarmadaControllerManager)
	karmadaSchedulerImageRepository           = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, constants.KarmadaScheduler)
	karmadaWebhookImageRepository             = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, constants.KarmadaWebhook)
	karmadaDeschedulerImageRepository         = fmt.Sprintf("%s/%s", constants.KarmadaDefaultRepository, constants.KarmadaDescheduler)
)

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
		hc.Networking.DNSDomain = pointer.String(constants.KarmadaDefaultDNSDomain)
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
			obj.Etcd.Local.Replicas = pointer.Int32(1)
		}
		if len(obj.Etcd.Local.Image.ImageRepository) == 0 {
			obj.Etcd.Local.Image.ImageRepository = etcdImageRepository
		}
		if len(obj.Etcd.Local.Image.ImageTag) == 0 {
			obj.Etcd.Local.Image.ImageTag = constants.EtcdDefaultVersion
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
	if apiserver.Replicas == nil {
		apiserver.Replicas = pointer.Int32(1)
	}
	if apiserver.ServiceSubnet == nil {
		apiserver.ServiceSubnet = pointer.String(constants.KarmadaDefaultServiceSubnet)
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
		aggregated.Image.ImageTag = constants.KarmadaDefaultVersion
	}
	if aggregated.Replicas == nil {
		aggregated.Replicas = pointer.Int32(1)
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
	if kubeControllerManager.Replicas == nil {
		kubeControllerManager.Replicas = pointer.Int32(1)
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
		karmadaControllerManager.Image.ImageTag = constants.KarmadaDefaultVersion
	}
	if karmadaControllerManager.Replicas == nil {
		karmadaControllerManager.Replicas = pointer.Int32(1)
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
		scheduler.Image.ImageTag = constants.KarmadaDefaultVersion
	}
	if scheduler.Replicas == nil {
		scheduler.Replicas = pointer.Int32(1)
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
		webhook.Image.ImageTag = constants.KarmadaDefaultVersion
	}
	if webhook.Replicas == nil {
		webhook.Replicas = pointer.Int32(1)
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
		descheduler.Image.ImageTag = constants.KarmadaDefaultVersion
	}
	if descheduler.Replicas == nil {
		descheduler.Replicas = pointer.Int32(1)
	}
}
