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

package resourcebinding

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util"
)

// ValidatingAdmission validates ResourceBinding object when creating/updating.
type ValidatingAdmission struct {
	client.Client
	Decoder admission.Decoder
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}

type frqProcessOutcome struct {
	validationMessages []string
	updatedFRQCount    int
	noFRQsFound        bool                // True if this attempt found no FRQs
	earlyExitResponse  *admission.Response // Response if this attempt decided to exit early (denial, permanent error)
	ProcessError       error               // Error for RetryOnConflict (e.g., conflict error or nil for success/final decision)
}

// Handle implements admission.Handler interface.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	if !features.FeatureGate.Enabled(features.FederatedQuotaEnforcement) {
		klog.V(5).Infof("FederatedQuotaEnforcement feature gate is disabled, skipping validation for ResourceBinding %s/%s", req.Namespace, req.Name)
		return admission.Allowed("")
	}

	isDryRun := req.DryRun != nil && *req.DryRun

	rb, oldRB, extractResp := v.decodeAndValidateRBs(req)
	if extractResp != nil {
		return *extractResp
	}

	newRbTotalUsage, oldRbTotalUsage, respCalc := v.calculateRBUsages(rb, oldRB)
	if respCalc != nil {
		return *respCalc
	}

	totalRbDelta := calculateDelta(newRbTotalUsage, oldRbTotalUsage)
	klog.V(4).Infof("Calculated total RB delta for %s/%s: %v", rb.Namespace, rb.Name, totalRbDelta)

	if len(totalRbDelta) == 0 {
		klog.V(2).Infof("No effective resource quantity delta for ResourceBinding %s/%s. Skipping quota validation.", rb.Namespace, rb.Name)
		return admission.Allowed("No effective resource quantity delta for ResourceBinding, skipping quota validation.")
	}

	frqOutcome := v.processFRQsWithRetries(ctx, rb, totalRbDelta, isDryRun)

	return v.finalizeResponse(rb, frqOutcome)
}

func (v *ValidatingAdmission) processFRQsWithRetries(ctx context.Context, rb *workv1alpha2.ResourceBinding, totalRbDelta corev1.ResourceList, isDryRun bool) frqProcessOutcome {
	var overallOutcome frqProcessOutcome

	retrySystemError := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		attemptResult := v.executeFRQProcessingAttempt(ctx, rb, totalRbDelta, isDryRun)

		if attemptResult.ProcessError == nil {
			overallOutcome = attemptResult
		}
		return attemptResult.ProcessError
	})

	overallOutcome.ProcessError = retrySystemError
	return overallOutcome
}

func (v *ValidatingAdmission) executeFRQProcessingAttempt(ctx context.Context, rb *workv1alpha2.ResourceBinding, totalRbDelta corev1.ResourceList, isDryRun bool) frqProcessOutcome {
	outcome := frqProcessOutcome{}
	var currentFrqsToUpdateStatus []*policyv1alpha1.FederatedResourceQuota
	var currentValidationMessages []string

	frqList, listResp := v.listFRQs(ctx, rb.Namespace)
	if listResp != nil {
		outcome.earlyExitResponse = listResp
		return outcome // attemptError is nil, signaling non-retryable handled error
	}

	if len(frqList.Items) == 0 {
		outcome.noFRQsFound = true
		// validationMessages and updatedFRQCount will remain empty/zero.
		return outcome // attemptError is nil, successful attempt (no FRQs to process)
	}

	for _, frqItem := range frqList.Items {
		newStatus, msg, denialResp := v.processSingleFRQ(frqItem, rb.Namespace, rb.Name, totalRbDelta)
		if denialResp != nil {
			outcome.earlyExitResponse = denialResp
			return outcome // attemptError is nil, request denied
		}
		if newStatus != nil {
			newFRQItem := frqItem.DeepCopy()
			newFRQItem.Status.OverallUsed = newStatus
			currentFrqsToUpdateStatus = append(currentFrqsToUpdateStatus, newFRQItem)
		}
		if msg != "" {
			currentValidationMessages = append(currentValidationMessages, msg)
		}
	}

	updateErr := v.updateFRQStatusesWithRetrySignal(ctx, currentFrqsToUpdateStatus, isDryRun, rb.Namespace, rb.Name)
	if updateErr != nil {
		if apierrors.IsConflict(updateErr) {
			klog.V(4).Infof("Conflict detected while updating FRQ status for RB %s/%s, will retry: %v", rb.Namespace, rb.Name, updateErr)
			outcome.ProcessError = updateErr // Signal RetryOnConflict to retry
			return outcome
		}
		errMsg := fmt.Sprintf("permanent error updating FRQ statuses for RB %s/%s: %v", rb.Namespace, rb.Name, updateErr)
		klog.Error(errMsg)
		resp := admission.Errored(http.StatusInternalServerError, errors.New(errMsg))
		outcome.earlyExitResponse = &resp
		return outcome // attemptError is nil, non-retryable handled error
	}

	outcome.validationMessages = currentValidationMessages
	outcome.updatedFRQCount = len(currentFrqsToUpdateStatus)
	return outcome // attemptError is nil, successful attempt
}

func (v *ValidatingAdmission) finalizeResponse(rb *workv1alpha2.ResourceBinding, outcome frqProcessOutcome) admission.Response {
	if outcome.ProcessError != nil {
		errMsg := fmt.Sprintf("failed to apply FederatedResourceQuota updates after multiple retries for RB %s/%s: %v", rb.Namespace, rb.Name, outcome.ProcessError)
		klog.Error(errMsg)
		return admission.Errored(http.StatusInternalServerError, errors.New(errMsg))
	}

	if outcome.earlyExitResponse != nil {
		return *outcome.earlyExitResponse
	}

	if outcome.noFRQsFound {
		klog.V(2).Infof("No FederatedResourceQuotas found in namespace %s for ResourceBinding %s. Allowing operation.", rb.Namespace, rb.Name)
		return admission.Allowed("No FederatedResourceQuotas found in the namespace, skipping quota check.")
	}

	finalMessage := fmt.Sprintf("All relevant FederatedResourceQuota checks passed for ResourceBinding %s/%s.", rb.Namespace, rb.Name)
	if len(outcome.validationMessages) > 0 {
		finalMessage = fmt.Sprintf("%s %s", finalMessage, strings.Join(outcome.validationMessages, " "))
	}
	if outcome.updatedFRQCount > 0 {
		finalMessage = fmt.Sprintf("%s %d FRQ(s) updated.", finalMessage, outcome.updatedFRQCount)
	} else {
		finalMessage = fmt.Sprintf("%s No FRQs required update.", finalMessage)
	}
	klog.V(2).Infof("Admission allowed for ResourceBinding %s/%s: %s", rb.Namespace, rb.Name, finalMessage)
	return admission.Allowed(finalMessage)
}

// updateFRQStatusesWithRetrySignal attempts to update FRQ statuses.
// It returns an error if any update fails. This error can be a conflict error (for retrying by RetryOnConflict)
// or a different error (which will be treated as permanent by the caller within the retry func).
func (v *ValidatingAdmission) updateFRQStatusesWithRetrySignal(ctx context.Context, frqsToUpdateStatus []*policyv1alpha1.FederatedResourceQuota, isDryRun bool, rbNamespace, rbName string) error {
	if len(frqsToUpdateStatus) == 0 {
		klog.V(4).Infof("No FederatedResourceQuotas required an update in this processing attempt for RB %s/%s.", rbNamespace, rbName)
		return nil
	}

	klog.V(3).Infof("Attempting to update %d FederatedResourceQuotas for RB %s/%s (dryRun: %v).", len(frqsToUpdateStatus), rbNamespace, rbName, isDryRun)
	updateOptions := []client.SubResourceUpdateOption{}
	if isDryRun {
		updateOptions = append(updateOptions, client.DryRunAll)
	}

	for _, frqItem := range frqsToUpdateStatus {
		klog.V(4).Infof("Updating status of FRQ %s/%s with OverallUsed: %v for RB %s/%s",
			frqItem.Namespace, frqItem.Name, frqItem.Status.OverallUsed, rbNamespace, rbName)

		if errUpdate := v.Client.Status().Update(ctx, frqItem, updateOptions...); errUpdate != nil {
			klog.Warningf("Failed to UPDATE status of FederatedResourceQuota %s/%s for RB %s/%s: %v. Returning error for retry evaluation.",
				frqItem.Namespace, frqItem.Name, rbNamespace, rbName, errUpdate)
			return errUpdate // Return the original error from Update (could be conflict)
		}

		logMsg := fmt.Sprintf("Successfully updated status of FRQ %s/%s for RB %s/%s.", frqItem.Namespace, frqItem.Name, rbNamespace, rbName)
		if isDryRun {
			logMsg = fmt.Sprintf("Dry run: Successfully simulated status update of FRQ %s/%s for RB %s/%s.", frqItem.Namespace, frqItem.Name, rbNamespace, rbName)
		}
		klog.V(2).Info(logMsg)
	}
	return nil
}

// decodeAndValidateRBs decodes current and old (for updates) ResourceBindings,
// performs essential preliminary checks, and checks for relevant changes in update operations.
// Returns the RBs or an admission response for early exit.
func (v *ValidatingAdmission) decodeAndValidateRBs(req admission.Request) (
	currentRB *workv1alpha2.ResourceBinding,
	oldRB *workv1alpha2.ResourceBinding, // Will be nil if not an update or if not applicable
	earlyExitResp *admission.Response,
) {
	decodedRB := &workv1alpha2.ResourceBinding{}
	if err := v.Decoder.Decode(req, decodedRB); err != nil {
		klog.Errorf("Failed to decode ResourceBinding %s/%s: %v", req.Namespace, req.Name, err)
		resp := admission.Errored(http.StatusBadRequest, err)
		return nil, nil, &resp
	}
	klog.V(2).Infof("Processing ResourceBinding(%s/%s) for request: %s (%s)", decodedRB.Namespace, decodedRB.Name, req.Operation, req.UID)

	if req.Operation == admissionv1.Create && len(decodedRB.Spec.Clusters) == 0 {
		klog.V(4).Infof("ResourceBinding %s/%s is being created but not yet scheduled, skipping quota validation.", decodedRB.Namespace, decodedRB.Name)
		resp := admission.Allowed("ResourceBinding not yet scheduled for Create operation.")
		return decodedRB, nil, &resp
	}

	var decodedOldRB *workv1alpha2.ResourceBinding
	if req.Operation == admissionv1.Update {
		tempOldRB := &workv1alpha2.ResourceBinding{}
		if err := v.Decoder.DecodeRaw(req.OldObject, tempOldRB); err != nil {
			klog.Errorf("Failed to decode old ResourceBinding %s/%s: %v", req.Namespace, req.Name, err)
			resp := admission.Errored(http.StatusBadRequest, err)
			return decodedRB, nil, &resp
		}
		decodedOldRB = tempOldRB

		if !isQuotaRelevantFieldChanged(decodedOldRB, decodedRB) {
			klog.V(4).Infof("ResourceBinding %s/%s updated, but no quota relevant fields changed, skipping quota validation.", decodedRB.Namespace, decodedRB.Name)
			resp := admission.Allowed("No quota relevant fields changed.")
			return decodedRB, decodedOldRB, &resp
		}
	}
	return decodedRB, decodedOldRB, nil
}

func (v *ValidatingAdmission) calculateRBUsages(rb, oldRB *workv1alpha2.ResourceBinding) (corev1.ResourceList, corev1.ResourceList, *admission.Response) {
	newRbTotalUsage, err := calculateResourceUsage(rb)
	if err != nil {
		klog.Errorf("Error calculating resource usage for new ResourceBinding %s/%s: %v", rb.Namespace, rb.Name, err)
		resp := admission.Errored(http.StatusInternalServerError, err)
		return nil, nil, &resp
	}
	klog.V(4).Infof("Calculated total usage for incoming RB %s/%s: %v", rb.Namespace, rb.Name, newRbTotalUsage)

	oldRbTotalUsage := corev1.ResourceList{}
	if oldRB != nil {
		oldRbTotalUsage, err = calculateResourceUsage(oldRB)
		if err != nil {
			klog.Errorf("Error calculating resource usage for old ResourceBinding %s/%s: %v", oldRB.Namespace, oldRB.Name, err)
			resp := admission.Errored(http.StatusInternalServerError, err)
			return nil, nil, &resp
		}
		klog.V(4).Infof("Calculated total usage for old RB %s/%s: %v", oldRB.Namespace, oldRB.Name, oldRbTotalUsage)
	}
	return newRbTotalUsage, oldRbTotalUsage, nil
}

func (v *ValidatingAdmission) listFRQs(ctx context.Context, namespace string) (*policyv1alpha1.FederatedResourceQuotaList, *admission.Response) {
	frqList := &policyv1alpha1.FederatedResourceQuotaList{}
	if err := v.Client.List(ctx, frqList, client.InNamespace(namespace)); err != nil {
		klog.Errorf("Failed to list FederatedResourceQuotas in namespace %s: %v", namespace, err)
		resp := admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed to list FederatedResourceQuotas: %w", err))
		return nil, &resp
	}
	klog.V(2).Infof("Found %d FederatedResourceQuotas in namespace %s to evaluate.", len(frqList.Items), namespace)
	return frqList, nil
}

func (v *ValidatingAdmission) processSingleFRQ(frqItem policyv1alpha1.FederatedResourceQuota, rbNamespace, rbName string, totalRbDelta corev1.ResourceList) (newStatusToSet corev1.ResourceList, validationMsg string, denialResp *admission.Response) {
	klog.V(4).Infof("Evaluating FRQ %s/%s for RB %s/%s", frqItem.Namespace, frqItem.Name, rbNamespace, rbName)
	if frqItem.Spec.Overall == nil {
		klog.V(4).Infof("FRQ %s/%s has no spec.overall defined, skipping its evaluation.", frqItem.Namespace, frqItem.Name)
		return nil, "", nil
	}

	deltaForThisFRQ := filterResourceListByKeys(totalRbDelta, frqItem.Spec.Overall)
	klog.V(4).Infof("Filtered delta for FRQ %s/%s (from total RB delta %v): %v",
		frqItem.Namespace, frqItem.Name, totalRbDelta, deltaForThisFRQ)

	if isResourceListEffectivelyZero(deltaForThisFRQ, frqItem.Spec.Overall) {
		klog.V(4).Infof("No effective resource delta for FRQ %s/%s concerning RB %s/%s. Skipping update for this FRQ.", frqItem.Namespace, frqItem.Name, rbNamespace, rbName)
		return nil, "", nil
	}

	potentialNewOverallUsedForThisFRQ := addResourceLists(frqItem.Status.OverallUsed, deltaForThisFRQ)

	isAllowed, errMsg := isAllowed(potentialNewOverallUsedForThisFRQ, frqItem)
	if !isAllowed {
		klog.Warningf("Quota exceeded for FederatedResourceQuota %s/%s. ResourceBinding %s/%s will be denied.",
			frqItem.Namespace, frqItem.Name, rbNamespace, rbName)
		resp := buildDenyResponse(errMsg)
		return nil, "", resp
	}

	msg := fmt.Sprintf("Quota check passed for FRQ %s/%s.", frqItem.Namespace, frqItem.Name)
	klog.V(3).Infof("FRQ %s/%s will be updated. New OverallUsed: %v", frqItem.Namespace, frqItem.Name, potentialNewOverallUsedForThisFRQ)
	return potentialNewOverallUsedForThisFRQ, msg, nil
}

func buildDenyResponse(errMsg string) *admission.Response {
	resp := admission.Response{
		AdmissionResponse: admissionv1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Message: errMsg,
				Reason:  util.QuotaExceededReason,
				Code:    int32(http.StatusForbidden),
			},
		},
	}
	return &resp
}

func calculateResourceUsage(rb *workv1alpha2.ResourceBinding) (corev1.ResourceList, error) {
	if rb == nil || rb.Spec.ReplicaRequirements == nil || len(rb.Spec.ReplicaRequirements.ResourceRequest) == 0 || len(rb.Spec.Clusters) == 0 {
		return corev1.ResourceList{}, nil
	}

	totalReplicas := int32(0)
	for _, cluster := range rb.Spec.Clusters {
		totalReplicas += cluster.Replicas
	}

	if totalReplicas == 0 {
		return corev1.ResourceList{}, nil
	}
	if totalReplicas < 0 {
		return nil, fmt.Errorf("total replicas in clusters cannot be negative for RB %s/%s: %d", rb.Namespace, rb.Name, totalReplicas)
	}

	usage := corev1.ResourceList{}
	replicaCount := int64(totalReplicas)

	for resourceName, quantityPerReplica := range rb.Spec.ReplicaRequirements.ResourceRequest {
		if quantityPerReplica.IsZero() {
			continue
		}
		if quantityPerReplica.Sign() < 0 {
			return nil, fmt.Errorf("resource request for %s in RB %s/%s has a negative value: %s", resourceName, rb.Namespace, rb.Name, quantityPerReplica.String())
		}

		totalQuantity := quantityPerReplica.DeepCopy()
		totalQuantity.Mul(replicaCount)

		usage[resourceName] = totalQuantity
	}
	klog.V(4).Infof("Calculated resource usage for ResourceBinding %s/%s: %v", rb.Namespace, rb.Name, usage)
	return usage, nil
}

func isQuotaRelevantFieldChanged(oldRB, newRB *workv1alpha2.ResourceBinding) bool {
	if oldRB == nil || newRB == nil {
		return true
	}
	oldReq := corev1.ResourceList{}
	if oldRB.Spec.ReplicaRequirements != nil {
		oldReq = oldRB.Spec.ReplicaRequirements.ResourceRequest
	}
	newReq := corev1.ResourceList{}
	if newRB.Spec.ReplicaRequirements != nil {
		newReq = newRB.Spec.ReplicaRequirements.ResourceRequest
	}
	if !areResourceListsEqual(oldReq, newReq) {
		return true
	}
	oldScheduledReplicas := int32(0)
	for _, c := range oldRB.Spec.Clusters {
		oldScheduledReplicas += c.Replicas
	}
	newScheduledReplicas := int32(0)
	for _, c := range newRB.Spec.Clusters {
		newScheduledReplicas += c.Replicas
	}
	return oldScheduledReplicas != newScheduledReplicas
}

func areResourceListsEqual(a, b corev1.ResourceList) bool {
	if len(a) != len(b) {
		return false
	}
	for key, valA := range a {
		valB, ok := b[key]
		if !ok {
			return false
		}
		if valA.Cmp(valB) != 0 {
			return false
		}
	}
	return true
}

// calculateDelta computes newUsage - oldUsage.
// The resulting delta list will only contain entries with non-zero quantities.
func calculateDelta(newUsage, oldUsage corev1.ResourceList) corev1.ResourceList {
	delta := corev1.ResourceList{}
	allResourceNames := make(map[corev1.ResourceName]struct{})
	for name := range newUsage {
		allResourceNames[name] = struct{}{}
	}
	for name := range oldUsage {
		allResourceNames[name] = struct{}{}
	}

	for name := range allResourceNames {
		newQ := newUsage[name] // .Quantity is a value type, zero if not present
		oldQ := oldUsage[name] // .Quantity is a value type, zero if not present

		deltaQ := newQ.DeepCopy()
		deltaQ.Sub(oldQ)

		if !deltaQ.IsZero() {
			delta[name] = deltaQ
		}
	}
	return delta
}

func addResourceLists(list1, list2 corev1.ResourceList) corev1.ResourceList {
	resUtil := util.NewResource(list1)
	resUtil.Add(list2)
	result := resUtil.ResourceList()
	if result == nil {
		return corev1.ResourceList{}
	}
	return result
}

func isAllowed(requested corev1.ResourceList, frqItem policyv1alpha1.FederatedResourceQuota) (bool, string) {
	allowedLimits := frqItem.Spec.Overall
	if allowedLimits == nil {
		return true, ""
	}
	for name, reqQty := range requested {
		if reqQty.IsZero() {
			continue
		}
		limitQty, ok := allowedLimits[name]
		if !ok {
			klog.V(5).Infof("Resource %s (requested sum: %s) is not defined in quota limits %v; considered allowed by this FRQ.", name, reqQty.String(), allowedLimits)
			continue
		}
		if reqQty.Cmp(limitQty) > 0 {
			msg := fmt.Sprintf("FederatedResourceQuota(%s/%s) exceeded for resource %s: requested sum %s, limit %s.", frqItem.Namespace, frqItem.Name, name, reqQty.String(), limitQty.String())
			klog.Warning(msg)
			return false, msg
		}
	}
	return true, ""
}

func filterResourceListByKeys(original corev1.ResourceList, filterKeySource corev1.ResourceList) corev1.ResourceList {
	if original == nil || filterKeySource == nil {
		return corev1.ResourceList{}
	}
	filtered := corev1.ResourceList{}
	for keyInFilter := range filterKeySource {
		if valInOriginal, ok := original[keyInFilter]; ok {
			filtered[keyInFilter] = valInOriginal.DeepCopy()
		}
	}
	return filtered
}

func isResourceListEffectivelyZero(listToCheck corev1.ResourceList, filterKeySource corev1.ResourceList) bool {
	if filterKeySource == nil {
		return true
	}
	for keyTrackedByFRQ := range filterKeySource {
		quantityInList, ok := listToCheck[keyTrackedByFRQ]
		if ok && !quantityInList.IsZero() {
			return false
		}
	}
	return true
}
