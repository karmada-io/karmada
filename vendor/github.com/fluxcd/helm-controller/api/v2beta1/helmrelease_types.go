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

package v2beta1

import (
	"encoding/json"
	"strings"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
)

const HelmReleaseKind = "HelmRelease"
const HelmReleaseFinalizer = "finalizers.fluxcd.io"

// Kustomize Helm PostRenderer specification.
type Kustomize struct {
	// Strategic merge and JSON patches, defined as inline YAML objects,
	// capable of targeting objects based on kind, label and annotation selectors.
	// +optional
	Patches []kustomize.Patch `json:"patches,omitempty"`

	// Strategic merge patches, defined as inline YAML objects.
	// +optional
	PatchesStrategicMerge []apiextensionsv1.JSON `json:"patchesStrategicMerge,omitempty"`

	// JSON 6902 patches, defined as inline YAML objects.
	// +optional
	PatchesJSON6902 []kustomize.JSON6902Patch `json:"patchesJson6902,omitempty"`

	// Images is a list of (image name, new name, new tag or digest)
	// for changing image names, tags or digests. This can also be achieved with a
	// patch, but this operator is simpler to specify.
	// +optional
	Images []kustomize.Image `json:"images,omitempty" yaml:"images,omitempty"`
}

// PostRenderer contains a Helm PostRenderer specification.
type PostRenderer struct {
	// Kustomization to apply as PostRenderer.
	// +optional
	Kustomize *Kustomize `json:"kustomize,omitempty"`
}

// HelmReleaseSpec defines the desired state of a Helm release.
type HelmReleaseSpec struct {
	// Chart defines the template of the v1beta2.HelmChart that should be created
	// for this HelmRelease.
	// +required
	Chart HelmChartTemplate `json:"chart"`

	// Interval at which to reconcile the Helm release.
	// +required
	Interval metav1.Duration `json:"interval"`

	// KubeConfig for reconciling the HelmRelease on a remote cluster.
	// When used in combination with HelmReleaseSpec.ServiceAccountName,
	// forces the controller to act on behalf of that Service Account at the
	// target cluster.
	// If the --default-service-account flag is set, its value will be used as
	// a controller level fallback for when HelmReleaseSpec.ServiceAccountName
	// is empty.
	// +optional
	KubeConfig *KubeConfig `json:"kubeConfig,omitempty"`

	// Suspend tells the controller to suspend reconciliation for this HelmRelease,
	// it does not apply to already started reconciliations. Defaults to false.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// ReleaseName used for the Helm release. Defaults to a composition of
	// '[TargetNamespace-]Name'.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=53
	// +kubebuilder:validation:Optional
	// +optional
	ReleaseName string `json:"releaseName,omitempty"`

	// TargetNamespace to target when performing operations for the HelmRelease.
	// Defaults to the namespace of the HelmRelease.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Optional
	// +optional
	TargetNamespace string `json:"targetNamespace,omitempty"`

	// StorageNamespace used for the Helm storage.
	// Defaults to the namespace of the HelmRelease.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Optional
	// +optional
	StorageNamespace string `json:"storageNamespace,omitempty"`

	// DependsOn may contain a meta.NamespacedObjectReference slice with
	// references to HelmRelease resources that must be ready before this HelmRelease
	// can be reconciled.
	// +optional
	DependsOn []meta.NamespacedObjectReference `json:"dependsOn,omitempty"`

	// Timeout is the time to wait for any individual Kubernetes operation (like Jobs
	// for hooks) during the performance of a Helm action. Defaults to '5m0s'.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// MaxHistory is the number of revisions saved by Helm for this HelmRelease.
	// Use '0' for an unlimited number of revisions; defaults to '10'.
	// +optional
	MaxHistory *int `json:"maxHistory,omitempty"`

	// The name of the Kubernetes service account to impersonate
	// when reconciling this HelmRelease.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Install holds the configuration for Helm install actions for this HelmRelease.
	// +optional
	Install *Install `json:"install,omitempty"`

	// Upgrade holds the configuration for Helm upgrade actions for this HelmRelease.
	// +optional
	Upgrade *Upgrade `json:"upgrade,omitempty"`

	// Test holds the configuration for Helm test actions for this HelmRelease.
	// +optional
	Test *Test `json:"test,omitempty"`

	// Rollback holds the configuration for Helm rollback actions for this HelmRelease.
	// +optional
	Rollback *Rollback `json:"rollback,omitempty"`

	// Uninstall holds the configuration for Helm uninstall actions for this HelmRelease.
	// +optional
	Uninstall *Uninstall `json:"uninstall,omitempty"`

	// ValuesFrom holds references to resources containing Helm values for this HelmRelease,
	// and information about how they should be merged.
	ValuesFrom []ValuesReference `json:"valuesFrom,omitempty"`

	// Values holds the values for this Helm release.
	// +optional
	Values *apiextensionsv1.JSON `json:"values,omitempty"`

	// PostRenderers holds an array of Helm PostRenderers, which will be applied in order
	// of their definition.
	// +optional
	PostRenderers []PostRenderer `json:"postRenderers,omitempty"`
}

// GetInstall returns the configuration for Helm install actions for the
// HelmRelease.
func (in HelmReleaseSpec) GetInstall() Install {
	if in.Install == nil {
		return Install{}
	}
	return *in.Install
}

// GetUpgrade returns the configuration for Helm upgrade actions for this
// HelmRelease.
func (in HelmReleaseSpec) GetUpgrade() Upgrade {
	if in.Upgrade == nil {
		return Upgrade{}
	}
	return *in.Upgrade
}

// GetTest returns the configuration for Helm test actions for this HelmRelease.
func (in HelmReleaseSpec) GetTest() Test {
	if in.Test == nil {
		return Test{}
	}
	return *in.Test
}

// GetRollback returns the configuration for Helm rollback actions for this
// HelmRelease.
func (in HelmReleaseSpec) GetRollback() Rollback {
	if in.Rollback == nil {
		return Rollback{}
	}
	return *in.Rollback
}

// GetUninstall returns the configuration for Helm uninstall actions for this
// HelmRelease.
func (in HelmReleaseSpec) GetUninstall() Uninstall {
	if in.Uninstall == nil {
		return Uninstall{}
	}
	return *in.Uninstall
}

// KubeConfig references a Kubernetes secret that contains a kubeconfig file.
type KubeConfig struct {
	// SecretRef holds the name to a secret that contains a key with
	// the kubeconfig file as the value. If no key is specified the key will
	// default to 'value'. The secret must be in the same namespace as
	// the HelmRelease.
	// It is recommended that the kubeconfig is self-contained, and the secret
	// is regularly updated if credentials such as a cloud-access-token expire.
	// Cloud specific `cmd-path` auth helpers will not function without adding
	// binaries and credentials to the Pod that is responsible for reconciling
	// the HelmRelease.
	// +required
	SecretRef meta.SecretKeyReference `json:"secretRef,omitempty"`
}

// HelmChartTemplate defines the template from which the controller will
// generate a v1beta2.HelmChart object in the same namespace as the referenced
// v1beta2.Source.
type HelmChartTemplate struct {
	// Spec holds the template for the v1beta2.HelmChartSpec for this HelmRelease.
	// +required
	Spec HelmChartTemplateSpec `json:"spec"`
}

// HelmChartTemplateSpec defines the template from which the controller will
// generate a v1beta2.HelmChartSpec object.
type HelmChartTemplateSpec struct {
	// The name or path the Helm chart is available at in the SourceRef.
	// +required
	Chart string `json:"chart"`

	// Version semver expression, ignored for charts from v1beta2.GitRepository and
	// v1beta2.Bucket sources. Defaults to latest when omitted.
	// +kubebuilder:default:=*
	// +optional
	Version string `json:"version,omitempty"`

	// The name and namespace of the v1beta2.Source the chart is available at.
	// +required
	SourceRef CrossNamespaceObjectReference `json:"sourceRef"`

	// Interval at which to check the v1beta2.Source for updates. Defaults to
	// 'HelmReleaseSpec.Interval'.
	// +optional
	Interval *metav1.Duration `json:"interval,omitempty"`

	// Determines what enables the creation of a new artifact. Valid values are
	// ('ChartVersion', 'Revision').
	// See the documentation of the values for an explanation on their behavior.
	// Defaults to ChartVersion when omitted.
	// +kubebuilder:validation:Enum=ChartVersion;Revision
	// +kubebuilder:default:=ChartVersion
	// +optional
	ReconcileStrategy string `json:"reconcileStrategy,omitempty"`

	// Alternative list of values files to use as the chart values (values.yaml
	// is not included by default), expected to be a relative path in the SourceRef.
	// Values files are merged in the order of this list with the last file overriding
	// the first. Ignored when omitted.
	// +optional
	ValuesFiles []string `json:"valuesFiles,omitempty"`

	// Alternative values file to use as the default chart values, expected to
	// be a relative path in the SourceRef. Deprecated in favor of ValuesFiles,
	// for backwards compatibility the file defined here is merged before the
	// ValuesFiles items. Ignored when omitted.
	// +optional
	// +deprecated
	ValuesFile string `json:"valuesFile,omitempty"`
}

// GetInterval returns the configured interval for the v1beta2.HelmChart,
// or the given default.
func (in HelmChartTemplate) GetInterval(defaultInterval metav1.Duration) metav1.Duration {
	if in.Spec.Interval == nil {
		return defaultInterval
	}
	return *in.Spec.Interval
}

// GetNamespace returns the namespace targeted namespace for the
// v1beta2.HelmChart, or the given default.
func (in HelmChartTemplate) GetNamespace(defaultNamespace string) string {
	if in.Spec.SourceRef.Namespace == "" {
		return defaultNamespace
	}
	return in.Spec.SourceRef.Namespace
}

// DeploymentAction defines a consistent interface for Install and Upgrade.
// +kubebuilder:object:generate=false
type DeploymentAction interface {
	GetDescription() string
	GetRemediation() Remediation
}

// Remediation defines a consistent interface for InstallRemediation and
// UpgradeRemediation.
// +kubebuilder:object:generate=false
type Remediation interface {
	GetRetries() int
	MustIgnoreTestFailures(bool) bool
	MustRemediateLastFailure() bool
	GetStrategy() RemediationStrategy
	GetFailureCount(hr HelmRelease) int64
	IncrementFailureCount(hr *HelmRelease)
	RetriesExhausted(hr HelmRelease) bool
}

// Install holds the configuration for Helm install actions performed for this
// HelmRelease.
type Install struct {
	// Timeout is the time to wait for any individual Kubernetes operation (like
	// Jobs for hooks) during the performance of a Helm install action. Defaults to
	// 'HelmReleaseSpec.Timeout'.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Remediation holds the remediation configuration for when the Helm install
	// action for the HelmRelease fails. The default is to not perform any action.
	// +optional
	Remediation *InstallRemediation `json:"remediation,omitempty"`

	// DisableWait disables the waiting for resources to be ready after a Helm
	// install has been performed.
	// +optional
	DisableWait bool `json:"disableWait,omitempty"`

	// DisableWaitForJobs disables waiting for jobs to complete after a Helm
	// install has been performed.
	// +optional
	DisableWaitForJobs bool `json:"disableWaitForJobs,omitempty"`

	// DisableHooks prevents hooks from running during the Helm install action.
	// +optional
	DisableHooks bool `json:"disableHooks,omitempty"`

	// DisableOpenAPIValidation prevents the Helm install action from validating
	// rendered templates against the Kubernetes OpenAPI Schema.
	// +optional
	DisableOpenAPIValidation bool `json:"disableOpenAPIValidation,omitempty"`

	// Replace tells the Helm install action to re-use the 'ReleaseName', but only
	// if that name is a deleted release which remains in the history.
	// +optional
	Replace bool `json:"replace,omitempty"`

	// SkipCRDs tells the Helm install action to not install any CRDs. By default,
	// CRDs are installed if not already present.
	//
	// Deprecated use CRD policy (`crds`) attribute with value `Skip` instead.
	//
	// +deprecated
	// +optional
	SkipCRDs bool `json:"skipCRDs,omitempty"`

	// CRDs upgrade CRDs from the Helm Chart's crds directory according
	// to the CRD upgrade policy provided here. Valid values are `Skip`,
	// `Create` or `CreateReplace`. Default is `Create` and if omitted
	// CRDs are installed but not updated.
	//
	// Skip: do neither install nor replace (update) any CRDs.
	//
	// Create: new CRDs are created, existing CRDs are neither updated nor deleted.
	//
	// CreateReplace: new CRDs are created, existing CRDs are updated (replaced)
	// but not deleted.
	//
	// By default, CRDs are applied (installed) during Helm install action.
	// With this option users can opt-in to CRD replace existing CRDs on Helm
	// install actions, which is not (yet) natively supported by Helm.
	// https://helm.sh/docs/chart_best_practices/custom_resource_definitions.
	//
	// +kubebuilder:validation:Enum=Skip;Create;CreateReplace
	// +optional
	CRDs CRDsPolicy `json:"crds,omitempty"`

	// CreateNamespace tells the Helm install action to create the
	// HelmReleaseSpec.TargetNamespace if it does not exist yet.
	// On uninstall, the namespace will not be garbage collected.
	// +optional
	CreateNamespace bool `json:"createNamespace,omitempty"`
}

// GetTimeout returns the configured timeout for the Helm install action,
// or the given default.
func (in Install) GetTimeout(defaultTimeout metav1.Duration) metav1.Duration {
	if in.Timeout == nil {
		return defaultTimeout
	}
	return *in.Timeout
}

// GetDescription returns a description for the Helm install action.
func (in Install) GetDescription() string {
	return "install"
}

// GetRemediation returns the configured Remediation for the Helm install action.
func (in Install) GetRemediation() Remediation {
	if in.Remediation == nil {
		return InstallRemediation{}
	}
	return *in.Remediation
}

// InstallRemediation holds the configuration for Helm install remediation.
type InstallRemediation struct {
	// Retries is the number of retries that should be attempted on failures before
	// bailing. Remediation, using an uninstall, is performed between each attempt.
	// Defaults to '0', a negative integer equals to unlimited retries.
	// +optional
	Retries int `json:"retries,omitempty"`

	// IgnoreTestFailures tells the controller to skip remediation when the Helm
	// tests are run after an install action but fail. Defaults to
	// 'Test.IgnoreFailures'.
	// +optional
	IgnoreTestFailures *bool `json:"ignoreTestFailures,omitempty"`

	// RemediateLastFailure tells the controller to remediate the last failure, when
	// no retries remain. Defaults to 'false'.
	// +optional
	RemediateLastFailure *bool `json:"remediateLastFailure,omitempty"`
}

// GetRetries returns the number of retries that should be attempted on
// failures.
func (in InstallRemediation) GetRetries() int {
	return in.Retries
}

// MustIgnoreTestFailures returns the configured IgnoreTestFailures or the given
// default.
func (in InstallRemediation) MustIgnoreTestFailures(def bool) bool {
	if in.IgnoreTestFailures == nil {
		return def
	}
	return *in.IgnoreTestFailures
}

// MustRemediateLastFailure returns whether to remediate the last failure when
// no retries remain.
func (in InstallRemediation) MustRemediateLastFailure() bool {
	if in.RemediateLastFailure == nil {
		return false
	}
	return *in.RemediateLastFailure
}

// GetStrategy returns the strategy to use for failure remediation.
func (in InstallRemediation) GetStrategy() RemediationStrategy {
	return UninstallRemediationStrategy
}

// GetFailureCount gets the failure count.
func (in InstallRemediation) GetFailureCount(hr HelmRelease) int64 {
	return hr.Status.InstallFailures
}

// IncrementFailureCount increments the failure count.
func (in InstallRemediation) IncrementFailureCount(hr *HelmRelease) {
	hr.Status.InstallFailures++
}

// RetriesExhausted returns true if there are no remaining retries.
func (in InstallRemediation) RetriesExhausted(hr HelmRelease) bool {
	return in.Retries >= 0 && in.GetFailureCount(hr) > int64(in.Retries)
}

// CRDsPolicy defines the install/upgrade approach to use for CRDs when
// installing or upgrading a HelmRelease.
type CRDsPolicy string

const (
	// Skip CRDs do neither install nor replace (update) any CRDs.
	Skip CRDsPolicy = "Skip"
	// Create CRDs which do not already exist, do not replace (update) already existing
	// CRDs and keep (do not delete) CRDs which no longer exist in the current release.
	Create CRDsPolicy = "Create"
	// Create CRDs which do not already exist, Replace (update) already existing CRDs
	// and keep (do not delete) CRDs which no longer exist in the current release.
	CreateReplace CRDsPolicy = "CreateReplace"
)

// Upgrade holds the configuration for Helm upgrade actions for this
// HelmRelease.
type Upgrade struct {
	// Timeout is the time to wait for any individual Kubernetes operation (like
	// Jobs for hooks) during the performance of a Helm upgrade action. Defaults to
	// 'HelmReleaseSpec.Timeout'.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Remediation holds the remediation configuration for when the Helm upgrade
	// action for the HelmRelease fails. The default is to not perform any action.
	// +optional
	Remediation *UpgradeRemediation `json:"remediation,omitempty"`

	// DisableWait disables the waiting for resources to be ready after a Helm
	// upgrade has been performed.
	// +optional
	DisableWait bool `json:"disableWait,omitempty"`

	// DisableWaitForJobs disables waiting for jobs to complete after a Helm
	// upgrade has been performed.
	// +optional
	DisableWaitForJobs bool `json:"disableWaitForJobs,omitempty"`

	// DisableHooks prevents hooks from running during the Helm upgrade action.
	// +optional
	DisableHooks bool `json:"disableHooks,omitempty"`

	// DisableOpenAPIValidation prevents the Helm upgrade action from validating
	// rendered templates against the Kubernetes OpenAPI Schema.
	// +optional
	DisableOpenAPIValidation bool `json:"disableOpenAPIValidation,omitempty"`

	// Force forces resource updates through a replacement strategy.
	// +optional
	Force bool `json:"force,omitempty"`

	// PreserveValues will make Helm reuse the last release's values and merge in
	// overrides from 'Values'. Setting this flag makes the HelmRelease
	// non-declarative.
	// +optional
	PreserveValues bool `json:"preserveValues,omitempty"`

	// CleanupOnFail allows deletion of new resources created during the Helm
	// upgrade action when it fails.
	// +optional
	CleanupOnFail bool `json:"cleanupOnFail,omitempty"`

	// CRDs upgrade CRDs from the Helm Chart's crds directory according
	// to the CRD upgrade policy provided here. Valid values are `Skip`,
	// `Create` or `CreateReplace`. Default is `Skip` and if omitted
	// CRDs are neither installed nor upgraded.
	//
	// Skip: do neither install nor replace (update) any CRDs.
	//
	// Create: new CRDs are created, existing CRDs are neither updated nor deleted.
	//
	// CreateReplace: new CRDs are created, existing CRDs are updated (replaced)
	// but not deleted.
	//
	// By default, CRDs are not applied during Helm upgrade action. With this
	// option users can opt-in to CRD upgrade, which is not (yet) natively supported by Helm.
	// https://helm.sh/docs/chart_best_practices/custom_resource_definitions.
	//
	// +kubebuilder:validation:Enum=Skip;Create;CreateReplace
	// +optional
	CRDs CRDsPolicy `json:"crds,omitempty"`
}

// GetTimeout returns the configured timeout for the Helm upgrade action, or the
// given default.
func (in Upgrade) GetTimeout(defaultTimeout metav1.Duration) metav1.Duration {
	if in.Timeout == nil {
		return defaultTimeout
	}
	return *in.Timeout
}

// GetDescription returns a description for the Helm upgrade action.
func (in Upgrade) GetDescription() string {
	return "upgrade"
}

// GetRemediation returns the configured Remediation for the Helm upgrade
// action.
func (in Upgrade) GetRemediation() Remediation {
	if in.Remediation == nil {
		return UpgradeRemediation{}
	}
	return *in.Remediation
}

// UpgradeRemediation holds the configuration for Helm upgrade remediation.
type UpgradeRemediation struct {
	// Retries is the number of retries that should be attempted on failures before
	// bailing. Remediation, using 'Strategy', is performed between each attempt.
	// Defaults to '0', a negative integer equals to unlimited retries.
	// +optional
	Retries int `json:"retries,omitempty"`

	// IgnoreTestFailures tells the controller to skip remediation when the Helm
	// tests are run after an upgrade action but fail.
	// Defaults to 'Test.IgnoreFailures'.
	// +optional
	IgnoreTestFailures *bool `json:"ignoreTestFailures,omitempty"`

	// RemediateLastFailure tells the controller to remediate the last failure, when
	// no retries remain. Defaults to 'false' unless 'Retries' is greater than 0.
	// +optional
	RemediateLastFailure *bool `json:"remediateLastFailure,omitempty"`

	// Strategy to use for failure remediation. Defaults to 'rollback'.
	// +kubebuilder:validation:Enum=rollback;uninstall
	// +optional
	Strategy *RemediationStrategy `json:"strategy,omitempty"`
}

// GetRetries returns the number of retries that should be attempted on
// failures.
func (in UpgradeRemediation) GetRetries() int {
	return in.Retries
}

// MustIgnoreTestFailures returns the configured IgnoreTestFailures or the given
// default.
func (in UpgradeRemediation) MustIgnoreTestFailures(def bool) bool {
	if in.IgnoreTestFailures == nil {
		return def
	}
	return *in.IgnoreTestFailures
}

// MustRemediateLastFailure returns whether to remediate the last failure when
// no retries remain.
func (in UpgradeRemediation) MustRemediateLastFailure() bool {
	if in.RemediateLastFailure == nil {
		return in.Retries > 0
	}
	return *in.RemediateLastFailure
}

// GetStrategy returns the strategy to use for failure remediation.
func (in UpgradeRemediation) GetStrategy() RemediationStrategy {
	if in.Strategy == nil {
		return RollbackRemediationStrategy
	}
	return *in.Strategy
}

// GetFailureCount gets the failure count.
func (in UpgradeRemediation) GetFailureCount(hr HelmRelease) int64 {
	return hr.Status.UpgradeFailures
}

// IncrementFailureCount increments the failure count.
func (in UpgradeRemediation) IncrementFailureCount(hr *HelmRelease) {
	hr.Status.UpgradeFailures++
}

// RetriesExhausted returns true if there are no remaining retries.
func (in UpgradeRemediation) RetriesExhausted(hr HelmRelease) bool {
	return in.Retries >= 0 && in.GetFailureCount(hr) > int64(in.Retries)
}

// RemediationStrategy returns the strategy to use to remediate a failed install
// or upgrade.
type RemediationStrategy string

const (
	// RollbackRemediationStrategy represents a Helm remediation strategy of Helm
	// rollback.
	RollbackRemediationStrategy RemediationStrategy = "rollback"

	// UninstallRemediationStrategy represents a Helm remediation strategy of Helm
	// uninstall.
	UninstallRemediationStrategy RemediationStrategy = "uninstall"
)

// Test holds the configuration for Helm test actions for this HelmRelease.
type Test struct {
	// Enable enables Helm test actions for this HelmRelease after an Helm install
	// or upgrade action has been performed.
	// +optional
	Enable bool `json:"enable,omitempty"`

	// Timeout is the time to wait for any individual Kubernetes operation during
	// the performance of a Helm test action. Defaults to 'HelmReleaseSpec.Timeout'.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// IgnoreFailures tells the controller to skip remediation when the Helm tests
	// are run but fail. Can be overwritten for tests run after install or upgrade
	// actions in 'Install.IgnoreTestFailures' and 'Upgrade.IgnoreTestFailures'.
	// +optional
	IgnoreFailures bool `json:"ignoreFailures,omitempty"`
}

// GetTimeout returns the configured timeout for the Helm test action,
// or the given default.
func (in Test) GetTimeout(defaultTimeout metav1.Duration) metav1.Duration {
	if in.Timeout == nil {
		return defaultTimeout
	}
	return *in.Timeout
}

// Rollback holds the configuration for Helm rollback actions for this
// HelmRelease.
type Rollback struct {
	// Timeout is the time to wait for any individual Kubernetes operation (like
	// Jobs for hooks) during the performance of a Helm rollback action. Defaults to
	// 'HelmReleaseSpec.Timeout'.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// DisableWait disables the waiting for resources to be ready after a Helm
	// rollback has been performed.
	// +optional
	DisableWait bool `json:"disableWait,omitempty"`

	// DisableWaitForJobs disables waiting for jobs to complete after a Helm
	// rollback has been performed.
	// +optional
	DisableWaitForJobs bool `json:"disableWaitForJobs,omitempty"`

	// DisableHooks prevents hooks from running during the Helm rollback action.
	// +optional
	DisableHooks bool `json:"disableHooks,omitempty"`

	// Recreate performs pod restarts for the resource if applicable.
	// +optional
	Recreate bool `json:"recreate,omitempty"`

	// Force forces resource updates through a replacement strategy.
	// +optional
	Force bool `json:"force,omitempty"`

	// CleanupOnFail allows deletion of new resources created during the Helm
	// rollback action when it fails.
	// +optional
	CleanupOnFail bool `json:"cleanupOnFail,omitempty"`
}

// GetTimeout returns the configured timeout for the Helm rollback action, or
// the given default.
func (in Rollback) GetTimeout(defaultTimeout metav1.Duration) metav1.Duration {
	if in.Timeout == nil {
		return defaultTimeout
	}
	return *in.Timeout
}

// Uninstall holds the configuration for Helm uninstall actions for this
// HelmRelease.
type Uninstall struct {
	// Timeout is the time to wait for any individual Kubernetes operation (like
	// Jobs for hooks) during the performance of a Helm uninstall action. Defaults
	// to 'HelmReleaseSpec.Timeout'.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// DisableHooks prevents hooks from running during the Helm rollback action.
	// +optional
	DisableHooks bool `json:"disableHooks,omitempty"`

	// KeepHistory tells Helm to remove all associated resources and mark the
	// release as deleted, but retain the release history.
	// +optional
	KeepHistory bool `json:"keepHistory,omitempty"`

	// DisableWait disables waiting for all the resources to be deleted after
	// a Helm uninstall is performed.
	// +optional
	DisableWait bool `json:"disableWait,omitempty"`
}

// GetTimeout returns the configured timeout for the Helm uninstall action, or
// the given default.
func (in Uninstall) GetTimeout(defaultTimeout metav1.Duration) metav1.Duration {
	if in.Timeout == nil {
		return defaultTimeout
	}
	return *in.Timeout
}

// HelmReleaseStatus defines the observed state of a HelmRelease.
type HelmReleaseStatus struct {
	// ObservedGeneration is the last observed generation.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	meta.ReconcileRequestStatus `json:",inline"`

	// Conditions holds the conditions for the HelmRelease.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastAppliedRevision is the revision of the last successfully applied source.
	// +optional
	LastAppliedRevision string `json:"lastAppliedRevision,omitempty"`

	// LastAttemptedRevision is the revision of the last reconciliation attempt.
	// +optional
	LastAttemptedRevision string `json:"lastAttemptedRevision,omitempty"`

	// LastAttemptedValuesChecksum is the SHA1 checksum of the values of the last
	// reconciliation attempt.
	// +optional
	LastAttemptedValuesChecksum string `json:"lastAttemptedValuesChecksum,omitempty"`

	// LastReleaseRevision is the revision of the last successful Helm release.
	// +optional
	LastReleaseRevision int `json:"lastReleaseRevision,omitempty"`

	// HelmChart is the namespaced name of the HelmChart resource created by
	// the controller for the HelmRelease.
	// +optional
	HelmChart string `json:"helmChart,omitempty"`

	// Failures is the reconciliation failure count against the latest desired
	// state. It is reset after a successful reconciliation.
	// +optional
	Failures int64 `json:"failures,omitempty"`

	// InstallFailures is the install failure count against the latest desired
	// state. It is reset after a successful reconciliation.
	// +optional
	InstallFailures int64 `json:"installFailures,omitempty"`

	// UpgradeFailures is the upgrade failure count against the latest desired
	// state. It is reset after a successful reconciliation.
	// +optional
	UpgradeFailures int64 `json:"upgradeFailures,omitempty"`
}

// GetHelmChart returns the namespace and name of the HelmChart.
func (in HelmReleaseStatus) GetHelmChart() (string, string) {
	if in.HelmChart == "" {
		return "", ""
	}
	if split := strings.Split(in.HelmChart, string(types.Separator)); len(split) > 1 {
		return split[0], split[1]
	}
	return "", ""
}

// HelmReleaseProgressing resets any failures and registers progress toward
// reconciling the given HelmRelease by setting the meta.ReadyCondition to
// 'Unknown' for meta.ProgressingReason.
func HelmReleaseProgressing(hr HelmRelease) HelmRelease {
	hr.Status.Conditions = []metav1.Condition{}
	newCondition := metav1.Condition{
		Type:    meta.ReadyCondition,
		Status:  metav1.ConditionUnknown,
		Reason:  meta.ProgressingReason,
		Message: "Reconciliation in progress",
	}
	apimeta.SetStatusCondition(hr.GetStatusConditions(), newCondition)
	resetFailureCounts(&hr)
	return hr
}

// HelmReleaseNotReady registers a failed reconciliation of the given HelmRelease.
func HelmReleaseNotReady(hr HelmRelease, reason, message string) HelmRelease {
	newCondition := metav1.Condition{
		Type:    meta.ReadyCondition,
		Status:  metav1.ConditionFalse,
		Reason:  reason,
		Message: message,
	}
	apimeta.SetStatusCondition(hr.GetStatusConditions(), newCondition)
	hr.Status.Failures++
	return hr
}

// HelmReleaseReady registers a successful reconciliation of the given HelmRelease.
func HelmReleaseReady(hr HelmRelease) HelmRelease {
	newCondition := metav1.Condition{
		Type:    meta.ReadyCondition,
		Status:  metav1.ConditionTrue,
		Reason:  ReconciliationSucceededReason,
		Message: "Release reconciliation succeeded",
	}
	apimeta.SetStatusCondition(hr.GetStatusConditions(), newCondition)
	hr.Status.LastAppliedRevision = hr.Status.LastAttemptedRevision
	resetFailureCounts(&hr)
	return hr
}

// HelmReleaseAttempted registers an attempt of the given HelmRelease with the given state.
// and returns the modified HelmRelease and a boolean indicating a state change.
func HelmReleaseAttempted(hr HelmRelease, revision string, releaseRevision int, valuesChecksum string) (HelmRelease, bool) {
	changed := hr.Status.LastAttemptedRevision != revision ||
		hr.Status.LastReleaseRevision != releaseRevision ||
		hr.Status.LastAttemptedValuesChecksum != valuesChecksum
	hr.Status.LastAttemptedRevision = revision
	hr.Status.LastReleaseRevision = releaseRevision
	hr.Status.LastAttemptedValuesChecksum = valuesChecksum

	return hr, changed
}

func resetFailureCounts(hr *HelmRelease) {
	hr.Status.Failures = 0
	hr.Status.InstallFailures = 0
	hr.Status.UpgradeFailures = 0
}

const (
	// SourceIndexKey is the key used for indexing HelmReleases based on
	// their sources.
	SourceIndexKey string = ".metadata.source"
)

// +genclient
// +genclient:Namespaced
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=hr
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// HelmRelease is the Schema for the helmreleases API
type HelmRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HelmReleaseSpec `json:"spec,omitempty"`
	// +kubebuilder:default:={"observedGeneration":-1}
	Status HelmReleaseStatus `json:"status,omitempty"`
}

// GetRequeueAfter returns the duration after which the HelmRelease
// must be reconciled again.
func (in HelmRelease) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// GetValues unmarshals the raw values to a map[string]interface{} and returns
// the result.
func (in HelmRelease) GetValues() map[string]interface{} {
	var values map[string]interface{}
	if in.Spec.Values != nil {
		_ = json.Unmarshal(in.Spec.Values.Raw, &values)
	}
	return values
}

// GetReleaseName returns the configured release name, or a composition of
// '[TargetNamespace-]Name'.
func (in HelmRelease) GetReleaseName() string {
	if in.Spec.ReleaseName != "" {
		return in.Spec.ReleaseName
	}
	if in.Spec.TargetNamespace != "" {
		return strings.Join([]string{in.Spec.TargetNamespace, in.Name}, "-")
	}
	return in.Name
}

// GetReleaseNamespace returns the configured TargetNamespace, or the namespace
// of the HelmRelease.
func (in HelmRelease) GetReleaseNamespace() string {
	if in.Spec.TargetNamespace != "" {
		return in.Spec.TargetNamespace
	}
	return in.Namespace
}

// GetStorageNamespace returns the configured StorageNamespace for helm, or the namespace
// of the HelmRelease.
func (in HelmRelease) GetStorageNamespace() string {
	if in.Spec.StorageNamespace != "" {
		return in.Spec.StorageNamespace
	}
	return in.Namespace
}

// GetHelmChartName returns the name used by the controller for the HelmChart creation.
func (in HelmRelease) GetHelmChartName() string {
	return strings.Join([]string{in.Namespace, in.Name}, "-")
}

// GetTimeout returns the configured Timeout, or the default of 300s.
func (in HelmRelease) GetTimeout() metav1.Duration {
	if in.Spec.Timeout == nil {
		return metav1.Duration{Duration: 300 * time.Second}
	}
	return *in.Spec.Timeout
}

// GetMaxHistory returns the configured MaxHistory, or the default of 10.
func (in HelmRelease) GetMaxHistory() int {
	if in.Spec.MaxHistory == nil {
		return 10
	}
	return *in.Spec.MaxHistory
}

// GetDependsOn returns the list of dependencies across-namespaces.
func (in HelmRelease) GetDependsOn() []meta.NamespacedObjectReference {
	return in.Spec.DependsOn
}

// GetConditions returns the status conditions of the object.
func (in HelmRelease) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *HelmRelease) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetStatusConditions returns a pointer to the Status.Conditions slice.
// Deprecated: use GetConditions instead.
func (in *HelmRelease) GetStatusConditions() *[]metav1.Condition {
	return &in.Status.Conditions
}

// +kubebuilder:object:root=true

// HelmReleaseList contains a list of HelmRelease objects.
type HelmReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HelmRelease `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HelmRelease{}, &HelmReleaseList{})
}
