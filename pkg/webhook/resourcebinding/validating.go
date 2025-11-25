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
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
	"github.com/karmada-io/karmada/pkg/util"
	"github.com/karmada-io/karmada/pkg/util/helper"
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
	noFRQsFound        bool  // True if this attempt found no FRQs
	earlyExitError     error // Response if this attempt decided to exit early (denial, permanent error)
	ProcessError       error // Error for RetryOnConflict (e.g., conflict error or nil for success/final decision)
}

// Handle implements admission.Handler interface.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	rb, oldRB, err := v.decodeRBs(req)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Processing ResourceBinding(%s/%s) for request: %s (%s)", rb.Namespace, rb.Name, req.Operation, req.UID)

	if err := v.validateFederatedResourceQuota(ctx, req, rb, oldRB); err != nil {
		if apierrors.IsInternalError(err) {
			klog.Errorf("Internal error while processing ResourceBinding %s/%s: %v", rb.Namespace, rb.Name, err)
			return admission.Errored(http.StatusInternalServerError, err)
		}
		klog.Errorf("Admission denied for ResourceBinding %s/%s: %v", rb.Namespace, rb.Name, err)
		return admission.Denied(err.Error())
	}

	if features.FeatureGate.Enabled(features.MultiplePodTemplatesScheduling) {
		if err := v.validateComponents(rb.Spec.Components, field.NewPath("spec").Child("components")); err != nil {
			klog.Errorf("Admission denied for ResourceBinding %s/%s: %v", rb.Namespace, rb.Name, err)
			return admission.Denied(err.Error())
		}
	}

	return admission.Allowed("")
}

// validateFederatedResourceQuota checks the FederatedResourceQuota for the ResourceBinding.
// It returns a list of errors if validation fails, or nil if it passes.
func (v *ValidatingAdmission) validateFederatedResourceQuota(ctx context.Context, req admission.Request, rb, oldRB *workv1alpha2.ResourceBinding) error {
	if !features.FeatureGate.Enabled(features.FederatedQuotaEnforcement) {
		klog.V(5).Infof("FederatedQuotaEnforcement feature gate is disabled, skipping validation for ResourceBinding %s/%s", req.Namespace, req.Name)
		return nil
	}

	if req.Operation == admissionv1.Create && len(rb.Spec.Clusters) == 0 {
		klog.V(4).Infof("ResourceBinding %s/%s is being created but not yet scheduled, skipping quota validation.", rb.Namespace, rb.Name)
		return nil
	}

	if req.Operation == admissionv1.Update {
		if !isQuotaRelevantFieldChanged(oldRB, rb) {
			klog.V(4).Infof("ResourceBinding %s/%s updated, but no quota relevant fields changed, skipping quota validation.", rb.Namespace, rb.Name)
			return nil
		}
	}

	isDryRun := req.DryRun != nil && *req.DryRun

	newRbTotalUsage, oldRbTotalUsage, err := v.calculateRBUsages(rb, oldRB)
	if err != nil {
		return err
	}

	totalRbDelta := calculateDelta(newRbTotalUsage, oldRbTotalUsage)
	klog.V(4).Infof("Calculated total RB delta for %s/%s: %v", rb.Namespace, rb.Name, totalRbDelta)

	if len(totalRbDelta) == 0 {
		klog.V(2).Infof("No effective resource quantity delta for ResourceBinding %s/%s. Skipping quota validation.", rb.Namespace, rb.Name)
		return nil
	}

	outcome := v.processFRQsWithRetries(ctx, rb, totalRbDelta, isDryRun)
	return v.handleFRQOutcome(rb, outcome)
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

	frqList, listError := v.listFRQs(ctx, rb.Namespace)
	if listError != nil {
		outcome.earlyExitError = listError
		return outcome // attemptError is nil, signaling non-retryable handled error
	}

	if len(frqList.Items) == 0 {
		outcome.noFRQsFound = true
		// validationMessages and updatedFRQCount will remain empty/zero.
		return outcome // attemptError is nil, successful attempt (no FRQs to process)
	}

	for _, frqItem := range frqList.Items {
		newStatus, msg, denialError := v.processSingleFRQ(&frqItem, rb.Namespace, rb.Name, totalRbDelta)
		if denialError != nil {
			outcome.earlyExitError = denialError
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
		outcome.earlyExitError = apierrors.NewInternalError(errors.New(errMsg))
		return outcome // attemptError is nil, non-retryable handled error
	}

	outcome.validationMessages = currentValidationMessages
	outcome.updatedFRQCount = len(currentFrqsToUpdateStatus)
	return outcome // attemptError is nil, successful attempt
}

func (v *ValidatingAdmission) handleFRQOutcome(rb *workv1alpha2.ResourceBinding, outcome frqProcessOutcome) error {
	if outcome.ProcessError != nil {
		errMsg := fmt.Sprintf("failed to apply FederatedResourceQuota updates after multiple retries for RB %s/%s: %v", rb.Namespace, rb.Name, outcome.ProcessError)
		klog.Error(errMsg)
		return apierrors.NewInternalError(errors.New(errMsg))
	}

	if outcome.earlyExitError != nil {
		return outcome.earlyExitError
	}

	if outcome.noFRQsFound {
		klog.V(2).Infof("No FederatedResourceQuotas found in namespace %s for ResourceBinding %s. Allowing operation.", rb.Namespace, rb.Name)
		return nil
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
	return nil
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

// decodeRBs decodes current and old (for updates) ResourceBindings.
// Returns the RBs or an admission response for early exit on decoding failure.
func (v *ValidatingAdmission) decodeRBs(req admission.Request) (
	currentRB *workv1alpha2.ResourceBinding,
	oldRB *workv1alpha2.ResourceBinding, // Will be nil if not an update
	earlyExitResp error,
) {
	decodedRB := &workv1alpha2.ResourceBinding{}
	if err := v.Decoder.Decode(req, decodedRB); err != nil {
		klog.Errorf("Failed to decode ResourceBinding %s/%s: %v", req.Namespace, req.Name, err)
		return nil, nil, err
	}

	if req.Operation != admissionv1.Update {
		return decodedRB, nil, nil
	}

	decodedOldRB := &workv1alpha2.ResourceBinding{}
	if err := v.Decoder.DecodeRaw(req.OldObject, decodedOldRB); err != nil {
		klog.Errorf("Failed to decode old ResourceBinding %s/%s: %v", req.Namespace, req.Name, err)
		return nil, nil, err
	}

	return decodedRB, decodedOldRB, nil
}

func (v *ValidatingAdmission) calculateRBUsages(rb, oldRB *workv1alpha2.ResourceBinding) (corev1.ResourceList, corev1.ResourceList, error) {
	newRbTotalUsage := helper.CalculateResourceUsage(rb)
	klog.V(4).Infof("Calculated total usage for incoming RB %s/%s: %v", rb.Namespace, rb.Name, newRbTotalUsage)

	oldRbTotalUsage := corev1.ResourceList{}
	if oldRB != nil {
		oldRbTotalUsage = helper.CalculateResourceUsage(oldRB)
		klog.V(4).Infof("Calculated total usage for old RB %s/%s: %v", oldRB.Namespace, oldRB.Name, oldRbTotalUsage)
	}
	return newRbTotalUsage, oldRbTotalUsage, nil
}

func (v *ValidatingAdmission) listFRQs(ctx context.Context, namespace string) (*policyv1alpha1.FederatedResourceQuotaList, error) {
	frqList := &policyv1alpha1.FederatedResourceQuotaList{}
	if err := v.Client.List(ctx, frqList, client.InNamespace(namespace)); err != nil {
		klog.Errorf("Failed to list FederatedResourceQuotas in namespace %s: %v", namespace, err)
		return nil, apierrors.NewInternalError(fmt.Errorf("failed to list FederatedResourceQuotas: %w", err))
	}
	klog.V(2).Infof("Found %d FederatedResourceQuotas in namespace %s to evaluate.", len(frqList.Items), namespace)
	return frqList, nil
}

func (v *ValidatingAdmission) processSingleFRQ(frqItem *policyv1alpha1.FederatedResourceQuota, rbNamespace, rbName string, totalRbDelta corev1.ResourceList) (newStatusToSet corev1.ResourceList, validationMsg string, err error) {
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
		klog.Warningf("Quota exceeded for FederatedResourceQuota %s/%s. ResourceBinding %s/%s will be denied. %s",
			frqItem.Namespace, frqItem.Name, rbNamespace, rbName, errMsg)
		return nil, "", errors.New(errMsg)
	}

	msg := fmt.Sprintf("Quota check passed for FRQ %s/%s.", frqItem.Namespace, frqItem.Name)
	klog.V(3).Infof("FRQ %s/%s will be updated. New OverallUsed: %v", frqItem.Namespace, frqItem.Name, potentialNewOverallUsedForThisFRQ)
	return potentialNewOverallUsedForThisFRQ, msg, nil
}

// validateComponents checks the validity of the Components field in the ResourceBinding.
func (v *ValidatingAdmission) validateComponents(components []workv1alpha2.Component, fldPath *field.Path) error {
	var allErrs field.ErrorList
	componentNames := make(map[string]struct{})
	for index, component := range components {
		if len(component.Name) == 0 {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(index).Child("name"), component.Name, "component names must be non-empty"))
		} else if _, exists := componentNames[component.Name]; exists {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(index).Child("name"), component.Name, "component names must be unique"))
		} else {
			componentNames[component.Name] = struct{}{}
		}
	}
	return allErrs.ToAggregate()
}

func isQuotaRelevantFieldChanged(oldRB, newRB *workv1alpha2.ResourceBinding) bool {
	if oldRB == nil || newRB == nil {
		return true
	}
	if isResourceRequestChanged(oldRB, newRB) {
		return true
	}
	if isScheduledReplicasChanged(oldRB, newRB) {
		return true
	}
	if (len(oldRB.Spec.Components) != 0 || len(newRB.Spec.Components) != 0) && isScheduledClusterChanged(oldRB, newRB) {
		return true
	}
	return isComponentsChanged(oldRB, newRB)
}

func isResourceRequestChanged(oldRB, newRB *workv1alpha2.ResourceBinding) bool {
	oldReq := corev1.ResourceList{}
	if oldRB.Spec.ReplicaRequirements != nil {
		oldReq = oldRB.Spec.ReplicaRequirements.ResourceRequest
	}
	newReq := corev1.ResourceList{}
	if newRB.Spec.ReplicaRequirements != nil {
		newReq = newRB.Spec.ReplicaRequirements.ResourceRequest
	}
	return !areResourceListsEqual(oldReq, newReq)
}

func isScheduledReplicasChanged(oldRB, newRB *workv1alpha2.ResourceBinding) bool {
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

func isScheduledClusterChanged(oldRB, newRB *workv1alpha2.ResourceBinding) bool {
	if len(oldRB.Spec.Clusters) != len(newRB.Spec.Clusters) {
		return true
	}
	oldClusterMap := make(map[string]struct{})
	for _, c := range oldRB.Spec.Clusters {
		oldClusterMap[c.Name] = struct{}{}
	}
	for _, c := range newRB.Spec.Clusters {
		if _, exists := oldClusterMap[c.Name]; !exists {
			return true
		}
	}
	return false
}

func isComponentsChanged(oldRB, newRB *workv1alpha2.ResourceBinding) bool {
	if len(oldRB.Spec.Components) != len(newRB.Spec.Components) {
		return true
	}
	for i := range oldRB.Spec.Components {
		oldComponent := oldRB.Spec.Components[i]
		newComponent := newRB.Spec.Components[i]
		if oldComponent.Replicas != newComponent.Replicas {
			return true
		}

		if (oldComponent.ReplicaRequirements == nil) != (newComponent.ReplicaRequirements == nil) {
			return true
		}

		if oldComponent.ReplicaRequirements != nil && newComponent.ReplicaRequirements != nil && !areResourceListsEqual(oldComponent.ReplicaRequirements.ResourceRequest, newComponent.ReplicaRequirements.ResourceRequest) {
			return true
		}
	}
	return false
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

func isAllowed(requested corev1.ResourceList, frqItem *policyv1alpha1.FederatedResourceQuota) (bool, string) {
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
