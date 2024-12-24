package webhook

import (
	"fmt"
	"net"
	"net/url"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	corev1 "k8s.io/client-go/listers/core/v1"
)

// ServiceResolver knows how to convert a service reference into an actual location.
type ServiceResolver interface {
	ResolveEndpoint(namespace, name string, port int32) (*url.URL, error)
}

// NewServiceResolver returns a ServiceResolver that parses service first,
// if service not exist, constructs a service URL from a given namespace and name.
func NewServiceResolver(services corev1.ServiceLister) ServiceResolver {
	return &serviceResolver{
		services:        services,
		defaultResolver: webhookutil.NewDefaultServiceResolver(),
	}
}

type serviceResolver struct {
	services        corev1.ServiceLister
	defaultResolver ServiceResolver
}

func (r *serviceResolver) ResolveEndpoint(namespace, name string, port int32) (*url.URL, error) {
	res, err := resolveCluster(r.services, namespace, name, port)
	if err != nil && apierrors.IsNotFound(err) {
		return r.defaultResolver.ResolveEndpoint(namespace, name, port)
	}
	return res, err
}

// resolveCluster parses Service resource to url.
// It is lifted from https://github.com/kubernetes/apiserver/blob/release-1.31/pkg/util/proxy/proxy.go#L105.
func resolveCluster(services corev1.ServiceLister, namespace, id string, port int32) (*url.URL, error) {
	svc, err := services.Services(namespace).Get(id)
	if err != nil {
		return nil, err
	}

	switch {
	case svc.Spec.Type == v1.ServiceTypeClusterIP && svc.Spec.ClusterIP == v1.ClusterIPNone:
		return nil, fmt.Errorf(`cannot route to service with ClusterIP "None"`)
	// use IP from a clusterIP for these service types
	case svc.Spec.Type == v1.ServiceTypeClusterIP, svc.Spec.Type == v1.ServiceTypeLoadBalancer, svc.Spec.Type == v1.ServiceTypeNodePort:
		svcPort, err := findServicePort(svc, port)
		if err != nil {
			return nil, err
		}
		return &url.URL{
			Scheme: "https",
			Host:   net.JoinHostPort(svc.Spec.ClusterIP, fmt.Sprintf("%d", svcPort.Port)),
		}, nil
	case svc.Spec.Type == v1.ServiceTypeExternalName:
		return &url.URL{
			Scheme: "https",
			Host:   net.JoinHostPort(svc.Spec.ExternalName, fmt.Sprintf("%d", port)),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported service type %q", svc.Spec.Type)
	}
}

// findServicePort finds the service port by name or numerically.
func findServicePort(svc *v1.Service, port int32) (*v1.ServicePort, error) {
	for _, svcPort := range svc.Spec.Ports {
		if svcPort.Port == port {
			return &svcPort, nil
		}
	}
	return nil, errors.NewServiceUnavailable(fmt.Sprintf("no service port %d found for service %q", port, svc.Name))
}
