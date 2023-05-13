/*
Copyright 2017 The Kubernetes Authors.

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

// This code is directly lifted from the Kubernetes codebase.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/master/pkg/apis/networking/validation/validation.go

package lifted

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	pathvalidation "k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	netutils "k8s.io/utils/net"
)

var validateServiceName = apimachineryvalidation.NameIsDNS1035Label
var validateSecretName = apimachineryvalidation.NameIsDNSSubdomain

var validateIngressClassName = apimachineryvalidation.NameIsDNSSubdomain

var (
	supportedPathTypes = sets.NewString(
		string(networkingv1.PathTypeExact),
		string(networkingv1.PathTypePrefix),
		string(networkingv1.PathTypeImplementationSpecific),
	)
	invalidPathSequences = []string{"//", "/./", "/../", "%2f", "%2F"}
	invalidPathSuffixes  = []string{"/..", "/."}
)

// IngressValidationOptions cover beta to GA transitions for HTTP PathType
type IngressValidationOptions struct {
	// AllowInvalidSecretName indicates whether spec.tls[*].secretName values that are not valid Secret names should be allowed
	AllowInvalidSecretName bool

	// AllowInvalidWildcardHostRule indicates whether invalid rule values are allowed in rules with wildcard hostnames
	AllowInvalidWildcardHostRule bool
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation.go#L326-L348

// ValidateIngressSpec tests if required fields in the IngressSpec are set.
func ValidateIngressSpec(spec *networkingv1.IngressSpec, fldPath *field.Path, opts IngressValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(spec.Rules) == 0 && spec.DefaultBackend == nil {
		errMsg := fmt.Sprintf("either `%s` or `rules` must be specified", "defaultBackend")
		allErrs = append(allErrs, field.Invalid(fldPath, spec.Rules, errMsg))
	}
	if spec.DefaultBackend != nil {
		allErrs = append(allErrs, validateIngressBackend(spec.DefaultBackend, fldPath.Child("defaultBackend"), opts)...)
	}
	if len(spec.Rules) > 0 {
		allErrs = append(allErrs, validateIngressRules(spec.Rules, fldPath.Child("rules"), opts)...)
	}
	if len(spec.TLS) > 0 {
		allErrs = append(allErrs, validateIngressTLS(spec, fldPath.Child("tls"), opts)...)
	}
	if spec.IngressClassName != nil {
		for _, msg := range validateIngressClassName(*spec.IngressClassName, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("ingressClassName"), *spec.IngressClassName, msg))
		}
	}
	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation.go#L468C1-L509

func validateIngressBackend(backend *networkingv1.IngressBackend, fldPath *field.Path, opts IngressValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	hasResourceBackend := backend.Resource != nil
	hasServiceBackend := backend.Service != nil

	switch {
	case hasResourceBackend && hasServiceBackend:
		return append(allErrs, field.Invalid(fldPath, "", "cannot set both resource and service backends"))
	case hasResourceBackend:
		allErrs = append(allErrs, validateIngressTypedLocalObjectReference(backend.Resource, fldPath.Child("resource"))...)
	case hasServiceBackend:
		if len(backend.Service.Name) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("service", "name"), ""))
		} else {
			for _, msg := range validateServiceName(backend.Service.Name, false) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("service", "name"), backend.Service.Name, msg))
			}
		}

		hasPortName := len(backend.Service.Port.Name) > 0
		hasPortNumber := backend.Service.Port.Number != 0
		if hasPortName && hasPortNumber {
			allErrs = append(allErrs, field.Invalid(fldPath, "", "cannot set both port name & port number"))
		} else if hasPortName {
			for _, msg := range validation.IsValidPortName(backend.Service.Port.Name) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("service", "port", "name"), backend.Service.Port.Name, msg))
			}
		} else if hasPortNumber {
			for _, msg := range validation.IsValidPortNum(int(backend.Service.Port.Number)) {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("service", "port", "number"), backend.Service.Port.Number, msg))
			}
		} else {
			allErrs = append(allErrs, field.Required(fldPath, "port name or number is required"))
		}
	default:
		allErrs = append(allErrs, field.Invalid(fldPath, "", "resource or service backend is required"))
	}
	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation.go#L547C1-L578

func validateIngressTypedLocalObjectReference(params *corev1.TypedLocalObjectReference, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if params == nil {
		return allErrs
	}

	if params.APIGroup != nil {
		for _, msg := range validation.IsDNS1123Subdomain(*params.APIGroup) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("apiGroup"), *params.APIGroup, msg))
		}
	}

	if params.Kind == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("kind"), "kind is required"))
	} else {
		for _, msg := range pathvalidation.IsValidPathSegmentName(params.Kind) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("kind"), params.Kind, msg))
		}
	}

	if params.Name == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("name"), "name is required"))
	} else {
		for _, msg := range pathvalidation.IsValidPathSegmentName(params.Name) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("name"), params.Name, msg))
		}
	}

	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation.go#L379-L409

func validateIngressRules(ingressRules []networkingv1.IngressRule, fldPath *field.Path, opts IngressValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(ingressRules) == 0 {
		return append(allErrs, field.Required(fldPath, ""))
	}
	for i, ih := range ingressRules {
		wildcardHost := false
		if len(ih.Host) > 0 {
			if isIP := netutils.ParseIPSloppy(ih.Host) != nil; isIP {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("host"), ih.Host, "must be a DNS name, not an IP address"))
			}
			// TODO: Ports and ips are allowed in the host part of a url
			// according to RFC 3986, consider allowing them.
			if strings.Contains(ih.Host, "*") {
				for _, msg := range validation.IsWildcardDNS1123Subdomain(ih.Host) {
					allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("host"), ih.Host, msg))
				}
				wildcardHost = true
			} else {
				for _, msg := range validation.IsDNS1123Subdomain(ih.Host) {
					allErrs = append(allErrs, field.Invalid(fldPath.Index(i).Child("host"), ih.Host, msg))
				}
			}
		}

		if !wildcardHost || !opts.AllowInvalidWildcardHostRule {
			allErrs = append(allErrs, validateIngressRuleValue(&ih.IngressRuleValue, fldPath.Index(i), opts)...)
		}
	}
	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation.go#L411C1-L417

func validateIngressRuleValue(ingressRule *networkingv1.IngressRuleValue, fldPath *field.Path, opts IngressValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}
	if ingressRule.HTTP != nil {
		allErrs = append(allErrs, validateHTTPIngressRuleValue(ingressRule.HTTP, fldPath.Child("http"), opts)...)
	}
	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation.go#L419-L428

func validateHTTPIngressRuleValue(httpIngressRuleValue *networkingv1.HTTPIngressRuleValue, fldPath *field.Path, opts IngressValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(httpIngressRuleValue.Paths) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("paths"), ""))
	}
	for i := range httpIngressRuleValue.Paths {
		allErrs = append(allErrs, validateHTTPIngressPath(&httpIngressRuleValue.Paths[i], fldPath.Child("paths").Index(i), opts)...)
	}
	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation.go#L430-L466

func validateHTTPIngressPath(path *networkingv1.HTTPIngressPath, fldPath *field.Path, opts IngressValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}

	if path.PathType == nil {
		return append(allErrs, field.Required(fldPath.Child("pathType"), "pathType must be specified"))
	}

	switch *path.PathType {
	case networkingv1.PathTypeExact, networkingv1.PathTypePrefix:
		if !strings.HasPrefix(path.Path, "/") {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("path"), path.Path, "must be an absolute path"))
		}
		if len(path.Path) > 0 {
			for _, invalidSeq := range invalidPathSequences {
				if strings.Contains(path.Path, invalidSeq) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("path"), path.Path, fmt.Sprintf("must not contain '%s'", invalidSeq)))
				}
			}

			for _, invalidSuff := range invalidPathSuffixes {
				if strings.HasSuffix(path.Path, invalidSuff) {
					allErrs = append(allErrs, field.Invalid(fldPath.Child("path"), path.Path, fmt.Sprintf("cannot end with '%s'", invalidSuff)))
				}
			}
		}
	case networkingv1.PathTypeImplementationSpecific:
		if len(path.Path) > 0 {
			if !strings.HasPrefix(path.Path, "/") {
				allErrs = append(allErrs, field.Invalid(fldPath.Child("path"), path.Path, "must be an absolute path"))
			}
		}
	default:
		allErrs = append(allErrs, field.NotSupported(fldPath.Child("pathType"), *path.PathType, supportedPathTypes.List()))
	}
	allErrs = append(allErrs, validateIngressBackend(&path.Backend, fldPath.Child("backend"), opts)...)
	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation.go#L299-L324

func validateIngressTLS(spec *networkingv1.IngressSpec, fldPath *field.Path, opts IngressValidationOptions) field.ErrorList {
	allErrs := field.ErrorList{}
	// TODO: Perform a more thorough validation of spec.TLS.Hosts that takes
	// the wildcard spec from RFC 6125 into account.
	for tlsIndex, itls := range spec.TLS {
		for i, host := range itls.Hosts {
			if strings.Contains(host, "*") {
				for _, msg := range validation.IsWildcardDNS1123Subdomain(host) {
					allErrs = append(allErrs, field.Invalid(fldPath.Index(tlsIndex).Child("hosts").Index(i), host, msg))
				}
				continue
			}
			for _, msg := range validation.IsDNS1123Subdomain(host) {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(tlsIndex).Child("hosts").Index(i), host, msg))
			}
		}

		if !opts.AllowInvalidSecretName {
			for _, msg := range validateTLSSecretName(itls.SecretName) {
				allErrs = append(allErrs, field.Invalid(fldPath.Index(tlsIndex).Child("secretName"), itls.SecretName, msg))
			}
		}
	}

	return allErrs
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation.go#L641-L646

func validateTLSSecretName(name string) []string {
	if len(name) == 0 {
		return nil
	}
	return validateSecretName(name, false)
}

// +lifted:source=https://github.com/kubernetes/kubernetes/blob/release-1.27/pkg/apis/networking/validation/validation.go#L357-L377

// ValidateIngressLoadBalancerStatus validates required fields on an IngressLoadBalancerStatus
func ValidateIngressLoadBalancerStatus(status *networkingv1.IngressLoadBalancerStatus, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	for i, ingress := range status.Ingress {
		idxPath := fldPath.Child("ingress").Index(i)
		if len(ingress.IP) > 0 {
			if isIP := netutils.ParseIPSloppy(ingress.IP) != nil; !isIP {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("ip"), ingress.IP, "must be a valid IP address"))
			}
		}
		if len(ingress.Hostname) > 0 {
			for _, msg := range validation.IsDNS1123Subdomain(ingress.Hostname) {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("hostname"), ingress.Hostname, msg))
			}
			if isIP := netutils.ParseIPSloppy(ingress.Hostname) != nil; isIP {
				allErrs = append(allErrs, field.Invalid(idxPath.Child("hostname"), ingress.Hostname, "must be a DNS name, not an IP address"))
			}
		}
	}
	return allErrs
}
