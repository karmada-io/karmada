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

package apply

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	fakekarmadaclient "github.com/karmada-io/karmada/pkg/generated/clientset/versioned/fake"
	"github.com/karmada-io/karmada/pkg/util/names"
)

func TestValidateApplyCommand(t *testing.T) {
	tests := []struct {
		name      string
		applyOpts *CommandApplyOptions
		prep      func(karmadaclientset.Interface) error
		wantErr   bool
		errMsg    string
	}{
		{
			name: "Validate_WithAllClustersAndClusterArgs_CanNotBeUsedTogether",
			applyOpts: &CommandApplyOptions{
				AllClusters: true,
				Clusters:    []string{"member1", "member2"},
			},
			prep:    func(karmadaclientset.Interface) error { return nil },
			wantErr: true,
			errMsg:  "--all-clusters and --cluster cannot be used together",
		},
		{
			name: "Validate_WithNonExistentCluster_ClusterDoesNotExist",
			applyOpts: &CommandApplyOptions{
				Clusters:      []string{"member1"},
				karmadaClient: fakekarmadaclient.NewSimpleClientset(),
			},
			prep:    func(karmadaclientset.Interface) error { return nil },
			wantErr: true,
			errMsg:  "cluster member1 does not exist",
		},
		{
			name: "Validate_WithExistentCluster_Validated",
			applyOpts: &CommandApplyOptions{
				Clusters:      []string{"member1"},
				karmadaClient: fakekarmadaclient.NewSimpleClientset(),
			},
			prep: func(client karmadaclientset.Interface) error {
				_, err := client.ClusterV1alpha1().Clusters().Create(context.TODO(), &clusterv1alpha1.Cluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "member1",
					},
				}, metav1.CreateOptions{})
				if err != nil {
					return fmt.Errorf("failed to create cluster %s, got: %v", "member1", err)
				}
				return nil
			},
			wantErr: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.applyOpts.karmadaClient); err != nil {
				t.Fatalf("failed tp prep test environment before validating apply options, got: %v", err)
			}
			err := test.applyOpts.Validate()
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}

func TestGeneratePropagationObject(t *testing.T) {
	tests := []struct {
		name      string
		applyOpts *CommandApplyOptions
		info      *resource.Info
		policy    runtime.Object
		prep      func(runtime.Object, *resource.Info) error
	}{
		{
			name: "GenerationPropagationObject_WithNamespace_PropagationPolicyGenerated",
			applyOpts: &CommandApplyOptions{
				Clusters: []string{"member1", "member2"},
			},
			info: &resource.Info{
				Name:      "example-policy",
				Namespace: "test",
				Mapping: &meta.RESTMapping{
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "deployment",
					},
					Scope: meta.RESTScopeNamespace,
				},
			},
			policy: &policyv1alpha1.PropagationPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "policy.karmada.io/v1alpha1",
					Kind:       "PropagationPolicy",
				},
				Spec: policyv1alpha1.PropagationSpec{
					Placement: policyv1alpha1.Placement{
						ClusterAffinity: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"member1", "member2"},
						},
					},
				},
			},
			prep: func(obj runtime.Object, info *resource.Info) error {
				policy := obj.(*policyv1alpha1.PropagationPolicy)
				resourceSelectors, metadata, err := initializePropagationPolicy(info)
				if err != nil {
					return fmt.Errorf("failed to initialize propagation specification, got: %v", err)
				}
				policy.Spec.ResourceSelectors = resourceSelectors
				policy.ObjectMeta = metadata
				return nil
			},
		},
		{
			name: "GenerationPropagationObject_WithoutNamespace_ClusterPropagationPolicyGenerated",
			applyOpts: &CommandApplyOptions{
				Clusters: []string{"member1", "member2"},
			},
			info: &resource.Info{
				Name: "example-policy",
				Mapping: &meta.RESTMapping{
					GroupVersionKind: schema.GroupVersionKind{
						Group:   "apps",
						Version: "v1",
						Kind:    "deployment",
					},
					Scope: meta.RESTScopeRoot,
				},
			},
			policy: &policyv1alpha1.ClusterPropagationPolicy{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "policy.karmada.io/v1alpha1",
					Kind:       "ClusterPropagationPolicy",
				},
				Spec: policyv1alpha1.PropagationSpec{
					Placement: policyv1alpha1.Placement{
						ClusterAffinity: &policyv1alpha1.ClusterAffinity{
							ClusterNames: []string{"member1", "member2"},
						},
					},
				},
			},
			prep: func(obj runtime.Object, info *resource.Info) error {
				policy := obj.(*policyv1alpha1.ClusterPropagationPolicy)
				resourceSelectors, metadata, err := initializePropagationPolicy(info)
				if err != nil {
					return fmt.Errorf("failed to initialize propagation specification, got: %v", err)
				}
				policy.Spec.ResourceSelectors = resourceSelectors
				policy.ObjectMeta = metadata
				return nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.prep(test.policy, test.info); err != nil {
				t.Fatalf("failed to prep for generation propagation object, got: %v", err)
			}
			obj := test.applyOpts.generatePropagationObject(test.info)
			if !reflect.DeepEqual(obj, test.policy) {
				t.Errorf("expected policy object %v to be the same as %v", obj, test.policy)
			}
		})
	}
}

// initializePropagationPolicy initializes the propagation policy's resource selectors
// and metadata based on the provided resource information.
func initializePropagationPolicy(info *resource.Info) ([]policyv1alpha1.ResourceSelector, metav1.ObjectMeta, error) {
	gvk := info.Mapping.GroupVersionKind
	resourceSelectors := []policyv1alpha1.ResourceSelector{
		{
			APIVersion: gvk.GroupVersion().String(),
			Kind:       gvk.Kind,
			Name:       info.Name,
			Namespace:  info.Namespace,
		},
	}
	metadata := metav1.ObjectMeta{
		Name:      names.GeneratePolicyName(info.Namespace, info.Name, gvk.String()),
		Namespace: info.Namespace,
	}
	return resourceSelectors, metadata, nil
}
