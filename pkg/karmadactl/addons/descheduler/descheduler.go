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

package descheduler

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"

	addoninit "github.com/karmada-io/karmada/pkg/karmadactl/addons/init"
	addonutils "github.com/karmada-io/karmada/pkg/karmadactl/addons/utils"
	cmdutil "github.com/karmada-io/karmada/pkg/karmadactl/util"
	"github.com/karmada-io/karmada/pkg/util/names"
)

// AddonDescheduler describe the descheduler addon command process
var AddonDescheduler = &addoninit.Addon{
	Name:    names.KarmadaDeschedulerComponentName,
	Status:  status,
	Enable:  enableDescheduler,
	Disable: disableDescheduler,
}

var status = func(opts *addoninit.CommandAddonsListOption) (string, error) {
	deployment, err := opts.KubeClientSet.AppsV1().Deployments(opts.Namespace).Get(context.TODO(), names.KarmadaDeschedulerComponentName, metav1.GetOptions{})
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

var enableDescheduler = func(opts *addoninit.CommandAddonsEnableOption) error {
	// install karmada descheduler deployment on host cluster
	karmadaDeschedulerDeploymentBytes, err := addonutils.ParseTemplate(karmadaDeschedulerDeployment, DeploymentReplace{
		Namespace:         opts.Namespace,
		Replicas:          &opts.KarmadaDeschedulerReplicas,
		Image:             addoninit.KarmadaDeschedulerImage(opts),
		PriorityClassName: opts.KarmadaDeschedulerPriorityClass,
	})
	if err != nil {
		return fmt.Errorf("error when parsing karmada descheduler deployment template :%v", err)
	}

	karmadaDeschedulerDeployment := &appsv1.Deployment{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), karmadaDeschedulerDeploymentBytes, karmadaDeschedulerDeployment); err != nil {
		return fmt.Errorf("decode descheduler deployment error: %v", err)
	}

	if err := cmdutil.CreateOrUpdateDeployment(opts.KubeClientSet, karmadaDeschedulerDeployment); err != nil {
		return fmt.Errorf("create karmada descheduler deployment error: %v", err)
	}

	if err := addonutils.WaitForDeploymentRollout(opts.KubeClientSet, karmadaDeschedulerDeployment, opts.WaitComponentReadyTimeout); err != nil {
		return fmt.Errorf("wait karmada descheduler pod timeout: %v", err)
	}

	klog.Infof("Install karmada descheduler deployment on host cluster successfully")
	return nil
}

var disableDescheduler = func(opts *addoninit.CommandAddonsDisableOption) error {
	// uninstall karmada descheduler deployment on host cluster
	deployClient := opts.KubeClientSet.AppsV1().Deployments(opts.Namespace)
	if err := deployClient.Delete(context.TODO(), names.KarmadaDeschedulerComponentName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	klog.Infof("Uninstall karmada descheduler deployment on host cluster successfully")
	return nil
}
