/*
Copyright 2022 The Karmada Authors.

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

package search

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
	"k8s.io/client-go/kubernetes"
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

const (
	// aaAPIServiceName define apiservice name install on karmada control plane
	aaAPIServiceName = "v1alpha1.search.karmada.io"

	// etcdStatefulSetAndServiceName define etcd statefulSet and serviceName installed by init command
	etcdStatefulSetAndServiceName = "etcd"
	// karmadaAPIServerDeploymentAndServiceName defines the name of karmada-apiserver deployment and service installed by init command
	karmadaAPIServerDeploymentAndServiceName = "karmada-apiserver"

	// etcdContainerClientPort define etcd pod installed by init command
	etcdContainerClientPort = 2379
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

	if err := cmdutil.CreateService(opts.KubeClientSet, karmadaSearchService); err != nil {
		return fmt.Errorf("create karmada search service error: %v", err)
	}

	etcdServers, keyPrefix, err := etcdServers(opts)
	if err != nil {
		return err
	}

	klog.Infof("Install karmada search service on host cluster successfully")

	// install karmada search deployment on host clusters
	karmadaSearchDeploymentBytes, err := addonutils.ParseTemplate(karmadaSearchDeployment, DeploymentReplace{
		Namespace:  opts.Namespace,
		Replicas:   &opts.KarmadaSearchReplicas,
		ETCDSevers: etcdServers,
		KeyPrefix:  keyPrefix,
		Image:      addoninit.KarmadaSearchImage(opts),
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada search deployment template :%v", err)
	}

	karmadaSearchDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaSearchDeploymentBytes, karmadaSearchDeployment); err != nil {
		return fmt.Errorf("decode karmada search deployment error: %v", err)
	}
	if err := cmdutil.CreateOrUpdateDeployment(opts.KubeClientSet, karmadaSearchDeployment); err != nil {
		return fmt.Errorf("create karmada search deployment error: %v", err)
	}

	if err := cmdutil.WaitForDeploymentRollout(opts.KubeClientSet, karmadaSearchDeployment, opts.WaitComponentReadyTimeout); err != nil {
		return fmt.Errorf("wait karmada search pod status ready timeout: %v", err)
	}

	klog.Infof("Install karmada search deployment on host cluster successfully")
	return nil
}

func installComponentsOnKarmadaControlPlane(opts *addoninit.CommandAddonsEnableOption) error {
	// install karmada search AA service on karmada control plane
	aaServiceBytes, err := addonutils.ParseTemplate(karmadaSearchAAService, AAServiceReplace{
		Namespace:         opts.Namespace,
		HostClusterDomain: opts.HostClusterDomain,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada search AA service template :%v", err)
	}

	caCertName := fmt.Sprintf("%s.crt", options.CaCertAndKeyName)
	karmadaCerts, err := opts.KubeClientSet.CoreV1().Secrets(opts.Namespace).Get(context.TODO(), options.KarmadaCertsName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error when getting Secret %s/%s, which is used to fetch CaCert for building APIService: %+v", opts.Namespace, options.KarmadaCertsName, err)
	}

	aaService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aaServiceBytes, aaService); err != nil {
		return fmt.Errorf("decode karmada search AA service error: %v", err)
	}
	if err := cmdutil.CreateService(opts.KarmadaKubeClientSet, aaService); err != nil {
		return fmt.Errorf("create karmada search AA service error: %v", err)
	}

	// install karmada search apiservice on karmada control plane
	aaAPIServiceBytes, err := addonutils.ParseTemplate(karmadaSearchAAAPIService, AAApiServiceReplace{
		Name:      aaAPIServiceName,
		Namespace: opts.Namespace,
		CABundle:  base64.StdEncoding.EncodeToString(karmadaCerts.Data[caCertName]),
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada search AA apiservice template :%v", err)
	}

	aaAPIService := &apiregistrationv1.APIService{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), aaAPIServiceBytes, aaAPIService); err != nil {
		return fmt.Errorf("decode karmada search AA apiservice error: %v", err)
	}

	if err = cmdutil.CreateOrUpdateAPIService(opts.KarmadaAggregatorClientSet, aaAPIService); err != nil {
		return fmt.Errorf("create karmada search AA apiservice error: %v", err)
	}

	if err := initkarmada.WaitAPIServiceReady(opts.KarmadaAggregatorClientSet, aaAPIServiceName, time.Duration(opts.WaitAPIServiceReadyTimeout)*time.Second); err != nil {
		return err
	}

	klog.Infof("Install karmada search api server on karmada control plane successfully")
	return nil
}

const (
	etcdServerArgPrefix          = "--etcd-servers="
	etcdServerArgPrefixLength    = len(etcdServerArgPrefix)
	etcdKeyPrefixArgPrefix       = "--etcd-prefix="
	etcdKeyPrefixArgPrefixLength = len(etcdKeyPrefixArgPrefix)
)

func getExternalEtcdServerConfig(ctx context.Context, host kubernetes.Interface, namespace string) (servers, prefix string, err error) {
	var apiserver *appsv1.Deployment
	if apiserver, err = host.AppsV1().Deployments(namespace).Get(
		ctx, karmadaAPIServerDeploymentAndServiceName, metav1.GetOptions{}); err != nil {
		return
	}
	// should be only one container, but it may be injected others by mutating webhook of host cluster,
	// anyway, a for can handle all cases.
	var apiServerContainer *corev1.Container
	for i, container := range apiserver.Spec.Template.Spec.Containers {
		if container.Name == karmadaAPIServerDeploymentAndServiceName {
			apiServerContainer = &apiserver.Spec.Template.Spec.Containers[i]
			break
		}
	}
	if apiServerContainer == nil {
		return
	}
	for _, cmd := range apiServerContainer.Command {
		if strings.HasPrefix(cmd, etcdServerArgPrefix) {
			servers = cmd[etcdServerArgPrefixLength:]
		} else if strings.HasPrefix(cmd, etcdKeyPrefixArgPrefix) {
			prefix = cmd[etcdKeyPrefixArgPrefixLength:]
		}
		if servers != "" && prefix != "" {
			break
		}
	}
	return
}

func etcdServers(opts *addoninit.CommandAddonsEnableOption) (string, string, error) {
	ctx := context.TODO()
	sts, err := opts.KubeClientSet.AppsV1().StatefulSets(opts.Namespace).Get(ctx, etcdStatefulSetAndServiceName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if servers, prefix, cfgErr := getExternalEtcdServerConfig(ctx, opts.KubeClientSet, opts.Namespace); cfgErr != nil {
				return "", "", cfgErr
			} else if servers != "" {
				return servers, prefix, nil
			}
		}
		return "", "", err
	}

	etcdReplicas := *sts.Spec.Replicas
	etcdServers := ""

	for v := int32(0); v < etcdReplicas; v++ {
		etcdServers += fmt.Sprintf("https://%s-%v.%s.%s.svc.%s:%v", etcdStatefulSetAndServiceName, v, etcdStatefulSetAndServiceName, opts.Namespace, opts.HostClusterDomain, etcdContainerClientPort) + ","
	}

	return strings.TrimRight(etcdServers, ","), "", nil
}
