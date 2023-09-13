package metricsadapter

import (
	"context"
	"encoding/base64"
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
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
)

// aaAPIServiceName define apiservice name install on karmada control plane
var aaAPIServices = []string{
	"v1beta1.metrics.k8s.io",
	"v1beta1.custom.metrics.k8s.io",
	"v1beta2.custom.metrics.k8s.io",
}

// AddonMetricsAdapter describe the metrics-adapter addon command process
var AddonMetricsAdapter = &addoninit.Addon{
	Name:    addoninit.MetricsAdapterResourceName,
	Status:  status,
	Enable:  enableMetricsAdapter,
	Disable: disableMetricsAdapter,
}

var status = func(opts *addoninit.CommandAddonsListOption) (string, error) {
	// check karmada-metrics-adapter deployment status on host cluster
	deployClient := opts.KubeClientSet.AppsV1().Deployments(opts.Namespace)
	deployment, err := deployClient.Get(context.TODO(), addoninit.MetricsAdapterResourceName, metav1.GetOptions{})
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

	// check metrics.k8s.io apiservice is available on karmada control plane
	for _, aaAPIServiceName := range aaAPIServices {
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
	}

	return addoninit.AddonEnabledStatus, nil
}

var enableMetricsAdapter = func(opts *addoninit.CommandAddonsEnableOption) error {
	if err := installComponentsOnHostCluster(opts); err != nil {
		return err
	}

	if err := installComponentsOnKarmadaControlPlane(opts); err != nil {
		return err
	}

	return nil
}

var disableMetricsAdapter = func(opts *addoninit.CommandAddonsDisableOption) error {
	// delete karmada metrics adapter service on host cluster
	serviceClient := opts.KubeClientSet.CoreV1().Services(opts.Namespace)
	if err := serviceClient.Delete(context.TODO(), addoninit.MetricsAdapterResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	klog.Infof("Uninstall karmada metrics adapter service on host cluster successfully")

	// delete karmada metrics adapter deployment on host cluster
	deployClient := opts.KubeClientSet.AppsV1().Deployments(opts.Namespace)
	if err := deployClient.Delete(context.TODO(), addoninit.MetricsAdapterResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	klog.Infof("Uninstall karmada metrics adapter deployment on host cluster successfully")

	// delete karmada metrics adapter aa service on karmada control plane
	karmadaServiceClient := opts.KarmadaKubeClientSet.CoreV1().Services(opts.Namespace)
	if err := karmadaServiceClient.Delete(context.TODO(), addoninit.MetricsAdapterResourceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	klog.Infof("Uninstall karmada metrics adapter AA service on karmada control plane successfully")
	for _, aaAPIServiceName := range aaAPIServices {
		// delete karmada metrics adapter aa apiservice on karmada control plane
		if err := opts.KarmadaAggregatorClientSet.ApiregistrationV1().APIServices().Delete(context.TODO(), aaAPIServiceName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}

	klog.Infof("Uninstall karmada metrics adapter AA apiservice on karmada control plane successfully")
	return nil
}

func installComponentsOnHostCluster(opts *addoninit.CommandAddonsEnableOption) error {
	// install karmada metrics adapter service on host cluster
	karmadaMetricsAdapterServiceBytes, err := addonutils.ParseTemplate(karmadaMetricsAdapterService, ServiceReplace{
		Namespace: opts.Namespace,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada metrics adapter service template :%v", err)
	}

	karmadaMetricsAdapterService := &corev1.Service{}
	if err = kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaMetricsAdapterServiceBytes, karmadaMetricsAdapterService); err != nil {
		return fmt.Errorf("decode karmada metrics adapter service error: %v", err)
	}

	if err = cmdutil.CreateService(opts.KubeClientSet, karmadaMetricsAdapterService); err != nil {
		return fmt.Errorf("create karmada metrics adapter service error: %v", err)
	}

	klog.Infof("Install karmada metrics adapter service on host cluster successfully")

	// install karmada metrics adapter deployment on host clusters
	karmadaMetricsAdapterDeploymentBytes, err := addonutils.ParseTemplate(karmadaMetricsAdapterDeployment, DeploymentReplace{
		Namespace: opts.Namespace,
		Replicas:  &opts.KarmadaMetricsAdapterReplicas,
		Image:     addoninit.KarmadaMetricsAdapterImage(opts),
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada metrics adapter deployment template :%v", err)
	}

	karmadaMetricsAdapterDeployment := &appsv1.Deployment{}
	if err = kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaMetricsAdapterDeploymentBytes, karmadaMetricsAdapterDeployment); err != nil {
		return fmt.Errorf("decode karmada metrics adapter deployment error: %v", err)
	}
	if err = cmdutil.CreateOrUpdateDeployment(opts.KubeClientSet, karmadaMetricsAdapterDeployment); err != nil {
		return fmt.Errorf("create karmada metrics adapter deployment error: %v", err)
	}

	if err = cmdutil.WaitForDeploymentRollout(opts.KubeClientSet, karmadaMetricsAdapterDeployment, opts.WaitComponentReadyTimeout); err != nil {
		return fmt.Errorf("wait karmada metrics adapter pod status ready timeout: %v", err)
	}

	klog.Infof("Install karmada metrics adapter deployment on host cluster successfully")
	return nil
}

func installComponentsOnKarmadaControlPlane(opts *addoninit.CommandAddonsEnableOption) error {
	// install karmada metrics adapter AA service on karmada control plane
	aaServiceBytes, err := addonutils.ParseTemplate(karmadaMetricsAdapterAAService, AAServiceReplace{
		Namespace:         opts.Namespace,
		HostClusterDomain: opts.HostClusterDomain,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada metrics adapter AA service template :%v", err)
	}

	caCertName := fmt.Sprintf("%s.crt", options.CaCertAndKeyName)
	karmadaCerts, err := opts.KubeClientSet.CoreV1().Secrets(opts.Namespace).Get(context.TODO(), options.KarmadaCertsName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error when getting Secret %s/%s, which is used to fetch CaCert for building APISevice: %+v", opts.Namespace, options.KarmadaCertsName, err)
	}

	aaService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aaServiceBytes, aaService); err != nil {
		return fmt.Errorf("decode karmada metrics adapter AA service error: %v", err)
	}
	if err := cmdutil.CreateService(opts.KarmadaKubeClientSet, aaService); err != nil {
		return fmt.Errorf("create karmada metrics adapter AA service error: %v", err)
	}
	for _, aaAPIServiceName := range aaAPIServices {
		// install karmada metrics adapter apiservice on karmada control plane
		gv := strings.SplitN(aaAPIServiceName, ".", 2)
		aaAPIServiceBytes, err := addonutils.ParseTemplate(karmadaMetricsAdapterAAAPIService, AAApiServiceReplace{
			Name:      aaAPIServiceName,
			Namespace: opts.Namespace,
			Group:     gv[1],
			Version:   gv[0],
			CABundle:  base64.StdEncoding.EncodeToString(karmadaCerts.Data[caCertName]),
		})
		if err != nil {
			return fmt.Errorf("error when parsing karmada metrics adapter AA apiservice template :%v", err)
		}
		aaAPIService := &apiregistrationv1.APIService{}
		if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aaAPIServiceBytes, aaAPIService); err != nil {
			return fmt.Errorf("decode karmada metrics adapter AA apiservice error: %v", err)
		}

		if err = cmdutil.CreateOrUpdateAPIService(opts.KarmadaAggregatorClientSet, aaAPIService); err != nil {
			return fmt.Errorf("create karmada metrics adapter AA apiservice error: %v", err)
		}

		if err := initkarmada.WaitAPIServiceReady(opts.KarmadaAggregatorClientSet, aaAPIServiceName, time.Duration(opts.WaitAPIServiceReadyTimeout)*time.Second); err != nil {
			return err
		}
	}
	klog.Infof("Install karmada metrics adapter api server on karmada control plane successfully")
	return nil
}
