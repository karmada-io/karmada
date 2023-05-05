package controlplane

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
	"github.com/karmada-io/karmada/operator/pkg/constants"
	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
	"github.com/karmada-io/karmada/operator/pkg/util/patcher"
)

// EnsureControlPlaneComponent creates karmada controllerManager, kubeControllerManager, scheduler, webhook component
func EnsureControlPlaneComponent(component, name, namespace string, client clientset.Interface, cfg *operatorv1alpha1.KarmadaComponents) error {
	deployments, err := getComponentManifests(name, namespace, cfg)
	if err != nil {
		return err
	}

	deployment, ok := deployments[component]
	if !ok {
		return fmt.Errorf("no exist manifest for %s", component)
	}
	if err := apiclient.CreateOrUpdateDeployment(client, deployment); err != nil {
		return fmt.Errorf("failed to create deployment resource for component %s, err: %w", component, err)
	}
	return nil
}

func getComponentManifests(name, namespace string, cfg *operatorv1alpha1.KarmadaComponents) (map[string]*appsv1.Deployment, error) {
	kubeControllerManager, err := getKubeControllerManagerManifest(name, namespace, cfg.KubeControllerManager)
	if err != nil {
		return nil, err
	}
	karmadaControllerManager, err := getKarmadaControllerManagerManifest(name, namespace, cfg.KarmadaControllerManager)
	if err != nil {
		return nil, err
	}
	scheduler, err := getKarmadaSchedulerManifest(name, namespace, cfg.KarmadaScheduler)
	if err != nil {
		return nil, err
	}

	return map[string]*appsv1.Deployment{
		constants.KubeControllerManagerComponent:    kubeControllerManager,
		constants.KarmadaControllerManagerComponent: karmadaControllerManager,
		constants.KarmadaSchedulerComponent:         scheduler,
	}, nil
}

func getKubeControllerManagerManifest(name, namespace string, cfg *operatorv1alpha1.KubeControllerManager) (*appsv1.Deployment, error) {
	kubeControllerManageretBytes, err := util.ParseTemplate(KubeControllerManagerDeployment, struct {
		DeploymentName, Namespace, Image     string
		KarmadaCertsSecret, KubeconfigSecret string
		Replicas                             *int32
	}{
		DeploymentName:     util.KubeControllerManagerName(name),
		Namespace:          namespace,
		Image:              cfg.Image.Name(),
		KarmadaCertsSecret: util.KarmadaCertSecretName(name),
		KubeconfigSecret:   util.AdminKubeconfigSercretName(name),
		Replicas:           cfg.Replicas,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing KubeControllerManager Deployment template: %w", err)
	}

	kcm := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), kubeControllerManageretBytes, kcm); err != nil {
		return nil, fmt.Errorf("err when decoding KubeControllerManager Deployment: %w", err)
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).ForDeployment(kcm)
	return kcm, nil
}

func getKarmadaControllerManagerManifest(name, namespace string, cfg *operatorv1alpha1.KarmadaControllerManager) (*appsv1.Deployment, error) {
	karmadaControllerManageretBytes, err := util.ParseTemplate(KamradaControllerManagerDeployment, struct {
		Replicas                  *int32
		DeploymentName, Namespace string
		Image, KubeconfigSecret   string
	}{
		DeploymentName:   util.KarmadaControllerManagerName(name),
		Namespace:        namespace,
		Image:            cfg.Image.Name(),
		KubeconfigSecret: util.AdminKubeconfigSercretName(name),
		Replicas:         cfg.Replicas,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing KarmadaControllerManager Deployment template: %w", err)
	}

	kcm := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaControllerManageretBytes, kcm); err != nil {
		return nil, fmt.Errorf("err when decoding KarmadaControllerManager Deployment: %w", err)
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).ForDeployment(kcm)
	return kcm, nil
}

func getKarmadaSchedulerManifest(name, namespace string, cfg *operatorv1alpha1.KarmadaScheduler) (*appsv1.Deployment, error) {
	karmadaSchedulerBytes, err := util.ParseTemplate(KarmadaSchedulerDeployment, struct {
		Replicas                  *int32
		DeploymentName, Namespace string
		Image, KubeconfigSecret   string
	}{
		DeploymentName:   util.KarmadaSchedulerName(name),
		Namespace:        namespace,
		Image:            cfg.Image.Name(),
		KubeconfigSecret: util.AdminKubeconfigSercretName(name),
		Replicas:         cfg.Replicas,
	})
	if err != nil {
		return nil, fmt.Errorf("error when parsing KarmadaScheduler Deployment template: %w", err)
	}

	scheduler := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaSchedulerBytes, scheduler); err != nil {
		return nil, fmt.Errorf("err when decoding KarmadaScheduler Deployment: %w", err)
	}

	patcher.NewPatcher().WithAnnotations(cfg.Annotations).WithLabels(cfg.Labels).ForDeployment(scheduler)
	return scheduler, nil
}
