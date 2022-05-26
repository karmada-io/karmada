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
	// BucketKind is the string representation of a Bucket.
	BucketKind = "Bucket"
)

const (
	// GenericBucketProvider for any S3 API compatible storage Bucket.
	GenericBucketProvider string = "generic"
	// AmazonBucketProvider for an AWS S3 object storage Bucket.
	// Provides support for retrieving credentials from the AWS EC2 service.
	AmazonBucketProvider string = "aws"
	// GoogleBucketProvider for a Google Cloud Storage Bucket.
	// Provides support for authentication using a workload identity.
	GoogleBucketProvider string = "gcp"
	// AzureBucketProvider for an Azure Blob Storage Bucket.
	// Provides support for authentication using a Service Principal,
	// Managed Identity or Shared Key.
	AzureBucketProvider string = "azure"
)

// BucketSpec specifies the required configuration to produce an Artifact for
// an object storage bucket.
type BucketSpec struct {
	// Provider of the object storage bucket.
	// Defaults to 'generic', which expects an S3 (API) compatible object
	// storage.
	// +kubebuilder:validation:Enum=generic;aws;gcp;azure
	// +kubebuilder:default:=generic
	// +optional
	Provider string `json:"provider,omitempty"`

	// BucketName is the name of the object storage bucket.
	// +required
	BucketName string `json:"bucketName"`

	// Endpoint is the object storage address the BucketName is located at.
	// +required
	Endpoint string `json:"endpoint"`

	// Insecure allows connecting to a non-TLS HTTP Endpoint.
	// +optional
	Insecure bool `json:"insecure,omitempty"`

	// Region of the Endpoint where the BucketName is located in.
	// +optional
	Region string `json:"region,omitempty"`

	// SecretRef specifies the Secret containing authentication credentials
	// for the Bucket.
	// +optional
	SecretRef *meta.LocalObjectReference `json:"secretRef,omitempty"`

	// Interval at which to check the Endpoint for updates.
	// +required
	Interval metav1.Duration `json:"interval"`

	// Timeout for fetch operations, defaults to 60s.
	// +kubebuilder:default="60s"
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	// Ignore overrides the set of excluded patterns in the .sourceignore format
	// (which is the same as .gitignore). If not provided, a default will be used,
	// consult the documentation for your version to find out what those are.
	// +optional
	Ignore *string `json:"ignore,omitempty"`

	// Suspend tells the controller to suspend the reconciliation of this
	// Bucket.
	// +optional
	Suspend bool `json:"suspend,omitempty"`

	// AccessFrom specifies an Access Control List for allowing cross-namespace
	// references to this object.
	// NOTE: Not implemented, provisional as of https://github.com/fluxcd/flux2/pull/2092
	// +optional
	AccessFrom *acl.AccessFrom `json:"accessFrom,omitempty"`
}

// BucketStatus records the observed state of a Bucket.
type BucketStatus struct {
	// ObservedGeneration is the last observed generation of the Bucket object.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions holds the conditions for the Bucket.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// URL is the dynamic fetch link for the latest Artifact.
	// It is provided on a "best effort" basis, and using the precise
	// BucketStatus.Artifact data is recommended.
	// +optional
	URL string `json:"url,omitempty"`

	// Artifact represents the last successful Bucket reconciliation.
	// +optional
	Artifact *Artifact `json:"artifact,omitempty"`

	meta.ReconcileRequestStatus `json:",inline"`
}

const (
	// BucketOperationSucceededReason signals that the Bucket listing and fetch
	// operations succeeded.
	BucketOperationSucceededReason string = "BucketOperationSucceeded"

	// BucketOperationFailedReason signals that the Bucket listing or fetch
	// operations failed.
	BucketOperationFailedReason string = "BucketOperationFailed"
)

// GetConditions returns the status conditions of the object.
func (in Bucket) GetConditions() []metav1.Condition {
	return in.Status.Conditions
}

// SetConditions sets the status conditions on the object.
func (in *Bucket) SetConditions(conditions []metav1.Condition) {
	in.Status.Conditions = conditions
}

// GetRequeueAfter returns the duration after which the source must be reconciled again.
func (in Bucket) GetRequeueAfter() time.Duration {
	return in.Spec.Interval.Duration
}

// GetArtifact returns the latest artifact from the source if present in the status sub-resource.
func (in *Bucket) GetArtifact() *Artifact {
	return in.Status.Artifact
}

// +genclient
// +genclient:Namespaced
// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Endpoint",type=string,JSONPath=`.spec.endpoint`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].message",description=""

// Bucket is the Schema for the buckets API.
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BucketSpec `json:"spec,omitempty"`
	// +kubebuilder:default={"observedGeneration":-1}
	Status BucketStatus `json:"status,omitempty"`
}

// BucketList contains a list of Bucket objects.
// +kubebuilder:object:root=true
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bucket{}, &BucketList{})
}
