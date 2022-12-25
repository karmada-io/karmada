package v1alpha1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/validation"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/klog/v2"
	"k8s.io/utils/pointer"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// SetupWebhookWithManager new a webhook for karmada resource and register to manager.
func (karmada *Karmada) SetupWebhookWithManager(manager controllerruntime.Manager) error {
	return controllerruntime.NewWebhookManagedBy(manager).
		For(karmada).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update,path=/mutate-operator-karmada-io-v1alpha1-karmada,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.karmada.io,resources=karmadas,versions=v1alpha1,name=default.karmada.operator.karmada.io,admissionReviewVersions=v1alpha1

var _ webhook.Defaulter = &Karmada{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (karmada *Karmada) Default() {
	klog.V(2).Infof("Populating defaults for karmada(%s/%s)", karmada.GetName(), karmada.GetNamespace())

	if karmada.Spec.Components == nil {
		karmada.Spec.Components = &KarmadaComponents{}
	}

	populateDefaultsETCD(karmada)
	populateDefaultsKarmadaAPIServer(karmada)
	populateDefaultsKarmadaAggregratedAPIServer(karmada)
	populateDefaultsKubeControllerManager(karmada)
	populateDefaultsKarmadaControllerManager(karmada)
	populateDefaultsKarmadaScheduler(karmada)
	populateDefaultsKarmadaWebhook(karmada)
}

// populateDefaultsETCD populate defaults to etcd, if etcd is not set, be considered to
// install a local etcd in cluster. we set the etcd storage mode to emptydir temporarily,
// it will change to "pvc" or "HostPath" after completing ectd HA.
func populateDefaultsETCD(karmada *Karmada) {
	etcd := karmada.Spec.Components.Etcd
	if etcd == nil {
		etcd = &Etcd{}
	}
	if etcd.External == nil {
		if etcd.Local == nil {
			etcd.Local = &LocalEtcd{}
		}

		if len(etcd.Local.Image.ImageRepository) == 0 {
			etcd.Local.Image.ImageRepository = calculateImageRepository(karmada.Spec.PrivateRegistry, "registry.k8s.io", "etcd")
		}

		if len(etcd.Local.ImageTag) == 0 {
			etcd.Local.ImageTag = "3.5.3-0"
		}

		if etcd.Local.VolumeData == nil {
			etcd.Local.VolumeData = &VolumeData{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			}
		}
	}
	karmada.Spec.Components.Etcd = etcd
}

// populateDefaultsKarmadaAPIServer populate defaults to karmada APIserver.
// if the serviceSubnet is not be set, it's "10.96.0.0/12" by default.
func populateDefaultsKarmadaAPIServer(karmada *Karmada) {
	apiserver := karmada.Spec.Components.KarmadaAPIServer
	if apiserver == nil {
		apiserver = &KarmadaAPIServer{}
	}

	if len(apiserver.CommonSettings.Image.ImageRepository) == 0 {
		apiserver.CommonSettings.Image.ImageRepository = calculateImageRepository(karmada.Spec.PrivateRegistry, "registry.k8s.io", "kube-apiserver")
	}

	if len(apiserver.CommonSettings.Image.ImageTag) == 0 {
		apiserver.CommonSettings.Image.ImageTag = "v1.25.2"
	}

	if apiserver.CommonSettings.Replicas == nil {
		apiserver.CommonSettings.Replicas = pointer.Int32(1)
	}

	if apiserver.ServiceSubnet == nil {
		apiserver.ServiceSubnet = pointer.String("10.96.0.0/12")
	}
	karmada.Spec.Components.KarmadaAPIServer = apiserver
}

// populateDefaultsKarmadaAggregratedAPIServer populate defaults to karmada aggregrated apiserver.
func populateDefaultsKarmadaAggregratedAPIServer(karmada *Karmada) {
	aggregratedApiserver := karmada.Spec.Components.KarmadaAggregratedAPIServer
	if aggregratedApiserver == nil {
		aggregratedApiserver = &KarmadaAggregratedAPIServer{}
	}

	if len(aggregratedApiserver.CommonSettings.Image.ImageRepository) == 0 {
		aggregratedApiserver.CommonSettings.Image.ImageRepository = calculateImageRepository(karmada.Spec.PrivateRegistry, "docker.io", "karmada/karmada-aggregated-apiserver")
	}

	if len(aggregratedApiserver.CommonSettings.Image.ImageTag) == 0 {
		aggregratedApiserver.CommonSettings.Image.ImageTag = "v1.4.0"
	}

	if aggregratedApiserver.CommonSettings.Replicas == nil {
		aggregratedApiserver.CommonSettings.Replicas = pointer.Int32(1)
	}
	karmada.Spec.Components.KarmadaAggregratedAPIServer = aggregratedApiserver
}

// populateDefaultsKubeControllerManager populate defaults to kube controller manager.
func populateDefaultsKubeControllerManager(karmada *Karmada) {
	kubeControllerManager := karmada.Spec.Components.KubeControllerManager
	if kubeControllerManager == nil {
		kubeControllerManager = &KubeControllerManager{}
	}

	if len(kubeControllerManager.CommonSettings.Image.ImageRepository) == 0 {
		kubeControllerManager.CommonSettings.Image.ImageRepository = calculateImageRepository(karmada.Spec.PrivateRegistry,
			"registry.k8s.io", "karmada/kube-controller-manager")
	}

	if len(kubeControllerManager.CommonSettings.Image.ImageTag) == 0 {
		kubeControllerManager.CommonSettings.Image.ImageTag = "v1.25.2"
	}

	if kubeControllerManager.CommonSettings.Replicas == nil {
		kubeControllerManager.CommonSettings.Replicas = pointer.Int32(1)
	}
	karmada.Spec.Components.KubeControllerManager = kubeControllerManager
}

// populateDefaultsKarmadaControllerManager populate defaults to karmada controller manager.
func populateDefaultsKarmadaControllerManager(karmada *Karmada) {
	karmadaControllerManager := karmada.Spec.Components.KarmadaControllerManager
	if karmadaControllerManager == nil {
		karmadaControllerManager = &KarmadaControllerManager{}
	}

	if len(karmadaControllerManager.CommonSettings.Image.ImageRepository) == 0 {
		karmadaControllerManager.CommonSettings.Image.ImageRepository = calculateImageRepository(karmada.Spec.PrivateRegistry, "docker.io", "karmada/karmada-controller-manager")
	}

	if len(karmadaControllerManager.CommonSettings.Image.ImageTag) == 0 {
		karmadaControllerManager.CommonSettings.Image.ImageTag = "v1.4.0"
	}

	if karmadaControllerManager.CommonSettings.Replicas == nil {
		karmadaControllerManager.CommonSettings.Replicas = pointer.Int32(1)
	}
	karmada.Spec.Components.KarmadaControllerManager = karmadaControllerManager
}

// populateDefaultsKarmadaScheduler populate defaults to karmada scheduler.
func populateDefaultsKarmadaScheduler(karmada *Karmada) {
	karmadaScheduler := karmada.Spec.Components.KarmadaScheduler
	if karmadaScheduler == nil {
		karmadaScheduler = &KarmadaScheduler{}
	}

	if len(karmadaScheduler.CommonSettings.Image.ImageRepository) == 0 {
		karmadaScheduler.CommonSettings.Image.ImageRepository = calculateImageRepository(karmada.Spec.PrivateRegistry,
			"docker.io", "karmada/karmada-scheduler")
	}

	if len(karmadaScheduler.CommonSettings.Image.ImageTag) == 0 {
		karmadaScheduler.CommonSettings.Image.ImageTag = "v1.4.0"
	}

	if karmadaScheduler.CommonSettings.Replicas == nil {
		karmadaScheduler.CommonSettings.Replicas = pointer.Int32(1)
	}
	karmada.Spec.Components.KarmadaScheduler = karmadaScheduler
}

// populateDefaultsKarmadaWebhook populate defaults to karmada webhook.
func populateDefaultsKarmadaWebhook(karmada *Karmada) {
	webhook := karmada.Spec.Components.KarmadaWebhook
	if webhook == nil {
		webhook = &KarmadaWebhook{}
	}

	if len(webhook.CommonSettings.Image.ImageRepository) == 0 {
		webhook.CommonSettings.Image.ImageRepository = calculateImageRepository(karmada.Spec.PrivateRegistry,
			"docker.io", "karmada/karmada-webhook")
	}

	if len(webhook.CommonSettings.Image.ImageTag) == 0 {
		webhook.CommonSettings.Image.ImageTag = "v1.4.0"
	}

	if webhook.CommonSettings.Replicas == nil {
		webhook.CommonSettings.Replicas = pointer.Int32(1)
	}
	karmada.Spec.Components.KarmadaWebhook = webhook
}

// calculateImageRepository return a repository that includes two parts "registry/repository".
// the registry use privateRegistry as first priority, and if privateRegistry is not set,
// the default registry ("docker.io" or "registry.k8s.io") will be used.
func calculateImageRepository(privateRegistry *ImageRegistry, defaultRegistry, repository string) string {
	if privateRegistry != nil && len(privateRegistry.Registry) > 0 {
		defaultRegistry = privateRegistry.Registry
	}

	return fmt.Sprintf("%s/%s", defaultRegistry, repository)
}

// +kubebuilder:webhook:verbs=create;update,path=/validate-operator-karmada-io-v1alpha1-karmada,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.karmada.io,resources=karmadas,versions=v1alpha1,name=default.karmada.operator.karmada.io,admissionReviewVersions=v1alpha1

var _ webhook.Validator = &Karmada{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (karmada *Karmada) ValidateCreate() error {
	return karmada.validate(nil)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (karmada *Karmada) ValidateUpdate(old runtime.Object) error {
	oldK, ok := old.(*Karmada)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a Karmada but got a %T", old))
	}

	return karmada.validate(oldK)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
// it's nothing to do for us.
func (karmada *Karmada) ValidateDelete() error {
	return nil
}

// validate work on update and create operator.
func (karmada *Karmada) validate(old *Karmada) error {
	var allErrs field.ErrorList
	specPath := field.NewPath("spec")

	if karmada.Spec.HostCluster != nil && karmada.Spec.HostCluster.Networking != nil &&
		karmada.Spec.HostCluster.Networking.DNSDomain != nil {
		if errs := validation.NameIsDNSSubdomain(*karmada.Spec.HostCluster.Networking.DNSDomain, true); len(errs) != 0 {
			for _, err := range errs {
				allErrs = append(allErrs,
					field.Invalid(
						specPath.Child("hostCluster", "networking", "dnsDomain"),
						*karmada.Spec.HostCluster.Networking.DNSDomain,
						fmt.Sprintf("must be a valid domain value: %s", err),
					))
			}
		}
	}

	// either local etcd or external etcd to be set. we must ensure that there is a available etcd.
	if karmada.Spec.Components == nil {
		allErrs = append(allErrs,
			field.Required(
				specPath.Child("components"),
				"expected non-empty components object",
			))
	} else if karmada.Spec.Components.Etcd == nil {
		allErrs = append(allErrs,
			field.Required(
				specPath.Child("components", "etcd"),
				"expected non-empty etcd object",
			))
	} else if karmada.Spec.Components.Etcd.External == nil && karmada.Spec.Components.Etcd.Local == nil {
		allErrs = append(allErrs,
			field.Required(
				specPath.Child("components", "etcd", "data"),
				"expected either spec.components.etcd.external or spec.components.etcd.local to be populated",
			))
	}

	// TODO: add more validations for karmada cr.
	if len(allErrs) == 0 {
		return nil
	}
	return apierrors.NewInvalid(SchemeGroupVersion.WithKind("Karmada").GroupKind(), karmada.Name, allErrs)
}
