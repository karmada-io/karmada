/*
Copyright 2025 The Karmada Authors.

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

package helper

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	operatorv1alpha1 "github.com/karmada-io/karmada/operator/pkg/apis/operator/v1alpha1"
)

// NewKarmada returns a new Karmada instance.
func NewKarmada(namespace string, name string) *operatorv1alpha1.Karmada {
	return &operatorv1alpha1.Karmada{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: operatorv1alpha1.KarmadaSpec{
			CRDTarball: &operatorv1alpha1.CRDTarball{
				HTTPSource: &operatorv1alpha1.HTTPSource{URL: "http://local"},
			},
			Components: &operatorv1alpha1.KarmadaComponents{
				Etcd: &operatorv1alpha1.Etcd{},
				KarmadaAggregatedAPIServer: &operatorv1alpha1.KarmadaAggregatedAPIServer{
					CommonSettings: operatorv1alpha1.CommonSettings{
						Image: operatorv1alpha1.Image{
							ImageRepository: "docker.io/karmada/karmada-aggregated-apiserver",
							ImageTag:        "latest",
						},
						Replicas: ptr.To[int32](1),
					},
				},
				KarmadaControllerManager: &operatorv1alpha1.KarmadaControllerManager{
					CommonSettings: operatorv1alpha1.CommonSettings{
						Image: operatorv1alpha1.Image{
							ImageRepository: "docker.io/karmada/karmada-controller-manager",
							ImageTag:        "latest",
						},
						Replicas: ptr.To[int32](1),
					},
				},
				KarmadaScheduler: &operatorv1alpha1.KarmadaScheduler{
					CommonSettings: operatorv1alpha1.CommonSettings{
						Image: operatorv1alpha1.Image{
							ImageRepository: "docker.io/karmada/karmada-scheduler",
							ImageTag:        "latest",
						},
						Replicas: ptr.To[int32](1),
					},
				},
				KarmadaWebhook: &operatorv1alpha1.KarmadaWebhook{
					CommonSettings: operatorv1alpha1.CommonSettings{
						Image: operatorv1alpha1.Image{
							ImageRepository: "docker.io/karmada/karmada-webhook",
							ImageTag:        "latest",
						},
						Replicas: ptr.To[int32](1),
					},
				},
				KarmadaMetricsAdapter: &operatorv1alpha1.KarmadaMetricsAdapter{
					CommonSettings: operatorv1alpha1.CommonSettings{
						Image: operatorv1alpha1.Image{
							ImageRepository: "docker.io/karmada/karmada-metrics-adapter",
							ImageTag:        "latest",
						},
						Replicas: ptr.To[int32](1),
					},
				},
			},
		},
	}
}
