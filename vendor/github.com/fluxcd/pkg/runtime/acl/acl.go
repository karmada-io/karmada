/*
Copyright 2021 The Flux authors

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

package acl

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	aclapi "github.com/fluxcd/pkg/apis/acl"
)

// AccessDeniedError represents a failed access control list check.
type AccessDeniedError string

func (e AccessDeniedError) Error() string {
	return string(e)
}

func accessDeniedErrorf(f string, args ...interface{}) error {
	return AccessDeniedError(fmt.Sprintf(f, args...))
}

// IsAccessDenied returns true if the supplied error is an access denied error; e.g., as returned by
// HasAccessToRef.
func IsAccessDenied(e error) bool {
	_, ok := e.(AccessDeniedError)
	return ok
}

// Authorization is an ACL helper for asserting access to cross-namespace references.
type Authorization struct {
	client client.Client
}

// NewAuthorization takes a controller runtime client and returns an Authorization object that allows asserting
// access to cross-namespace references.
func NewAuthorization(kubeClient client.Client) *Authorization {
	return &Authorization{client: kubeClient}
}

// HasAccessToRef checks if a namespaced object has access to a cross-namespace reference based on
// the ACL defined on the referenced object. It returns `nil` if access is possible, or an
// AccessDeniedError if it is not possible; any other kind of error indicates that the check could
// not be completed.
func (a *Authorization) HasAccessToRef(ctx context.Context, object client.Object, reference types.NamespacedName, acl *aclapi.AccessFrom) error {
	// grant access if the object is in the same namespace as the reference
	if reference.Namespace == "" || object.GetNamespace() == reference.Namespace {
		return nil
	}

	// deny access if no ACL is defined on the reference
	if acl == nil {
		return accessDeniedErrorf("'%s/%s' can't be accessed due to missing ACL labels on 'accessFrom'",
			reference.Namespace, reference.Name)
	}

	// get the object's namespace labels
	var sourceNamespace corev1.Namespace
	if err := a.client.Get(ctx, types.NamespacedName{Name: object.GetNamespace()}, &sourceNamespace); err != nil {
		return err
	}
	sourceLabels := sourceNamespace.GetLabels()

	// check if the object's namespace labels match any ACL
	for _, selector := range acl.NamespaceSelectors {
		sel, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: selector.MatchLabels})
		if err != nil {
			return err
		}
		if sel.Matches(labels.Set(sourceLabels)) {
			return nil
		}
	}

	return accessDeniedErrorf("'%s/%s' can't be accessed due to ACL labels mismatch on namespace '%s'",
		reference.Namespace, reference.Name, object.GetNamespace())
}
