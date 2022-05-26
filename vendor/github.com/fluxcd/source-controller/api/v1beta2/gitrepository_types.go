/*
Copyright 2022 The Flux authors

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

package v1beta2

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/apis/meta"
)

const (
	// GitRepositoryKind is the string representation of a GitRepository.
	GitRepositoryKind = "GitRepository"

	// GoGitImplementation for performing Git operations using go-git.
	GoGitImplementation = "go-git"
	// LibGit2Implementation for performing Git operations using libgit2.
	LibGit2Implementation = "libgit2"
)

const (
	// IncludeUnavailableCondition indicates one of the includes is not
	// available. For example, because it does not exist, or does not have an
	// Artifact.
	// This is a "negative polarity" or "abnormal-true" type, and is only
	// present on the resource if it is True.
	IncludeUnavailableCondition string = "IncludeUnavailable"
)

// GitRepositorySpec specifies the required configuration to produce an
// Artifact for a Git repository.
type GitRepositorySpec struct {
	// URL specifies the Git repository URL, it can be an HTTP/S or SSH address.
	// +kubebuilder:validation:Pattern="^(http|https|ssh)://"
	// +required
	URL string `json:"url"`

	// SecretRef specifies the Secret containing authentication credentials for
	// the GitRepository.
	// For HTTPS repositories the Secret must contain 'username' and 'password'
	// fields.
	// For SSH repositories the Secret must contain 'identity'
	// and 'known_hosts' fields.
	// +optional
	SecretRef *meta.LocalObjectReference `json:"secretRef,omitempty"`

	// Interval at which to check the GitRepository for updates.
	// +required
	Interval metav1.Duration `json:"interval"`

	// Timeout for Git operations like cloning, defaults to 60s.
	// +kubebuilder:default="60s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Reference specifies the Git reference to resolve and monitor for
	// changes, defaults to the 'master' branch.
	// +optional
	Reference *GitRepositoryRef `json:"ref,omitempty"`

	// Verification specifies the configuration to verify the Git commit
	// signature(s).
	// +optional
	Verification *GitRepositoryVerification `json:"verify,omitempty"`

	// Ignore overrides the set of excluded patterns in the .sourceignore format
	// (which is the same as .gitignore). If not provided, a default will be used,
	// consult the documentation for your version to find out what those are.
	// +optional
	Ignore *string `json:"ignore,omitempty"`

	// Suspend tells the controller to suspend the reconciliation of this
	// GitRepository.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// GitImplementation specifies which Git client library implementation to
	// use. Defaults to 'go-git', valid values are ('go-git', 'libgit2').
	// +kubebuilder:validation:Enum=go-git;libgit2
	// +kubebuilder:default:=go-git
	// +optional
	GitImplementation string `json:"gitImplementation,omitempty"`

	// RecurseSubmodules enables the initialization of all submodules within
	// the GitRepository as cloned from the URL, using their default settings.
	// This option is available only when using the 'go-git' GitImplementation.
	// +optional
	RecurseSubmodules bool `json:"recurseSubmodules,omitempty"`

	// Include specifies a list of GitRepository resources which Artifacts
	// should be included in the Artifact produced for this GitRepository.
	Include []GitRepositoryInclude `json:"include,omitempty"`

	// AccessFrom specifies an Access Control List for allowing cross-namespace
	// references to this object.
	// NOTE: Not implemented, provisional as of https://github.com/fluxcd/flux2/pull/2092
	// +optional
	AccessFrom *acl.AccessFrom `json:"accessFrom,omitempty"`
}

// GitRepositoryInclude specifies a local reference to a GitRepository which
// Artifact (sub-)contents must be included, and where they should be placed.
type GitRepositoryInclude struct {
	// GitRepositoryRef specifies the GitRepository which Artifact contents
	// must be included.
	GitRepositoryRef meta.LocalObjectReference `json:"repository"`

	// FromPath specifies the path to copy contents from, defaults to the root
	// of the Artifact.
	// +optional
	FromPath string `json:"fromPath"`

	// ToPath specifies the path to copy contents to, defaults to the name of
	// the GitRepositoryRef.
	// +optional
	ToPath string `json:"toPath"`
}

// GetFromPath returns the specified FromPath.
func (in *GitRepositoryInclude) GetFromPath() string {
	return in.FromPath
}

// GetToPath returns the specified ToPath, falling back to the name of the
// GitRepositoryRef.
func (in *GitRepositoryInclude) GetToPath() string {
	if in.ToPath == "" {
		return in.GitRepositoryRef.Name
	}
	return in.ToPath
}

// GitRepositoryRef specifies the Git reference to resolve and checkout.
type GitRepositoryRef struct {
	// Branch to check out, defaults to 'master' if no other field is defined.
	//
	// When GitRepositorySpec.GitImplementation is set to 'go-git', a shallow
	// clone of the specified branch is performed.
	// +optional
	Branch string `json:"branch,omitempty"`

	// Tag to check out, takes precedence over Branch.
	// +optional
	Tag string `json:"tag,omitempty"`

	// SemVer tag expression to check out, takes precedence over Tag.
	// +optional
	SemVer string `json:"semver,omitempty"`

	// Commit SHA to check out, takes precedence over all reference fields.
	//
	// When GitRepositorySpec.GitImplementation is set to 'go-git', this can be
	// combined with Branch to shallow clone the branch, in which the commit is
	// expected to exist.
	// +optional
	Commit string `json:"commit,omitempty"`
}

// GitRepositoryVerification specifies the Git commit signature verification
// strategy.
type GitRepositoryVerification struct {
	// Mode specifies what Git object should be verified, currently ('head').
	// +kubebuilder:validation:Enum=head
	Mode string `json:"mode"`

	// SecretRef specifies the Secret containing the public keys of trusted Git
	// authors.
	SecretRef meta.LocalObjectReference `json:"secretRef,omitempty"`
}

// GitRepositoryStatus records the observed state of a Git repository.
type GitRepositoryStatus struct {
	// ObservedGeneration is the last observed generation of the GitRepository
	// object.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions holds the conditions for the GitRepository.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// URL is the dynamic fetch link for the latest Artifact.
	// It is provided on a "best effort" basis, and using the precise
	// GitRepositoryStatus.Artifact data is recommended.
	// +optional
	URL string `json:"url,omitempty"`

	// Artifact represents the last successful GitRepository reconciliation.
	// +optional
	Artifact *Artifact `json:"artifact,omitempty"`

	// IncludedArtifacts contains a list of the last successfully included
	// Artifacts as instructed by GitRepositorySpec.Include.
	// +optional
	IncludedArtifacts []*Artifact `json:"includedArtifacts,omitempty"`

	meta.ReconcileRequestStatus `json:",inline"`
}

const (
	// GitOperationSucceedReason signals that a Git operation (e.g. clone,
	// checkout, etc.) succeeded.
	GitOperationSucceedReason string = "GitOperationSucceeded"

	// GitOperationFailedReason signals that a Git operation (e.g. clone,
	// checkout, etc.) failed.
	GitOperationFailedReason string = "GitOperationFailed"
)

// GetConditions returns the status conditions of the object.
func (in GitRepository) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *GitRepository) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetRequeueAfter returns the duration after which the GitRepository must be
// reconciled again.
func (in GitRepository) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// GetArtifact returns the latest Artifact from the GitRepository if present in
// the status sub-resource.
func (in *GitRepository) GetArtifact() *Artifact {
	return in.Status.Artifact
}

// +genclient
// +genclient:Namespaced
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=gitrepo
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.spec.url`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// GitRepository is the Schema for the gitrepositories API.
type GitRepository struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GitRepositorySpec `json:"spec,omitempty"`
	// +kubebuilder:default={"observedGeneration":-1}
	Status GitRepositoryStatus `json:"status,omitempty"`
}

// GitRepositoryList contains a list of GitRepository objects.
// +kubebuilder:object:root=true
type GitRepositoryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitRepository `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitRepository{}, &GitRepositoryList{})
}
