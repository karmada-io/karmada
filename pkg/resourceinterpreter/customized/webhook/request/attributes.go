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

package request

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
)

// Attributes Attributes contains the attributes that call webhook.
type Attributes struct {
	Operation        configv1alpha1.InterpreterOperation
	Object           *unstructured.Unstructured
	ObservedObj      *unstructured.Unstructured
	ReplicasSet      int32
	AggregatedStatus []workv1alpha2.AggregatedStatusItem
}

// ResponseAttributes contains the attributes that response by the webhook.
type ResponseAttributes struct {
	Successful          bool
	Status              configv1alpha1.RequestStatus
	Replicas            int32
	ReplicaRequirements *workv1alpha2.ReplicaRequirements
	Dependencies        []configv1alpha1.DependentObjectReference
	Patch               []byte
	PatchType           configv1alpha1.PatchType
	RawStatus           runtime.RawExtension
	Healthy             bool
}
