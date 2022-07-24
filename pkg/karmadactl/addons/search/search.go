package search

import (
	"context"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1helper "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1/helper"

	addoninit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	addonutils "github.com/karmada-io/karmada/pkg/karmadactl/addons/utils"
	initkarmada "github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/karmada"
	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/kubernetes"
	initutils "github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

const (
	// aaAPIServiceName define apiservice name install on karmada control plane
	aaAPIServiceName = "v1alpha1.search.karmada.io"

	// etcdStatefulSetAndServiceName define etcd statefulSet and serviceName installed by init command
	etcdStatefulSetAndServiceName = "etcd"

	// etcdContainerClientPort define etcd pod installed by init command
	etcdContainerClientPort = 2379
)

var (
	karmadaSearchLabels = map[string]string{"app": addoninit.SearchResourceName, "apiserver": "true"}
)

// AddonSearch describe the search addon command process
var AddonSearch = &addoninit.Addon{
	Name:    addoninit.SearchResourceName,
	Status:  status,
	Enable:  enableSearch,
	Disable: disableSearch,
}

var status = func(opts *addoninit.CommandAddonsListOption) (string, error) {
	// check karmada-search deployment status on host cluster
	deployClient := opts.KubeClientSet.AppsV1().Deployments(opts.Namespace)
	deployment, err := deployClient.Get(context.TODO(), addoninit.SearchResourceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return addoninit.AddonDisabledStatus, nil
		}
		return addoninit.AddonUnknownStatus, err
	}
	if deployment.Status.Replicas != deployment.Status.ReadyReplicas ||
		deployment.Status.Replicas != deployment.Status.AvailableReplicas {
		return addoninit.AddonUnhealthyStatus, nil
	}

	// check karmada-search apiservice is available on karmada control plane
	apiService, err := opts.KarmadaAggregatorClientSet.ApiregistrationV1().APIServices().Get(context.TODO(), aaAPIServiceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return addoninit.AddonDisabledStatus, nil
		}
		return addoninit.AddonUnknownStatus, err
	}

	if !apiregistrationv1helper.IsAPIServiceConditionTrue(apiService, apiregistrationv1.Available) {
		return addoninit.AddonUnhealthyStatus, nil
	}

	return addoninit.AddonEnabledStatus, nil
}

var enableSearch = func(opts *addoninit.CommandAddonsEnableOption) error {

	if err := installComponentsOnHostCluster(opts); err != nil {
		return err
	}

	if err := installComponentsOnKarmadaControlPlane(opts); err != nil {
		return err
	}

	return nil
}

var disableSearch = func(opts *addoninit.CommandAddonsDisableOption) error {
	// delete karmada search service on host cluster
	serviceClient := opts.KubeClientSet.CoreV1().Services(opts.Namespace)
	if err := serviceClient.Delete(context.TODO(), addoninit.SearchResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	klog.Infof("Uninstall karmada search service on host cluster successfully")

	// delete karmada search deployment on host cluster
	deployClient := opts.KubeClientSet.AppsV1().Deployments(opts.Namespace)
	if err := deployClient.Delete(context.TODO(), addoninit.SearchResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	klog.Infof("Uninstall karmada search deployment on host cluster successfully")

	// delete karmada search aa service on karmada control plane
	karmadaServiceClient := opts.KarmadaKubeClientSet.CoreV1().Services(opts.Namespace)
	if err := karmadaServiceClient.Delete(context.TODO(), addoninit.SearchResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	klog.Infof("Uninstall karmada search AA service on karmada control plane successfully")

	// delete karmada search aa apiservice on karmada control plane
	if err := opts.KarmadaAggregatorClientSet.ApiregistrationV1().APIServices().Delete(context.TODO(), aaAPIServiceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	klog.Infof("Uninstall karmada search AA apiservice on karmada control plane successfully")
	return nil
}

func installComponentsOnHostCluster(opts *addoninit.CommandAddonsEnableOption) error {
	// install karmada search service on host cluster
	karmadaSearchServiceBytes, err := addonutils.ParseTemplate(karmadaSearchService, ServiceReplace{
		Namespace: opts.Namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada search service template :%v", err)
	}

	karmadaSearchService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaSearchServiceBytes, karmadaSearchService); err != nil {
		return fmt.Errorf("decode karmada search service error: %v", err)
	}

	if err := addonutils.CreateService(opts.KubeClientSet, karmadaSearchService); err != nil {
		return fmt.Errorf("create karmada search service error: %v", err)
	}

	etcdServers, err := etcdServers(opts)
	if err != nil {
		return err
	}

	klog.Infof("Install karmada search service on host cluster successfully")

	// install karmada search deployment on host clusters
	karmadaSearchDeploymentBytes, err := addonutils.ParseTemplate(karmadaSearchDeployment, DeploymentReplace{
		Namespace:  opts.Namespace,
		Replicas:   &opts.KarmadaSearchReplicas,
		ETCDSevers: etcdServers,
		Image:      opts.KarmadaSearchImage,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada search deployment template :%v", err)
	}

	karmadaSearchDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaSearchDeploymentBytes, karmadaSearchDeployment); err != nil {
		return fmt.Errorf("decode karmada search deployment error: %v", err)
	}
	if err := addonutils.CreateOrUpdateDeployment(opts.KubeClientSet, karmadaSearchDeployment); err != nil {
		return fmt.Errorf("create karmada search deployment error: %v", err)
	}

	if err := kubernetes.WaitPodReady(opts.KubeClientSet, opts.Namespace, initutils.MapToString(karmadaSearchLabels), opts.WaitPodReadyTimeout); err != nil {
		return fmt.Errorf("wait karmada search pod status ready timeout: %v", err)
	}

	klog.Infof("Install karmada search deployment on host cluster successfully")
	return nil
}

func installComponentsOnKarmadaControlPlane(opts *addoninit.CommandAddonsEnableOption) error {
	// install karmada search AA service on karmada control plane
	aaServiceBytes, err := addonutils.ParseTemplate(karmadaSearchAAService, AAServiceReplace{
		Namespace: opts.Namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada search AA service template :%v", err)
	}

	aaService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aaServiceBytes, aaService); err != nil {
		return fmt.Errorf("decode karmada search AA service error: %v", err)
	}
	if err := addonutils.CreateService(opts.KarmadaKubeClientSet, aaService); err != nil {
		return fmt.Errorf("create karmada search AA service error: %v", err)
	}

	// install karmada search apiservice on karmada control plane
	aaAPIServiceBytes, err := addonutils.ParseTemplate(karmadaSearchAAAPIService, AAApiServiceReplace{
		Name:      aaAPIServiceName,
		Namespace: opts.Namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada search AA apiservice template :%v", err)
	}

	aaAPIService := &apiregistrationv1.APIService{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aaAPIServiceBytes, aaAPIService); err != nil {
		return fmt.Errorf("decode karmada search AA apiservice error: %v", err)
	}

	if err = addonutils.CreateOrUpdateAPIService(opts.KarmadaAggregatorClientSet, aaAPIService); err != nil {
		return fmt.Errorf("craete karmada search AA apiservice error: %v", err)
	}

	if err := initkarmada.WaitAPIServiceReady(opts.KarmadaAggregatorClientSet, aaAPIServiceName, time.Duration(opts.WaitAPIServiceReadyTimeout)*time.Second); err != nil {
		return err
	}

	klog.Infof("Install karmada search api server on karmada control plane successfully")
	return nil
}

func etcdServers(opts *addoninit.CommandAddonsEnableOption) (string, error) {
	sts, err := opts.KubeClientSet.AppsV1().StatefulSets(opts.Namespace).Get(context.TODO(), "etcd", metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	ectdReplicas := *sts.Spec.Replicas
	ectdServers := ""

	for v := int32(0); v < ectdReplicas; v++ {
		ectdServers += fmt.Sprintf("https://%s-%v.%s.%s.svc.cluster.local:%v", etcdStatefulSetAndServiceName, v, etcdStatefulSetAndServiceName, opts.Namespace, etcdContainerClientPort) + ","
	}

	return strings.TrimRight(ectdServers, ","), nil
}
