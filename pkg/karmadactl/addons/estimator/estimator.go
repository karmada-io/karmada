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

package estimator

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	addoninit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	addonutils "github.com/karmada-io/karmada/pkg/karmadactl/addons/utils"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// AddonEstimator describe the estimator addon command process
var AddonEstimator = &addoninit.Addon{
	Name:    names.KarmadaSchedulerEstimatorComponentName,
	Status:  status,
	Enable:  enableEstimator,
	Disable: disableEstimator,
}

var status = func(opts *addoninit.CommandAddonsListOption) (string, error) {
	if len(opts.Cluster) == 0 {
		return addoninit.AddonUnknownStatus, nil
	}

	esName := names.GenerateEstimatorDeploymentName(opts.Cluster)
	deployment, err := opts.KubeClientSet.AppsV1().Deployments(opts.Namespace).Get(context.TODO(), esName, metav1.GetOptions{})
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

	return addoninit.AddonEnabledStatus, nil
}

var enableEstimator = func(opts *addoninit.CommandAddonsEnableOption) error {
	if len(opts.Cluster) == 0 {
		klog.Warning("Cluster is not specified in CommandAddonsEnableOption, estimator installation will skip.")
		return nil
	}

	pathOptions := &clientcmd.PathOptions{
		LoadingRules: &clientcmd.ClientConfigLoadingRules{
			ExplicitPath: opts.MemberKubeConfig,
		},
	}
	config, err := pathOptions.GetStartingConfig()
	if err != nil {
		return err
	}
	config.CurrentContext = opts.MemberContext
	configBytes, err := clientcmd.Write(*config)
	if err != nil {
		return fmt.Errorf("failure while serializing admin kubeConfig. %v", err)
	}

	secretName := fmt.Sprintf("%s-kubeconfig", opts.Cluster)
	secret := secretFromSpec(secretName, opts.Namespace, corev1.SecretTypeOpaque, map[string]string{secretName: string(configBytes)})
	if err := cmdutil.CreateOrUpdateSecret(opts.KubeClientSet, secret); err != nil {
		return fmt.Errorf("create or update scheduler estimator secret error: %v", err)
	}

	// init estimator service
	karmadaEstimatorServiceBytes, err := addonutils.ParseTemplate(karmadaEstimatorService, ServiceReplace{
		Namespace:         opts.Namespace,
		MemberClusterName: opts.Cluster,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada scheduler estimator service template :%v", err)
	}

	karmadaEstimatorService := &corev1.Service{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaEstimatorServiceBytes, karmadaEstimatorService); err != nil {
		return fmt.Errorf("decode karmada scheduler estimator service error: %v", err)
	}
	if err := cmdutil.CreateService(opts.KubeClientSet, karmadaEstimatorService); err != nil {
		return fmt.Errorf("create or update scheduler estimator service error: %v", err)
	}

	// init estimator deployment
	karmadaEstimatorDeploymentBytes, err := addonutils.ParseTemplate(karmadaEstimatorDeployment, DeploymentReplace{
		Namespace:         opts.Namespace,
		Replicas:          &opts.KarmadaEstimatorReplicas,
		Image:             addoninit.KarmadaSchedulerEstimatorImage(opts),
		MemberClusterName: opts.Cluster,
		PriorityClassName: opts.EstimatorPriorityClass,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada scheduler estimator deployment template :%v", err)
	}

	karmadaEstimatorDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaEstimatorDeploymentBytes, karmadaEstimatorDeployment); err != nil {
		return fmt.Errorf("decode karmada scheduler estimator deployment error: %v", err)
	}
	if err := cmdutil.CreateOrUpdateDeployment(opts.KubeClientSet, karmadaEstimatorDeployment); err != nil {
		return fmt.Errorf("create or update scheduler estimator deployment error: %v", err)
	}

	if err := addonutils.WaitForDeploymentRollout(opts.KubeClientSet, karmadaEstimatorDeployment, opts.WaitComponentReadyTimeout); err != nil {
		klog.Warning(err)
	}
	klog.Infof("Karmada scheduler estimator of member cluster %s is installed successfully.", opts.Cluster)

	return nil
}

var disableEstimator = func(opts *addoninit.CommandAddonsDisableOption) error {
	if len(opts.Cluster) == 0 {
		klog.Warning("Cluster is not specified in CommandAddonsDisableOption, estimator uninstallation will skip.")
		return nil
	}

	//delete deployment
	deployClient := opts.KubeClientSet.AppsV1().Deployments(opts.Namespace)
	if err := deployClient.Delete(context.TODO(), fmt.Sprintf("%s-%s", names.KarmadaSchedulerEstimatorComponentName, opts.Cluster), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		klog.Exitln(err)
	}

	// delete service
	serviceClient := opts.KubeClientSet.CoreV1().Services(opts.Namespace)
	if err := serviceClient.Delete(context.TODO(), fmt.Sprintf("%s-%s", names.KarmadaSchedulerEstimatorComponentName, opts.Cluster), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		klog.Exitln(err)
	}

	// delete secret
	secretClient := opts.KubeClientSet.CoreV1().Secrets(opts.Namespace)
	if err := secretClient.Delete(context.TODO(), fmt.Sprintf("%s-kubeconfig", opts.Cluster), metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		klog.Exitln(err)
	}

	klog.Infof("Karmada scheduler estimator of member cluster %s is removed successfully.", opts.Cluster)
	return nil
}

func secretFromSpec(name string, namespace string, secretType corev1.SecretType, data map[string]string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		//Immutable:  immutable,
		Type:       secretType,
		StringData: data,
	}
}
