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

package meta

const (
	// ReconcileRequestAnnotation is the annotation used for triggering a reconciliation
	// outside of a defined schedule. The value is interpreted as a token, and any change
	// in value SHOULD trigger a reconciliation.
	ReconcileRequestAnnotation string = "reconcile.fluxcd.io/requestedAt"
)

// ReconcileAnnotationValue returns a value for the reconciliation request annotation, which can be used to detect
// changes; and, a boolean indicating whether the annotation was set.
func ReconcileAnnotationValue(annotations map[string]string) (string, bool) {
	requestedAt, ok := annotations[ReconcileRequestAnnotation]
	return requestedAt, ok
}

// ReconcileRequestStatus is a struct to embed in a status type, so that all types using the mechanism have the same
// field. Use it like this:
//
//	type FooStatus struct {
//  	meta.ReconcileRequestStatus `json:",inline"`
//  	// other status fields...
// 	}
type ReconcileRequestStatus struct {
	// LastHandledReconcileAt holds the value of the most recent
	// reconcile request value, so a change of the annotation value
	// can be detected.
	// +optional
	LastHandledReconcileAt string `json:"lastHandledReconcileAt,omitempty"`
}

// GetLastHandledReconcileRequest returns the most recent reconcile request value from the ReconcileRequestStatus.
func (in ReconcileRequestStatus) GetLastHandledReconcileRequest() string {
	return in.LastHandledReconcileAt
}

// SetLastHandledReconcileRequest sets the most recent reconcile request value in the ReconcileRequestStatus.
func (in *ReconcileRequestStatus) SetLastHandledReconcileRequest(token string) {
	in.LastHandledReconcileAt = token
}

// StatusWithHandledReconcileRequest describes a status type which holds the value of the most recent
// ReconcileAnnotationValue.
// +k8s:deepcopy-gen=false
type StatusWithHandledReconcileRequest interface {
	GetLastHandledReconcileRequest() string
}

// StatusWithHandledReconcileRequestSetter describes a status with a setter for the most ReconcileAnnotationValue.
// +k8s:deepcopy-gen=false
type StatusWithHandledReconcileRequestSetter interface {
	SetLastHandledReconcileRequest(token string)
}
