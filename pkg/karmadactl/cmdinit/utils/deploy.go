/*
Copyright 2024 The Karmada Authors.

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

package utils

import (
	"context"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

// CreateDeployAndWait Create deployment and then waiting for ready
func CreateDeployAndWait(kubeClientSet kubernetes.Interface, deployment *appsv1.Deployment, waitComponentReadyTimeout int) error {
	if _, err := kubeClientSet.AppsV1().Deployments(deployment.GetNamespace()).Create(context.TODO(), deployment, metav1.CreateOptions{}); err != nil {
		klog.Warning(err)
	}
	return util.WaitForDeploymentRollout(kubeClientSet, deployment, time.Duration(waitComponentReadyTimeout)*time.Second)
}
