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

package webhook

import (
	"fmt"
	"net"
	"net/url"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	corev1lister "k8s.io/client-go/listers/core/v1"
)

// ServiceResolver knows how to convert a service reference into an actual location.
type ServiceResolver interface {
	ResolveEndpoint(namespace, name string, port int32) (*url.URL, error)
}

// NewServiceResolver returns a ServiceResolver that parses service first,
// if service not exist, constructs a service URL from a given namespace and name.
func NewServiceResolver(services corev1lister.ServiceLister) ServiceResolver {
	return &serviceResolver{
		services:        services,
		defaultResolver: webhookutil.NewDefaultServiceResolver(),
	}
}

type serviceResolver struct {
	services        corev1lister.ServiceLister
	defaultResolver ServiceResolver
}

func (r *serviceResolver) ResolveEndpoint(namespace, name string, port int32) (*url.URL, error) {
	svc, err := r.services.Services(namespace).Get(name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return r.defaultResolver.ResolveEndpoint(namespace, name, port)
		}
		return nil, err
	}
	return resolveCluster(svc, port)
}

// resolveCluster parses Service resource to url.
func resolveCluster(svc *corev1.Service, port int32) (*url.URL, error) {
	switch {
	case svc.Spec.Type == corev1.ServiceTypeClusterIP && svc.Spec.ClusterIP == corev1.ClusterIPNone:
		return nil, fmt.Errorf(`cannot route to service with ClusterIP "None"`)
	// use IP from a clusterIP for these service types
	case svc.Spec.Type == corev1.ServiceTypeClusterIP, svc.Spec.Type == corev1.ServiceTypeLoadBalancer, svc.Spec.Type == corev1.ServiceTypeNodePort:
		svcPort, err := findServicePort(svc, port)
		if err != nil {
			return nil, err
		}
		return &url.URL{
			Scheme: "https",
			Host:   net.JoinHostPort(svc.Spec.ClusterIP, fmt.Sprintf("%d", svcPort.Port)),
		}, nil
	case svc.Spec.Type == corev1.ServiceTypeExternalName:
		return &url.URL{
			Scheme: "https",
			Host:   net.JoinHostPort(svc.Spec.ExternalName, fmt.Sprintf("%d", port)),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported service type %q", svc.Spec.Type)
	}
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
