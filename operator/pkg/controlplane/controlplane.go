package controlplane

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/util/patcher"
)

// EnsureControlPlaneComponent creates karmada controllerManager, kubeControllerManager, scheduler, webhook component
func EnsureControlPlaneComponent(component, name, namespace string, featureGates map[string]bool, client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents) error {
	deployments, err := getComponentManifests(name, namespace, featureGates, cfg)
	if err != nil {
		return err
	}

	deployment, ok := deployments[component]
	if !ok {
		klog.Infof("Skip installing component %s(%s/%s)", component, namespace, name)
		return nil
	}

	if err := apiclient.CreateOrUpdateDeployment(client, deployment); err != nil {
		return fmt.Errorf("failed to create deployment resource for component %s, err: %w", component, err)
	}
	return nil
}

func getComponentManifests(name, namespace string, featureGates map[string]bool, cfg *operatorv1alpha1.KarmadaComponents) (map[string]*appsv1.Deployment, error) {
	kubeControllerManager, err := getKubeControllerManagerManifest(name, namespace, cfg.KubeControllerManager)
	if err != nil {
		return nil, err
	}
	karmadaControllerManager, err := getKarmadaControllerManagerManifest(name, namespace, featureGates, cfg.KarmadaControllerManager)
	if err != nil {
		return nil, err
	}
	karmadaScheduler, err := getKarmadaSchedulerManifest(name, namespace, featureGates, cfg.KarmadaScheduler)
	if err != nil {
		return nil, err
	}

	manifest := map[string]*appsv1.Deployment{
		constants.KarmadaControllerManagerComponent: karmadaControllerManager,
		constants.KubeControllerManagerComponent:    kubeControllerManager,
		constants.KarmadaSchedulerComponent:         karmadaScheduler,
	}

	if cfg.KarmadaDescheduler != nil {
		descheduler, err := getKarmadaDeschedulerManifest(name, namespace, featureGates, cfg.KarmadaDescheduler)
		if err != nil {
			return nil, err
		}
		manifest[constants.KarmadaDeschedulerComponent] = descheduler
	}

	return manifest, nil
}

func getKubeControllerManagerManifest(name, namespace string, cfg *operatorv1alpha1.KubeControllerManager) (*appsv1.Deployment, error) {
	kubeControllerManagerBytes, err := util.ParseTemplate(KubeControllerManagerDeployment, struct {
		DeploymentName, Namespace, Image     string
		KarmadaCertsSecret, KubeconfigSecret string
		Replicas                             *int32
	}{
		DeploymentName:     util.KubeControllerManagerName(name),
		Namespace:          namespace,
		Image:              cfg.Image.Name(),
		KarmadaCertsSecret: util.KarmadaCertSecretName(name),
		KubeconfigSecret:   util.AdminKubeconfigSecretName(name),
		Replicas:           cfg.Replicas,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing kube-controller-manager deployment template: %w", err)
	}

	kcm := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), kubeControllerManagerBytes, kcm); err != nil {
		return nil, fmt.Errorf("err when decoding kube-controller-manager deployment: %w", err)
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).
		WithLabels(cfg.Labels).WithExtraArgs(cfg.ExtraArgs).ForDeployment(kcm)
	return kcm, nil
}

func getKarmadaControllerManagerManifest(name, namespace string, featureGates map[string]bool, cfg *operatorv1alpha1.KarmadaControllerManager) (*appsv1.Deployment, error) {
	karmadaControllerManagerBytes, err := util.ParseTemplate(KamradaControllerManagerDeployment, struct {
		Replicas                                   *int32
		DeploymentName, Namespace, SystemNamespace string
		Image, KubeconfigSecret                    string
	}{
		DeploymentName:   util.KarmadaControllerManagerName(name),
		Namespace:        namespace,
		SystemNamespace:  namespace,
		Image:            cfg.Image.Name(),
		KubeconfigSecret: util.AdminKubeconfigSecretName(name),
		Replicas:         cfg.Replicas,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing karmada-controller-manager deployment template: %w", err)
	}

	kcm := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaControllerManagerBytes, kcm); err != nil {
		return nil, fmt.Errorf("err when decoding karmada-controller-manager deployment: %w", err)
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).
		WithExtraArgs(cfg.ExtraArgs).WithFeatureGates(featureGates).ForDeployment(kcm)
	return kcm, nil
}

func getKarmadaSchedulerManifest(name, namespace string, featureGates map[string]bool, cfg *operatorv1alpha1.KarmadaScheduler) (*appsv1.Deployment, error) {
	karmadaSchedulerBytes, err := util.ParseTemplate(KarmadaSchedulerDeployment, struct {
		Replicas                                   *int32
		DeploymentName, Namespace, SystemNamespace string
		Image, KubeconfigSecret                    string
	}{
		DeploymentName:   util.KarmadaSchedulerName(name),
		Namespace:        namespace,
		SystemNamespace:  namespace,
		Image:            cfg.Image.Name(),
		KubeconfigSecret: util.AdminKubeconfigSecretName(name),
		Replicas:         cfg.Replicas,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing karmada-scheduler deployment template: %w", err)
	}

	scheduler := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaSchedulerBytes, scheduler); err != nil {
		return nil, fmt.Errorf("err when decoding karmada-scheduler deployment: %w", err)
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).
		WithExtraArgs(cfg.ExtraArgs).WithFeatureGates(featureGates).ForDeployment(scheduler)
	return scheduler, nil
}

func getKarmadaDeschedulerManifest(name, namespace string, featureGates map[string]bool, cfg *operatorv1alpha1.KarmadaDescheduler) (*appsv1.Deployment, error) {
	karmadaDeschedulerBytes, err := util.ParseTemplate(KarmadaDeschedulerDeployment, struct {
		Replicas                                   *int32
		DeploymentName, Namespace, SystemNamespace string
		Image, KubeconfigSecret                    string
	}{
		DeploymentName:   util.KarmadaDeschedulerName(name),
		Namespace:        namespace,
		SystemNamespace:  namespace,
		Image:            cfg.Image.Name(),
		KubeconfigSecret: util.AdminKubeconfigSecretName(name),
		Replicas:         cfg.Replicas,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing karmada-descheduler deployment template: %w", err)
	}

	descheduler := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaDeschedulerBytes, descheduler); err != nil {
		return nil, fmt.Errorf("err when decoding karmada-descheduler deployment: %w", err)
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).
		WithExtraArgs(cfg.ExtraArgs).WithFeatureGates(featureGates).ForDeployment(descheduler)

	return descheduler, nil
}
