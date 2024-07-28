/*
Copyright 2023 The Karmada Authors.

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

package federatedresourcequotalimit

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/native"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/default/thirdparty"
)

// ValidatingAdmission validates resource templates to ensure there is sufficient capacity in the existing federated resource quota.
type ValidatingAdmission struct {
	client.Client
	Decoder     admission.Decoder
	Interpreter *native.DefaultInterpreter
	ThirdParty  *thirdparty.ConfigurableInterpreter
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Operation == admissionv1.Delete {
		// We only care about Creates and Updates
		return admission.Allowed("")
	}

	// Karmada will update resources with labels and annotations, we should skip validation
	if req.UserInfo.Username == "system:admin" {
		return admission.Allowed("")
	}

	obj := &unstructured.Unstructured{}
	if err := v.Decoder.Decode(req, obj); err != nil {
		klog.V(2).Infof("Error: %s.", err)
		return admission.Errored(http.StatusBadRequest, err)
	}

	klog.V(2).Infof("Validating FederatedResourceQuotaLimit for resource: Kind:%s Name:%s Namespace:%s",
		req.Kind.Kind, obj.GetName(), obj.GetNamespace())

	quotas := &policyv1alpha1.FederatedResourceQuotaList{}
	listOpt := &client.ListOptions{Namespace: obj.GetNamespace()}
	if err := v.List(ctx, quotas, listOpt); err != nil {
		// No resource quota is found, bypass webhook
		if apierrors.IsNotFound(err) {
			return admission.Allowed("")
		}
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !v.Interpreter.HookEnabled(obj.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretReplica) &&
		!v.ThirdParty.HookEnabled(obj.GroupVersionKind(), configv1alpha1.InterpreterOperationInterpretReplica) {
		// if there is no interpreter configured, abort this dependency promotion but continue to promote the resource
		klog.Warningf("There is no default or thirdparty interpreter for %s configured to support interpretReplicas, skipping validation.",
			obj.GroupVersionKind())
		return admission.Allowed("")
	}

	deltaResourceUsage, objResourceUsage, err := v.calculateDeltaUsage(req, obj)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// We get the first quota - FRQ controller only supports one quota per namespace currently.
	// TODO: We can consider adding this optimization if users want to set more than one quota per namespace.
	return checkQuotaLimitsExceeded(&quotas.DeepCopy().Items[0], deltaResourceUsage, objResourceUsage)
}

func (v *ValidatingAdmission) getReplicas(obj *unstructured.Unstructured) (replica int32, requires *workv1alpha2.ReplicaRequirements, err error) {
	var hookEnabled bool
	klog.V(2).Infof("Fetching replicaRequirement using third party interpreter.")
	replica, requires, hookEnabled, err = v.ThirdParty.GetReplicas(obj)
	if err != nil {
		return
	}
	if hookEnabled {
		return
	}
	klog.V(2).Infof("Fetching replicaRequirement using default interpreter.")
	replica, requires, err = v.Interpreter.GetReplicas(obj)
	return
}

func (v *ValidatingAdmission) calculateDeltaUsage(req admission.Request, obj *unstructured.Unstructured) (deltaUsage corev1.ResourceList, objUsage corev1.ResourceList, err error) {
	replicas, replicaRequirements, err := v.getReplicas(obj)
	if err != nil {
		return
	}

	deltaResourceUsage := corev1.ResourceList{}
	for resourceName, resourceQuantity := range replicaRequirements.ResourceRequest {
		resourceQuantity.Mul(int64(replicas))
		deltaResourceUsage[resourceName] = resourceQuantity
	}

	// Making a copy for later use in checking limits
	objResourceUsage := deltaResourceUsage

	if req.Operation == admissionv1.Update {
		oldObj := &unstructured.Unstructured{}
		if err := v.Decoder.DecodeRaw(req.OldObject, oldObj); err != nil {
			return nil, nil, err
		}
		oldReplicas, oldReplicaRequirements, _ := v.getReplicas(oldObj)
		for resourceName, resourceQuantity := range oldReplicaRequirements.ResourceRequest {
			resourceQuantity.Mul(int64(oldReplicas))
			currentUsage := deltaResourceUsage[resourceName]
			currentUsage.Sub(resourceQuantity)
			deltaResourceUsage[resourceName] = currentUsage
		}
	}

	return deltaResourceUsage, objResourceUsage, nil
}

// TODO Update resourceRequest to account for replicas
func checkQuotaLimitsExceeded(quota *policyv1alpha1.FederatedResourceQuota, deltaResourceUsage corev1.ResourceList, objResourceUsage corev1.ResourceList) admission.Response {
	overallUsed := quota.Status.OverallUsed
	overallLimits := quota.Spec.Overall

	for resourceName, resourceQuantity := range deltaResourceUsage {
		klog.V(4).Infof("Checking resourceName: %s.", resourceName)
		// Update quantity to reflect number of replicas
		limitedQuantity, limitExists := overallLimits[resourceName]
		usedQuantity := overallUsed[resourceName]

		// Update usedQuantity += resourceQuantity to reflect footprint
		usedQuantity.Add(resourceQuantity)

		klog.V(4).Infof("Used quantity is: %f.", usedQuantity.AsApproximateFloat64())
		klog.V(4).Infof("Limited quantity is: %f.", limitedQuantity.AsApproximateFloat64())

		if limitExists {
			if usedQuantity.Cmp(limitedQuantity) > 0 {
				usedQuantity.Sub(resourceQuantity)
				requestingQuantity := objResourceUsage[resourceName]
				return admission.Denied(fmt.Sprintf("Exceeded quota: %s, requested: %s, used: %s, limited: %s",
					resourceName, requestingQuantity.String(), usedQuantity.String(), limitedQuantity.String()))
			}
		}
	}
	return admission.Allowed("")
}
