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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/util/interpreter/validation"
)

// CreateResourceInterpreterContext returns the unique request uid, the ResourceInterpreterContext object to send the webhook,
// or an error if the webhook does not support receiving any of the versions we know to send.
func CreateResourceInterpreterContext(versions []string, attributes *Attributes) (uid types.UID, request runtime.Object, err error) {
	for _, version := range versions {
		switch version {
		case configv1alpha1.GroupVersion.Version:
			uid = uuid.NewUUID()
			request = CreateV1alpha1ResourceInterpreterContext(uid, attributes)
			return
		}
	}

	err = fmt.Errorf("webhook does not accept known ResourceInterpreterContext versions (v1alpha1)")
	return
}

// CreateV1alpha1ResourceInterpreterContext creates an ResourceInterpreterContext for the provided RequestAttributes.
func CreateV1alpha1ResourceInterpreterContext(uid types.UID, attributes *Attributes) *configv1alpha1.ResourceInterpreterContext {
	r := &configv1alpha1.ResourceInterpreterContext{
		Request: &configv1alpha1.ResourceInterpreterRequest{
			UID: uid,
			Kind: metav1.GroupVersionKind{
				Group:   attributes.Object.GroupVersionKind().Group,
				Version: attributes.Object.GroupVersionKind().Version,
				Kind:    attributes.Object.GetKind(),
			},
			Name:      attributes.Object.GetName(),
			Namespace: attributes.Object.GetNamespace(),
			Operation: attributes.Operation,
			Object: runtime.RawExtension{
				Object: attributes.Object.DeepCopyObject(),
			},
			DesiredReplicas:  &attributes.ReplicasSet,
			AggregatedStatus: attributes.AggregatedStatus,
		},
	}

	if attributes.ObservedObj != nil {
		r.Request.ObservedObject = &runtime.RawExtension{Object: attributes.ObservedObj.DeepCopyObject()}
	}
	return r
}

// VerifyResourceInterpreterContext checks the validity of the provided resourceInterpreterContext, and returns ResponseAttributes,
// or an error if the provided resourceInterpreterContext was not valid.
func VerifyResourceInterpreterContext(uid types.UID, operation configv1alpha1.InterpreterOperation, interpreterContext runtime.Object) (response *ResponseAttributes, err error) {
	switch r := interpreterContext.(type) {
	case *configv1alpha1.ResourceInterpreterContext:
		if r.Response == nil {
			return nil, fmt.Errorf("webhook response was absent")
		}

		if r.Response.UID != uid {
			return nil, fmt.Errorf("expected response.uid %q, got %q", uid, r.Response.UID)
		}

		res, err := verifyResourceInterpreterContext(operation, r.Response)
		if err != nil {
			return nil, err
		}
		return res, nil
	default:
		return nil, fmt.Errorf("unexpected response type %T", interpreterContext)
	}
}

func verifyResourceInterpreterContext(operation configv1alpha1.InterpreterOperation, response *configv1alpha1.ResourceInterpreterResponse) (*ResponseAttributes, error) {
	res := &ResponseAttributes{}

	res.Successful = response.Successful
	if !response.Successful {
		if response.Status == nil {
			return nil, fmt.Errorf("webhook require response.status when response.successful is false")
		}
		res.Status = *response.Status
		return res, nil
	}

	switch operation {
	case configv1alpha1.InterpreterOperationInterpretReplica:
		if response.Replicas == nil {
			return nil, fmt.Errorf("webhook returned nil response.replicas")
		}
		res.Replicas = *response.Replicas
		res.ReplicaRequirements = response.ReplicaRequirements
		return res, nil
	case configv1alpha1.InterpreterOperationInterpretDependency:
		err := validation.VerifyDependencies(response.Dependencies)
		if err != nil {
			return nil, err
		}
		res.Dependencies = response.Dependencies
		return res, nil
	case configv1alpha1.InterpreterOperationPrune, configv1alpha1.InterpreterOperationReviseReplica,
		configv1alpha1.InterpreterOperationRetain, configv1alpha1.InterpreterOperationAggregateStatus:
		err := verifyResourceInterpreterContextWithPatch(response)
		if err != nil {
			return nil, err
		}
		res.Patch = response.Patch
		if response.PatchType != nil {
			res.PatchType = *response.PatchType
		}
		return res, nil
	case configv1alpha1.InterpreterOperationInterpretStatus:
		if response.RawStatus == nil {
			return nil, fmt.Errorf("webhook returned nill response.rawStatus")
		}
		res.RawStatus = *response.RawStatus
		return res, nil
	case configv1alpha1.InterpreterOperationInterpretHealth:
		if response.Healthy == nil {
			return nil, fmt.Errorf("webhook returned nil response.healthy")
		}
		res.Healthy = *response.Healthy
		return res, nil
	default:
		return nil, fmt.Errorf("input wrong operation type: %s", operation)
	}
}

func verifyResourceInterpreterContextWithPatch(response *configv1alpha1.ResourceInterpreterResponse) error {
	if len(response.Patch) == 0 && response.PatchType == nil {
		return nil
	}
	if response.PatchType == nil {
		return fmt.Errorf("webhook returned nil response.patchType")
	}
	if len(response.Patch) == 0 {
		return fmt.Errorf("webhook returned empty response.patch")
	}
	patchType := *response.PatchType
	if patchType != configv1alpha1.PatchTypeJSONPatch {
		return fmt.Errorf("webhook returned invalid response.patchType of %q", patchType)
	}
	return nil
}
