/*
Copyright 2014 The Kubernetes Authors.

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

package lifted

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	networkingv1alpha1 "github.com/karmada-io/karmada/pkg/apis/networking/v1alpha1"
)

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation_test.go#L591-L824
// +lifted:changed

func TestValidateIngress(t *testing.T) {
	serviceBackend := &networkingv1.IngressServiceBackend{
		Name: "defaultbackend",
		Port: networkingv1.ServiceBackendPort{
			Name:   "",
			Number: 80,
		},
	}
	defaultBackend := networkingv1.IngressBackend{
		Service: serviceBackend,
	}
	pathTypePrefix := networkingv1.PathTypePrefix
	pathTypeImplementationSpecific := networkingv1.PathTypeImplementationSpecific
	pathTypeFoo := networkingv1.PathType("foo")

	baseMci := networkingv1alpha1.MultiClusterIngress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &defaultBackend,
			Rules: []networkingv1.IngressRule{{
				Host: "foo.bar.com",
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/foo",
							PathType: &pathTypeImplementationSpecific,
							Backend:  defaultBackend,
						}},
					},
				},
			}},
		},
		Status: networkingv1alpha1.MultiClusterIngressStatus{
			IngressStatus: networkingv1.IngressStatus{
				LoadBalancer: networkingv1.IngressLoadBalancerStatus{
					Ingress: []networkingv1.IngressLoadBalancerIngress{
						{IP: "127.0.0.1"},
					},
				},
			},
		},
	}

	testCases := map[string]struct {
		tweakIngress       func(mci *networkingv1alpha1.MultiClusterIngress)
		expectErrsOnFields []string
	}{
		"empty path (implementation specific)": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path = ""
			},
			expectErrsOnFields: []string{},
		},
		"valid path": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path = "/valid"
			},
			expectErrsOnFields: []string{},
		},
		// invalid use cases
		"backend with no service": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.DefaultBackend.Service.Name = ""
			},
			expectErrsOnFields: []string{
				"spec.defaultBackend.service.name",
			},
		},
		"invalid path type": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].PathType = &pathTypeFoo
			},
			expectErrsOnFields: []string{
				"spec.rules[0].http.paths[0].pathType",
			},
		},
		"empty path (prefix)": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Path = ""
				mci.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].PathType = &pathTypePrefix
			},
			expectErrsOnFields: []string{
				"spec.rules[0].http.paths[0].path",
			},
		},
		"no paths": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].IngressRuleValue.HTTP.Paths = []networkingv1.HTTPIngressPath{}
			},
			expectErrsOnFields: []string{
				"spec.rules[0].http.paths",
			},
		},
		"invalid host (foobar:80)": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].Host = "foobar:80"
			},
			expectErrsOnFields: []string{
				"spec.rules[0].host",
			},
		},
		"invalid host (127.0.0.1)": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].Host = "127.0.0.1"
			},
			expectErrsOnFields: []string{
				"spec.rules[0].host",
			},
		},
		"valid wildcard host": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].Host = "*.bar.com"
			},
			expectErrsOnFields: []string{},
		},
		"invalid wildcard host (foo.*.bar.com)": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].Host = "foo.*.bar.com"
			},
			expectErrsOnFields: []string{
				"spec.rules[0].host",
			},
		},
		"invalid wildcard host (*)": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].Host = "*"
			},
			expectErrsOnFields: []string{
				"spec.rules[0].host",
			},
		},
		"path resource backend and service name are not allowed together": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].IngressRuleValue = networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/foo",
							PathType: &pathTypeImplementationSpecific,
							Backend: networkingv1.IngressBackend{
								Service: serviceBackend,
								Resource: &corev1.TypedLocalObjectReference{
									APIGroup: ptr.To("example.com"),
									Kind:     "foo",
									Name:     "bar",
								},
							},
						}},
					},
				}
			},
			expectErrsOnFields: []string{
				"spec.rules[0].http.paths[0].backend",
			},
		},
		"path resource backend and service port are not allowed together": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.Rules[0].IngressRuleValue = networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: []networkingv1.HTTPIngressPath{{
							Path:     "/foo",
							PathType: &pathTypeImplementationSpecific,
							Backend: networkingv1.IngressBackend{
								Service: serviceBackend,
								Resource: &corev1.TypedLocalObjectReference{
									APIGroup: ptr.To("example.com"),
									Kind:     "foo",
									Name:     "bar",
								},
							},
						}},
					},
				}
			},
			expectErrsOnFields: []string{
				"spec.rules[0].http.paths[0].backend",
			},
		},
		"spec.backend resource and service name are not allowed together": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.DefaultBackend = &networkingv1.IngressBackend{
					Service: serviceBackend,
					Resource: &corev1.TypedLocalObjectReference{
						APIGroup: ptr.To("example.com"),
						Kind:     "foo",
						Name:     "bar",
					},
				}
			},
			expectErrsOnFields: []string{
				"spec.defaultBackend",
			},
		},
		"spec.backend resource and service port are not allowed together": {
			tweakIngress: func(mci *networkingv1alpha1.MultiClusterIngress) {
				mci.Spec.DefaultBackend = &networkingv1.IngressBackend{
					Service: serviceBackend,
					Resource: &corev1.TypedLocalObjectReference{
						APIGroup: ptr.To("example.com"),
						Kind:     "foo",
						Name:     "bar",
					},
				}
			},
			expectErrsOnFields: []string{
				"spec.defaultBackend",
			},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			mci := baseMci.DeepCopy()
			testCase.tweakIngress(mci)
			errs := ValidateIngressSpec(&mci.Spec, field.NewPath("spec"), IngressValidationOptions{})
			if len(testCase.expectErrsOnFields) != len(errs) {
				t.Fatalf("Expected %d errors, got %d errors: %v", len(testCase.expectErrsOnFields), len(errs), errs)
			}
			for i, err := range errs {
				if err.Field != testCase.expectErrsOnFields[i] {
					t.Errorf("Expected error on field: %s, got: %s", testCase.expectErrsOnFields[i], err.Error())
				}
			}
		})
	}
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation_test.go#L1747-L1836

func TestValidateIngressTLS(t *testing.T) {
	pathTypeImplementationSpecific := networkingv1.PathTypeImplementationSpecific
	serviceBackend := &networkingv1.IngressServiceBackend{
		Name: "defaultbackend",
		Port: networkingv1.ServiceBackendPort{
			Number: 80,
		},
	}
	defaultBackend := networkingv1.IngressBackend{
		Service: serviceBackend,
	}
	newValid := func() networkingv1alpha1.MultiClusterIngress {
		return networkingv1alpha1.MultiClusterIngress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: metav1.NamespaceDefault,
			},
			Spec: networkingv1.IngressSpec{
				DefaultBackend: &defaultBackend,
				Rules: []networkingv1.IngressRule{{
					Host: "foo.bar.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:     "/foo",
								PathType: &pathTypeImplementationSpecific,
								Backend:  defaultBackend,
							}},
						},
					},
				}},
			},
			Status: networkingv1alpha1.MultiClusterIngressStatus{
				IngressStatus: networkingv1.IngressStatus{
					LoadBalancer: networkingv1.IngressLoadBalancerStatus{
						Ingress: []networkingv1.IngressLoadBalancerIngress{
							{IP: "127.0.0.1"},
						},
					},
				},
			},
		}
	}

	errorCases := map[string]networkingv1alpha1.MultiClusterIngress{}

	wildcardHost := "foo.*.bar.com"
	badWildcardTLS := newValid()
	badWildcardTLS.Spec.Rules[0].Host = "*.foo.bar.com"
	badWildcardTLS.Spec.TLS = []networkingv1.IngressTLS{{
		Hosts: []string{wildcardHost},
	}}
	badWildcardTLSErr := fmt.Sprintf("spec.tls[0].hosts[0]: Invalid value: '%v'", wildcardHost)
	errorCases[badWildcardTLSErr] = badWildcardTLS

	for k, v := range errorCases {
		errs := ValidateIngressSpec(&v.Spec, field.NewPath("spec"), IngressValidationOptions{})
		if len(errs) == 0 {
			t.Errorf("expected failure for %q", k)
		} else {
			s := strings.Split(k, ":")
			err := errs[0]
			if err.Field != s[0] || !strings.Contains(err.Error(), s[1]) {
				t.Errorf("unexpected error: %q, expected: %q", err, k)
			}
		}
	}

	// Test for wildcard host and wildcard TLS
	validCases := map[string]networkingv1alpha1.MultiClusterIngress{}
	wildHost := "*.bar.com"
	goodWildcardTLS := newValid()
	goodWildcardTLS.Spec.Rules[0].Host = "*.bar.com"
	goodWildcardTLS.Spec.TLS = []networkingv1.IngressTLS{{
		Hosts: []string{wildHost},
	}}
	validCases[fmt.Sprintf("spec.tls[0].hosts: Valid value: '%v'", wildHost)] = goodWildcardTLS
	for k, v := range validCases {
		errs := ValidateIngressSpec(&v.Spec, field.NewPath("spec"), IngressValidationOptions{})
		if len(errs) != 0 {
			t.Errorf("expected success for %q", k)
		}
	}
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation_test.go#L1838-L1897

// TestValidateEmptyIngressTLS verifies that an empty TLS configuration can be
// specified, which ingress controllers may interpret to mean that TLS should be
// used with a default certificate that the ingress controller furnishes.
func TestValidateEmptyIngressTLS(t *testing.T) {
	pathTypeImplementationSpecific := networkingv1.PathTypeImplementationSpecific
	serviceBackend := &networkingv1.IngressServiceBackend{
		Name: "defaultbackend",
		Port: networkingv1.ServiceBackendPort{
			Number: 443,
		},
	}
	defaultBackend := networkingv1.IngressBackend{
		Service: serviceBackend,
	}
	newValid := func() networkingv1alpha1.MultiClusterIngress {
		return networkingv1alpha1.MultiClusterIngress{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "foo",
				Namespace: metav1.NamespaceDefault,
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{
					Host: "foo.bar.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								PathType: &pathTypeImplementationSpecific,
								Backend:  defaultBackend,
							}},
						},
					},
				}},
			},
		}
	}

	validCases := map[string]networkingv1alpha1.MultiClusterIngress{}
	goodEmptyTLS := newValid()
	goodEmptyTLS.Spec.TLS = []networkingv1.IngressTLS{
		{},
	}
	validCases[fmt.Sprintf("spec.tls[0]: Valid value: %v", goodEmptyTLS.Spec.TLS[0])] = goodEmptyTLS
	goodEmptyHosts := newValid()
	goodEmptyHosts.Spec.TLS = []networkingv1.IngressTLS{{
		Hosts: []string{},
	}}
	validCases[fmt.Sprintf("spec.tls[0]: Valid value: %v", goodEmptyHosts.Spec.TLS[0])] = goodEmptyHosts
	for k, v := range validCases {
		errs := ValidateIngressSpec(&v.Spec, field.NewPath("spec"), IngressValidationOptions{})
		if len(errs) != 0 {
			t.Errorf("expected success for %q", k)
		}
	}
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation_test.go#L1899-L1991
// +lifted:changed

func TestValidateIngressStatusUpdate(t *testing.T) {
	serviceBackend := &networkingv1.IngressServiceBackend{
		Name: "defaultbackend",
		Port: networkingv1.ServiceBackendPort{
			Number: 80,
		},
	}
	defaultBackend := networkingv1.IngressBackend{
		Service: serviceBackend,
	}

	newValid := func() networkingv1alpha1.MultiClusterIngress {
		return networkingv1alpha1.MultiClusterIngress{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "foo",
				Namespace:       metav1.NamespaceDefault,
				ResourceVersion: "9",
			},
			Spec: networkingv1.IngressSpec{
				DefaultBackend: &defaultBackend,
				Rules: []networkingv1.IngressRule{{
					Host: "foo.bar.com",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{{
								Path:    "/foo",
								Backend: defaultBackend,
							}},
						},
					},
				}},
			},
			Status: networkingv1alpha1.MultiClusterIngressStatus{
				IngressStatus: networkingv1.IngressStatus{
					LoadBalancer: networkingv1.IngressLoadBalancerStatus{
						Ingress: []networkingv1.IngressLoadBalancerIngress{
							{IP: "127.0.0.1", Hostname: "foo.bar.com"},
						},
					},
				},
			},
		}
	}
	newValue := newValid()
	newValue.Status = networkingv1alpha1.MultiClusterIngressStatus{
		IngressStatus: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{
					{IP: "127.0.0.2", Hostname: "foo.com"},
				},
			},
		},
	}

	invalidIP := newValid()
	invalidIP.Status = networkingv1alpha1.MultiClusterIngressStatus{
		IngressStatus: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{
					{IP: "abcd", Hostname: "foo.com"},
				},
			},
		},
	}
	invalidHostname := newValid()
	invalidHostname.Status = networkingv1alpha1.MultiClusterIngressStatus{
		IngressStatus: networkingv1.IngressStatus{
			LoadBalancer: networkingv1.IngressLoadBalancerStatus{
				Ingress: []networkingv1.IngressLoadBalancerIngress{
					{IP: "127.0.0.1", Hostname: "127.0.0.1"},
				},
			},
		},
	}

	errs := ValidateIngressLoadBalancerStatus(&newValue.Status.LoadBalancer, field.NewPath("status", "loadBalancer"))
	if len(errs) != 0 {
		t.Errorf("Unexpected error %v", errs)
	}

	errorCases := map[string]networkingv1alpha1.MultiClusterIngress{
		"status.loadBalancer.ingress[0].ip: Invalid value":       invalidIP,
		"status.loadBalancer.ingress[0].hostname: Invalid value": invalidHostname,
	}
	for k, v := range errorCases {
		errs := ValidateIngressLoadBalancerStatus(&v.Status.LoadBalancer, field.NewPath("status", "loadBalancer"))
		if len(errs) == 0 {
			t.Errorf("expected failure for %s", k)
		} else {
			s := strings.Split(k, ":")
			err := errs[0]
			if err.Field != s[0] || !strings.Contains(err.Error(), s[1]) {
				t.Errorf("unexpected error: %q, expected: %q", err, k)
			}
		}
	}
}
