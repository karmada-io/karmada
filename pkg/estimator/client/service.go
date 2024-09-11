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

package client

import (
	"context"
	"fmt"
	"net"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

// ResolveCluster parses Service resource content by itself.
// Fixes Issue https://github.com/karmada-io/karmada/issues/2487
// Modified from "k8s.io/apiserver/pkg/util/proxy/proxy.go:92 => func ResolveCluster"
func resolveCluster(kubeClient kubernetes.Interface, namespace, id string, port int32) ([]string, error) {
	svc, err := kubeClient.CoreV1().Services(namespace).Get(context.TODO(), id, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			/*
			 * When Deploying Karmada in Host Kubernetes Cluster, the kubeClient will connect kube-apiserver
			 * of Karmada Control Plane, rather than of host cluster.
			 * But the Service resource is defined in Host Kubernetes Cluster. So we cannot get its content here.
			 * The best thing we can do is just assemble hosts and ports according to a specific rule, and try to connect to them.
			 */
			return []string{
				net.JoinHostPort(fmt.Sprintf("%s.%s.svc.cluster.local", id, namespace), fmt.Sprintf("%d", port)),
				// To support the environment with a custom DNS suffix.
				net.JoinHostPort(fmt.Sprintf("%s.%s.svc", id, namespace), fmt.Sprintf("%d", port)),
			}, nil
		}

		return nil, err
	}

	if svc.Spec.Type != corev1.ServiceTypeExternalName {
		// We only support ExternalName type here.
		// See discussions in PR: https://github.com/karmada-io/karmada/pull/2574#discussion_r979539389
		return nil, fmt.Errorf("unsupported service type %q", svc.Spec.Type)
	}

	svcPort, err := findServicePort(svc, port)
	if err != nil {
		return nil, err
	}
	if svcPort.TargetPort.Type != intstr.Int {
		return nil, fmt.Errorf("ExternalName service type should have int target port, "+
			"current target port: %v", svcPort.TargetPort)
	}
	return []string{net.JoinHostPort(svc.Spec.ExternalName, fmt.Sprintf("%d", svcPort.TargetPort.IntVal))}, nil
}

// findServicePort finds the service port by name or numerically.
func findServicePort(svc *corev1.Service, port int32) (*corev1.ServicePort, error) {
	for _, svcPort := range svc.Spec.Ports {
		if svcPort.Port == port {
			return &svcPort, nil
		}
	}
	return nil, apierrors.NewServiceUnavailable(fmt.Sprintf("no service port %d found for service %q", port, svc.Name))
}
