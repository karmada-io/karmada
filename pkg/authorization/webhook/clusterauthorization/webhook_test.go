/*
Copyright 2021 The Karmada Authors.

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
package clusterauthorization

import (
	"context"
	"errors"
	"reflect"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/karmada-io/karmada/pkg/util/authorization_webhook"
	"github.com/karmada-io/karmada/pkg/webhook/clusterrestriction"
)

func TestAuthorizerAuthorizeResource(t *testing.T) {
	ctx := context.Background()
	testAgentName := "system:node:cluster-1"
	fakeObjects := []client.Object{
		&corev1.Secret{
			// typemeta is required to populate RESTMapper in fake client
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "cluster-1-secret",
				Annotations: map[string]string{
					clusterrestriction.OwnerAnnotationKey: testAgentName,
				},
			},
		},
		&corev1.Secret{
			// typemeta is required to populate RESTMapper in fake client
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "cluster-2-secret",
				Annotations: map[string]string{
					clusterrestriction.OwnerAnnotationKey: "system:node:cluster-2",
				},
			},
		},
		&corev1.Secret{
			// typemeta is required to populate RESTMapper in fake client
			TypeMeta: metav1.TypeMeta{
				Kind:       "Secret",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "no-owner",
			},
		},
	}

	testCases := []struct {
		name               string
		resourceAttributes authorizationv1.ResourceAttributes
		agentOperating     string
		expected           authorization_webhook.Response
	}{
		{
			name: `request to non-existing resource should return no opinion`,
			resourceAttributes: authorizationv1.ResourceAttributes{
				Version:   "v1",
				Resource:  "secrets",
				Namespace: "test",
				Name:      "non-exists",
				Verb:      "get",
			},
			agentOperating: testAgentName,
			expected:       noOpinionForNonExistingResources,
		},
		{
			name: `request to collection(no name in resourceattributes) should return no opinion`,
			resourceAttributes: authorizationv1.ResourceAttributes{
				Version:   "v1",
				Resource:  "secrets",
				Namespace: "test",
				Name:      "",
				Verb:      "list",
			},
			agentOperating: testAgentName,
			expected:       noOpinionForCollectionResources,
		},
		{
			name: `request to single object ownered by itself should allow`,
			resourceAttributes: authorizationv1.ResourceAttributes{
				Version:   "v1",
				Resource:  "secrets",
				Namespace: "test",
				Name:      "cluster-1-secret",
				Verb:      "get",
			},
			agentOperating: testAgentName,
			expected:       authorization_webhook.Allowed(),
		},
		{
			name: `request to single object ownered by another should deny`,
			resourceAttributes: authorizationv1.ResourceAttributes{
				Version:   "v1",
				Resource:  "secrets",
				Namespace: "test",
				Name:      "cluster-2-secret",
				Verb:      "get",
			},
			agentOperating: testAgentName,
			expected:       denyAccessingResourceOwnedByOther("get"),
		},
		{
			name: `request to single object without owner annotation should return no opinion`,
			resourceAttributes: authorizationv1.ResourceAttributes{
				Version:   "v1",
				Resource:  "secrets",
				Namespace: "test",
				Name:      "no-owner",
				Verb:      "get",
			},
			agentOperating: testAgentName,
			expected:       noOpinionResourcesWithoutOwner,
		},
		{
			name: `request to unknown resource should return error`,
			resourceAttributes: authorizationv1.ResourceAttributes{
				Version:   "v1",
				Resource:  "unknown",
				Namespace: "test",
				Name:      "test",
				Verb:      "get",
			},
			agentOperating: testAgentName,
			expected:       authorization_webhook.Errored(errors.New("no matches for /v1, Resource=unknown")),
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			a := &Authorizer{client: createFakeClient(fakeObjects...)}
			got := a.authorizeResourceAccess(
				ctx,
				testCase.resourceAttributes,
				testCase.agentOperating,
			)
			if !reflect.DeepEqual(got, testCase.expected) {
				t.Errorf("want %+v, but got %+v", testCase.expected, got)
			}
		})
	}
}

func TestAuthorizerAuthorizeNonResource(t *testing.T) {
	ctx := context.Background()
	testAgentName := "system:node:cluster-1"
	a := &Authorizer{client: createFakeClient()}
	got := a.authorizeNonResourceAccess(ctx, authorizationv1.NonResourceAttributes{}, testAgentName)
	expected := noOpinionForNonResources
	if !reflect.DeepEqual(got, expected) {
		t.Errorf("want %+v, but got %+v", expected, got)
	}
}

func createFakeClient(fakeObjects ...client.Object) client.Client {
	restMapper := meta.NewDefaultRESTMapper(nil)
	for _, o := range fakeObjects {
		scope := meta.RESTScopeNamespace
		if o.GetNamespace() == "" {
			scope = meta.RESTScopeRoot
		}
		restMapper.Add(o.GetObjectKind().GroupVersionKind(), scope)
	}
	fakeClient := fake.NewClientBuilder().
		WithRESTMapper(restMapper).
		WithObjects(fakeObjects...).
		Build()
	return fakeClient
}
