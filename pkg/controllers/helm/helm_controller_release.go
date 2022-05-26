/*
Copyright 2020 The Flux authors

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

package helm

import (
	"context"
	"crypto/sha1"
	"errors"
	"fmt"
	"time"

	v2 "github.com/fluxcd/helm-controller/api/v2beta1"
	apiacl "github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/runtime/acl"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/release"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConditionError represents an error with a status condition reason attached.
type ConditionError struct {
	Reason string
	Err    error
}

func (c ConditionError) Error() string {
	return c.Err.Error()
}

func (h *HelmController) reconcileV2HelmRelease(ctx context.Context, hr v2.HelmRelease, clusterName string) (v2.HelmRelease, controllerruntime.Result, error) {
	start := time.Now()
	// Return early if the HelmRelease is suspended.
	if hr.Spec.Suspend {
		klog.V(6).Info("HelmRelease reconciliation is suspended for this object")
		return hr, controllerruntime.Result{}, nil
	}

	hr, result, err := h.reconcile(ctx, hr, clusterName)

	// Log reconciliation duration
	durationMsg := fmt.Sprintf("HelmRelease reconcilation finished in %s", time.Now().Sub(start).String())
	if result.RequeueAfter > 0 {
		durationMsg = fmt.Sprintf("%s, next run in %s", durationMsg, result.RequeueAfter.String())
	}
	klog.V(6).Info(durationMsg)

	return hr, result, err
}

func (h *HelmController) reconcile(ctx context.Context, hr v2.HelmRelease, clusterName string) (v2.HelmRelease, controllerruntime.Result, error) {
	// Record the value of the reconciliation request, if any
	if v, ok := meta.ReconcileAnnotationValue(hr.GetAnnotations()); ok {
		hr.Status.SetLastHandledReconcileRequest(v)
	}

	// Observe HelmRelease generation.
	if hr.Status.ObservedGeneration != hr.Generation {
		hr.Status.ObservedGeneration = hr.Generation
		hr = v2.HelmReleaseProgressing(hr)
	}

	// Reconcile chart based on the HelmChartTemplate
	hc, reconcileErr := h.reconcileChart(ctx, &hr)
	if reconcileErr != nil {
		if acl.IsAccessDenied(reconcileErr) {
			klog.Error(reconcileErr, "access denied to cross-namespace source")
			return v2.HelmReleaseNotReady(hr, apiacl.AccessDeniedReason, reconcileErr.Error()),
				controllerruntime.Result{RequeueAfter: hr.Spec.Interval.Duration}, nil
		}

		msg := fmt.Sprintf("chart reconciliation failed: %s", reconcileErr.Error())
		return v2.HelmReleaseNotReady(hr, v2.ArtifactFailedReason, msg), controllerruntime.Result{Requeue: true}, reconcileErr
	}

	// Check chart readiness
	if hc.Generation != hc.Status.ObservedGeneration || !apimeta.IsStatusConditionTrue(hc.Status.Conditions, meta.ReadyCondition) {
		msg := fmt.Sprintf("HelmChart '%s/%s' is not ready", hc.GetNamespace(), hc.GetName())
		klog.V(6).Info(msg)
		// Do not requeue immediately, when the artifact is created
		// the watcher should trigger a reconciliation.
		return v2.HelmReleaseNotReady(hr, v2.ArtifactFailedReason, msg), controllerruntime.Result{RequeueAfter: hc.Spec.Interval.Duration}, nil
	}

	// Check dependencies
	if len(hr.Spec.DependsOn) > 0 {
		if err := h.checkDependencies(hr); err != nil {
			msg := fmt.Sprintf("dependencies do not meet ready condition (%s), retrying in %s",
				err.Error(), h.RequeueDependency.String())
			klog.V(6).Info(msg)

			// Exponential backoff would cause execution to be prolonged too much,
			// instead we requeue on a fixed interval.
			return v2.HelmReleaseNotReady(hr,
				v2.DependencyNotReadyReason, err.Error()), controllerruntime.Result{RequeueAfter: h.RequeueDependency}, nil
		}
		klog.V(6).Info("all dependencies are ready, proceeding with release")
	}

	// Compose values
	values, err := h.composeValues(ctx, hr)
	if err != nil {
		return v2.HelmReleaseNotReady(hr, v2.InitFailedReason, err.Error()), controllerruntime.Result{Requeue: true}, nil
	}

	// Load chart from artifact
	chart, err := h.loadHelmChart(hc)
	if err != nil {
		return v2.HelmReleaseNotReady(hr, v2.ArtifactFailedReason, err.Error()), controllerruntime.Result{Requeue: true}, nil
	}

	// Reconcile Helm release
	reconciledHr, reconcileErr := h.reconcileRelease(*hr.DeepCopy(), chart, values, clusterName)
	return reconciledHr, controllerruntime.Result{RequeueAfter: hr.Spec.Interval.Duration}, reconcileErr
}

func (h *HelmController) checkDependencies(hr v2.HelmRelease) error {
	for _, d := range hr.Spec.DependsOn {
		if d.Namespace == "" {
			d.Namespace = hr.GetNamespace()
		}
		dName := types.NamespacedName{
			Namespace: d.Namespace,
			Name:      d.Name,
		}
		var dHr v2.HelmRelease
		err := h.Get(context.Background(), dName, &dHr)
		if err != nil {
			return fmt.Errorf("unable to get '%s' dependency: %w", dName, err)
		}

		if len(dHr.Status.Conditions) == 0 || dHr.Generation != dHr.Status.ObservedGeneration {
			return fmt.Errorf("dependency '%s' is not ready", dName)
		}

		if !apimeta.IsStatusConditionTrue(dHr.Status.Conditions, meta.ReadyCondition) {
			return fmt.Errorf("dependency '%s' is not ready", dName)
		}
	}
	return nil
}

func (h *HelmController) reconcileRelease(hr v2.HelmRelease, chart *chart.Chart, values chartutil.Values, clusterName string) (v2.HelmRelease, error) {
	var err error

	// Determine last release revision.
	rel, observeLastReleaseErr := h.HelmReleaseWatcher.ObserveLastRelease(clusterName, hr)
	if observeLastReleaseErr != nil {
		err = fmt.Errorf("failed to get last release revision: %w", observeLastReleaseErr)
		return v2.HelmReleaseNotReady(hr, v2.GetLastReleaseFailedReason, "failed to get last release revision"), err
	}

	// Register the current release attempt.
	revision := chart.Metadata.Version
	releaseRevision := ReleaseRevision(rel)
	valuesChecksum := ValuesChecksum(values)
	hr, hasNewState := v2.HelmReleaseAttempted(hr, revision, releaseRevision, valuesChecksum)
	if hasNewState {
		hr = v2.HelmReleaseProgressing(hr)
	}

	// Check status of any previous release attempt.
	released := apimeta.FindStatusCondition(hr.Status.Conditions, v2.ReleasedCondition)
	if released != nil {
		switch released.Status {
		// Succeed if the previous release attempt succeeded.
		case metav1.ConditionTrue:
			return v2.HelmReleaseReady(hr), nil
		case metav1.ConditionFalse:
			// Fail if the previous release attempt remediation failed.
			remediated := apimeta.FindStatusCondition(hr.Status.Conditions, v2.RemediatedCondition)
			if remediated != nil && remediated.Status == metav1.ConditionFalse {
				err = fmt.Errorf("previous release attempt remediation failed")
				return v2.HelmReleaseNotReady(hr, remediated.Reason, remediated.Message), err
			}
		}

		// Fail if install retries are exhausted.
		if hr.Spec.GetInstall().GetRemediation().RetriesExhausted(hr) {
			err = fmt.Errorf("install retries exhausted")
			return v2.HelmReleaseNotReady(hr, released.Reason, err.Error()), err
		}

		// Fail if there is a release and upgrade retries are exhausted.
		// This avoids failing after an upgrade uninstall remediation strategy.
		if rel != nil && hr.Spec.GetUpgrade().GetRemediation().RetriesExhausted(hr) {
			err = fmt.Errorf("upgrade retries exhausted")
			return v2.HelmReleaseNotReady(hr, released.Reason, err.Error()), err
		}
	}

	// Deploy the release.
	var deployAction v2.DeploymentAction
	if rel == nil {
		deployAction = hr.Spec.GetInstall()
		rel, err = h.HelmReleaseWatcher.InstallRelease(clusterName, hr, chart, values)
		err = h.handleHelmActionResult(&hr, err, deployAction.GetDescription(),
			v2.ReleasedCondition, v2.InstallSucceededReason, v2.InstallFailedReason)
	} else {
		deployAction = hr.Spec.GetUpgrade()
		rel, err = h.HelmReleaseWatcher.UpgradeRelease(clusterName, hr, chart, values)
		err = h.handleHelmActionResult(&hr, err, deployAction.GetDescription(),
			v2.ReleasedCondition, v2.UpgradeSucceededReason, v2.UpgradeFailedReason)
	}
	remediation := deployAction.GetRemediation()

	// If there is a new release revision...
	if ReleaseRevision(rel) > releaseRevision {
		// Ensure release is not marked remediated.
		apimeta.RemoveStatusCondition(&hr.Status.Conditions, v2.RemediatedCondition)

		// If new release revision is successful and tests are enabled, run them.
		if err == nil && hr.Spec.GetTest().Enable {
			_, testErr := h.HelmReleaseWatcher.TestRelease(clusterName, hr)
			testErr = h.handleHelmActionResult(&hr, testErr, "test",
				v2.TestSuccessCondition, v2.TestSucceededReason, v2.TestFailedReason)

			// Propagate any test error if not marked ignored.
			if testErr != nil && !remediation.MustIgnoreTestFailures(hr.Spec.GetTest().IgnoreFailures) {
				testsPassing := apimeta.FindStatusCondition(hr.Status.Conditions, v2.TestSuccessCondition)
				newCondition := metav1.Condition{
					Type:    v2.ReleasedCondition,
					Status:  metav1.ConditionFalse,
					Reason:  testsPassing.Reason,
					Message: testsPassing.Message,
				}
				apimeta.SetStatusCondition(hr.GetStatusConditions(), newCondition)
				err = testErr
			}
		}
	}

	if err != nil {
		// Increment failure count for deployment action.
		remediation.IncrementFailureCount(&hr)
		// Remediate deployment failure if necessary.
		if !remediation.RetriesExhausted(hr) || remediation.MustRemediateLastFailure() {
			if ReleaseRevision(rel) <= releaseRevision {
				klog.V(6).Info(fmt.Sprintf("skipping remediation, no new release revision created"))
			} else {
				var remediationErr error
				switch remediation.GetStrategy() {
				case v2.RollbackRemediationStrategy:
					rollbackErr := h.HelmReleaseWatcher.RollbackRelease(clusterName, hr)
					remediationErr = h.handleHelmActionResult(&hr, rollbackErr, "rollback",
						v2.RemediatedCondition, v2.RollbackSucceededReason, v2.RollbackFailedReason)
				case v2.UninstallRemediationStrategy:
					uninstallErr := h.HelmReleaseWatcher.UninstallRelease(clusterName, hr)
					remediationErr = h.handleHelmActionResult(&hr, uninstallErr, "uninstall",
						v2.RemediatedCondition, v2.UninstallSucceededReason, v2.UninstallFailedReason)
				}
				if remediationErr != nil {
					err = remediationErr
				}
			}

			// Determine release after remediation.
			rel, observeLastReleaseErr = h.HelmReleaseWatcher.ObserveLastRelease(clusterName, hr)
			if observeLastReleaseErr != nil {
				err = &ConditionError{
					Reason: v2.GetLastReleaseFailedReason,
					Err:    errors.New("failed to get last release revision after remediation"),
				}
			}
		}
	}

	hr.Status.LastReleaseRevision = ReleaseRevision(rel)

	if err != nil {
		reason := v2.ReconciliationFailedReason
		if condErr := (*ConditionError)(nil); errors.As(err, &condErr) {
			reason = condErr.Reason
		}
		return v2.HelmReleaseNotReady(hr, reason, err.Error()), err
	}
	return v2.HelmReleaseReady(hr), nil
}

func (h *HelmController) patchStatus(ctx context.Context, hr *v2.HelmRelease) error {
	key := client.ObjectKeyFromObject(hr)
	latest := &v2.HelmRelease{}
	if err := h.Client.Get(ctx, key, latest); err != nil {
		return err
	}
	return h.Client.Status().Patch(ctx, hr, client.MergeFrom(latest))
}

func (h *HelmController) handleHelmActionResult(
	hr *v2.HelmRelease, err error, action string, condition string, succeededReason string, failedReason string) error {
	if err != nil {
		err = fmt.Errorf("Helm %s failed: %w", action, err)
		msg := err.Error()
		newCondition := metav1.Condition{
			Type:    condition,
			Status:  metav1.ConditionFalse,
			Reason:  failedReason,
			Message: msg,
		}
		apimeta.SetStatusCondition(hr.GetStatusConditions(), newCondition)
		return &ConditionError{Reason: failedReason, Err: err}
	} else {
		msg := fmt.Sprintf("Helm %s succeeded", action)
		newCondition := metav1.Condition{
			Type:    condition,
			Status:  metav1.ConditionTrue,
			Reason:  succeededReason,
			Message: msg,
		}
		apimeta.SetStatusCondition(hr.GetStatusConditions(), newCondition)
		return nil
	}
}

// ValuesChecksum calculates and returns the SHA1 checksum for the
// given chartutil.Values.
func ValuesChecksum(values chartutil.Values) string {
	var s string
	if len(values) != 0 {
		s, _ = values.YAML()
	}
	return fmt.Sprintf("%x", sha1.Sum([]byte(s)))
}

// ReleaseRevision returns the revision of the given release.Release.
func ReleaseRevision(rel *release.Release) int {
	if rel == nil {
		return 0
	}
	return rel.Version
}
