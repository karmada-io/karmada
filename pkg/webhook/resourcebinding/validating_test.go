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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubescheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	policyv1alpha1 "github.com/karmada-io/karmada/pkg/apis/policy/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/features"
)

// testScheme is the scheme used for fake client in tests.
var testScheme = runtime.NewScheme()

func init() {
	_ = kubescheme.AddToScheme(testScheme)
	_ = workv1alpha2.Install(testScheme)
	_ = policyv1alpha1.Install(testScheme)
}

// fakeDecoder mocks admission.Decoder for testing.
type fakeDecoder struct {
	err           error
	decodeObj     runtime.Object // For Decode method (Request.Object -> typed object)
	rawDecodedObj runtime.Object // For DecodeRaw method (Request.OldObject -> typed object)
}

func (f *fakeDecoder) Decode(_ admission.Request, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.decodeObj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.decodeObj).Elem())
	}
	return nil
}

func (f *fakeDecoder) DecodeRaw(_ runtime.RawExtension, obj runtime.Object) error {
	if f.err != nil {
		return f.err
	}
	if f.rawDecodedObj != nil {
		reflect.ValueOf(obj).Elem().Set(reflect.ValueOf(f.rawDecodedObj).Elem())
	}
	return nil
}

func marshalToRawExtension(t *testing.T, obj runtime.Object) runtime.RawExtension {
	t.Helper()
	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("Failed to marshal object %T: %v", obj, err)
	}
	return runtime.RawExtension{Raw: raw}
}

// --- RB Options ---
type RBOption func(*workv1alpha2.ResourceBinding)

func WithClusters(clusters []workv1alpha2.TargetCluster) RBOption {
	return func(rb *workv1alpha2.ResourceBinding) {
		rb.Spec.Clusters = clusters
	}
}

func WithReplicaRequirements(reqs corev1.ResourceList) RBOption {
	return func(rb *workv1alpha2.ResourceBinding) {
		if reqs != nil {
			rb.Spec.ReplicaRequirements = &workv1alpha2.ReplicaRequirements{ResourceRequest: reqs}
		} else {
			rb.Spec.ReplicaRequirements = nil
		}
	}
}

func WithResourceRef(ref workv1alpha2.ObjectReference) RBOption {
	return func(rb *workv1alpha2.ResourceBinding) {
		rb.Spec.Resource = ref
	}
}

func makeTestRB(namespace, name string, opts ...RBOption) *workv1alpha2.ResourceBinding {
	defaultResourceRef := workv1alpha2.ObjectReference{
		APIVersion: "v1", Kind: "Pod", Name: "test-pod-" + name, Namespace: namespace,
	}
	rb := &workv1alpha2.ResourceBinding{
		TypeMeta: metav1.TypeMeta{APIVersion: workv1alpha2.SchemeGroupVersion.String(), Kind: "ResourceBinding"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(fmt.Sprintf("rb-obj-uid-%s-%s", namespace, name)), // UID for the RB object itself
		},
		Spec: workv1alpha2.ResourceBindingSpec{Resource: defaultResourceRef},
	}
	for _, opt := range opts {
		opt(rb)
	}
	return rb
}

// --- FRQ Options ---
type FRQOption func(*policyv1alpha1.FederatedResourceQuota)

func WithOverallLimits(limits corev1.ResourceList) FRQOption {
	return func(frq *policyv1alpha1.FederatedResourceQuota) {
		frq.Spec.Overall = limits
	}
}

func WithOverallUsed(used corev1.ResourceList) FRQOption {
	return func(frq *policyv1alpha1.FederatedResourceQuota) {
		if used != nil {
			frq.Status.OverallUsed = used
		} else {
			frq.Status.OverallUsed = corev1.ResourceList{}
		}
	}
}

func makeTestFRQ(namespace, name string, opts ...FRQOption) *policyv1alpha1.FederatedResourceQuota {
	frq := &policyv1alpha1.FederatedResourceQuota{
		TypeMeta: metav1.TypeMeta{APIVersion: policyv1alpha1.SchemeGroupVersion.String(), Kind: "FederatedResourceQuota"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       types.UID(fmt.Sprintf("frq-obj-uid-%s-%s", namespace, name)), // UID for the FRQ object itself
		},
		Spec: policyv1alpha1.FederatedResourceQuotaSpec{},
	}
	for _, opt := range opts {
		opt(frq)
	}
	return frq
}

// boolPtr returns a pointer to a boolean value.
func boolPtr(b bool) *bool { return &b }

// --- Admission Request Builder ---
type admissionRequestBuilder struct {
	t         *testing.T
	operation admissionv1.Operation
	namespace string
	name      string // Resource Name
	uidSuffix string // Suffix for generating a unique request UID
	object    runtime.Object
	oldObject runtime.Object
	dryRun    *bool
}

func newAdmissionRequestBuilder(t *testing.T, op admissionv1.Operation, resourceNamespace, resourceName, uidSuffix string) *admissionRequestBuilder {
	return &admissionRequestBuilder{
		t:         t,
		operation: op,
		namespace: resourceNamespace,
		name:      resourceName,
		uidSuffix: uidSuffix,
	}
}

func (b *admissionRequestBuilder) WithObject(obj runtime.Object) *admissionRequestBuilder {
	b.object = obj
	return b
}

func (b *admissionRequestBuilder) WithOldObject(oldObj runtime.Object) *admissionRequestBuilder {
	b.oldObject = oldObj
	return b
}

func (b *admissionRequestBuilder) WithDryRun(isDryRun bool) *admissionRequestBuilder {
	b.dryRun = boolPtr(isDryRun)
	return b
}

func (b *admissionRequestBuilder) Build() admission.Request {
	// Generate a descriptive UID for the admission request itself
	requestUID := types.UID(fmt.Sprintf("req-uid-%s-%s-%s",
		strings.ToLower(string(b.operation)),
		strings.ToLower(b.name), // resource name
		b.uidSuffix))

	req := admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: b.operation,
			Name:      b.name,
			Namespace: b.namespace,
			UID:       requestUID,
		},
	}
	if b.object != nil {
		req.Object = marshalToRawExtension(b.t, b.object)
	}
	if b.oldObject != nil {
		req.OldObject = marshalToRawExtension(b.t, b.oldObject)
	}
	if b.dryRun != nil {
		req.DryRun = b.dryRun
	}
	return req
}

func TestValidatingAdmission_Handle(t *testing.T) {
	// --- Common Test Data ---
	commonRBUpdatePassOld := makeTestRB("quota-ns", "rb-update-pass",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1", Replicas: 1}}),
		WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m")}),
	)
	commonRBUpdatePassNew := makeTestRB("quota-ns", "rb-update-pass",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1", Replicas: 1}}),
		WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
	)

	// --- Test Case Specific Data ---

	// For: "decode error on new object"
	rbForDecodeErrorCase := makeTestRB("default", "test-rb-decode-err") // A minimal RB for the request
	frqForDecodeErrorCase := makeTestFRQ("default", "frq-for-decode-error",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
	)

	// For: "create operation not yet scheduled should be allowed"
	rbUnscheduledCreate := makeTestRB("default", "rb-unscheduled",
		WithClusters(nil),
		WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
	)
	frqForUnscheduledCreate := makeTestFRQ("default", "frq-for-create-unscheduled", // Unique FRQ name
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
	)

	// For: "update operation with no relevant field change should be allowed"
	rbForNoChange := makeTestRB("default", "rb-nochange",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1", Replicas: 1}}),
		WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
	)
	frqForNoChange := makeTestFRQ("default", "frq-for-nochange",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
	)

	// For: "total RB delta is zero should be allowed (update with identical states)"
	rbForDeltaZeroNew := makeTestRB("default", "rb-delta-zero", // Use same name for new/old RBs in this scenario
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1", Replicas: 1}}),
		WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
	)
	rbForDeltaZeroOld := makeTestRB("default", "rb-delta-zero",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m2", Replicas: 2}}),
		WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
	)
	frqForDeltaZero := makeTestFRQ("default", "frq-fordeltazero",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1Gi")}),
		WithOverallUsed(nil),
	)

	// For: "no FRQs found should be allowed"
	rbForNoFRQ := makeTestRB("ns-no-frq", "test-rb-no-frq",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1", Replicas: 1}}),
		WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}),
	)

	// For: "create quota exceeded should deny"
	rbCreateExceeds := makeTestRB("quota-ns", "rb-create-exceed",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1", Replicas: 1}}),
		WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
	)
	frqForCreateExceeds := makeTestFRQ("quota-ns", "frq-create-exceeds",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("150m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0m")}),
	)

	// For: "update passes quota (allowed response, non-dryrun)"
	frqForUpdatePassNonDryRun := makeTestFRQ("quota-ns", "frq-update-pass-nondryrun",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
	)

	// For: "update passes quota (allowed response, dry run)"
	frqForUpdatePassDryRun := makeTestFRQ("quota-ns", "frq-update-pass-dryrun",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
	)

	// For: "update fails quota (denied response)"
	rbUpdateFailOld := makeTestRB("quota-ns", "rb-update-fail",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1", Replicas: 1}}),
		WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("50m")}),
	)
	rbUpdateFailNew := makeTestRB("quota-ns", "rb-update-fail",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1", Replicas: 2}}),
		WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("80m")}),
	)
	frqForUpdateFail := makeTestFRQ("quota-ns", "frq-update-fail",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0m")}),
	)

	tests := []struct {
		name               string
		req                admission.Request
		decoder            admission.Decoder
		clientObjects      []client.Object
		featureGateEnabled bool
		wantResponse       admission.Response
	}{
		{
			name:               "feature gate disabled",
			req:                newAdmissionRequestBuilder(t, admissionv1.Create, "default", "any-rb", "fg-disabled").Build(),
			decoder:            &fakeDecoder{},
			clientObjects:      nil,
			featureGateEnabled: false,
			wantResponse:       admission.Allowed(""),
		},
		{
			name: "decode error on new object",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, rbForDecodeErrorCase.Namespace, rbForDecodeErrorCase.Name, "decode-err").
				WithObject(rbForDecodeErrorCase).
				Build(),
			decoder:            &fakeDecoder{err: errors.New("decode failed")},
			clientObjects:      []client.Object{frqForDecodeErrorCase},
			featureGateEnabled: true,
			wantResponse:       admission.Errored(http.StatusBadRequest, errors.New("decode failed")),
		},
		{
			name: "create operation not yet scheduled should be allowed",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, rbUnscheduledCreate.Namespace, rbUnscheduledCreate.Name, "create-unscheduled").
				WithObject(rbUnscheduledCreate).
				Build(),
			decoder:            &fakeDecoder{decodeObj: rbUnscheduledCreate},
			clientObjects:      []client.Object{frqForUnscheduledCreate},
			featureGateEnabled: true,
			wantResponse:       admission.Allowed("ResourceBinding not yet scheduled for Create operation."),
		},
		{
			name: "update operation with no relevant field change should be allowed",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, rbForNoChange.Namespace, rbForNoChange.Name, "update-nochange").
				WithObject(rbForNoChange).
				WithOldObject(rbForNoChange).
				Build(),
			decoder:            &fakeDecoder{decodeObj: rbForNoChange, rawDecodedObj: rbForNoChange},
			clientObjects:      []client.Object{frqForNoChange},
			featureGateEnabled: true,
			wantResponse:       admission.Allowed("No quota relevant fields changed."),
		},
		{
			name: "total RB delta is zero should be allowed (update with identical states)",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, rbForDeltaZeroNew.Namespace, rbForDeltaZeroNew.Name, "delta-zero").
				WithObject(rbForDeltaZeroNew).
				WithOldObject(rbForDeltaZeroOld).
				Build(),
			decoder:            &fakeDecoder{decodeObj: rbForDeltaZeroNew, rawDecodedObj: rbForDeltaZeroOld},
			clientObjects:      []client.Object{frqForDeltaZero},
			featureGateEnabled: true,
			wantResponse:       admission.Allowed("No effective resource quantity delta for ResourceBinding, skipping quota validation."),
		},
		{
			name: "no FRQs found should be allowed",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, rbForNoFRQ.Namespace, rbForNoFRQ.Name, "no-frq-found").
				WithObject(rbForNoFRQ).
				Build(),
			decoder:            &fakeDecoder{decodeObj: rbForNoFRQ},
			clientObjects:      []client.Object{},
			featureGateEnabled: true,
			wantResponse:       admission.Allowed("No FederatedResourceQuotas found in the namespace, skipping quota check."),
		},
		{
			name: "create quota exceeded should deny",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, rbCreateExceeds.Namespace, rbCreateExceeds.Name, "create-exceeds").
				WithObject(rbCreateExceeds).
				Build(),
			decoder:            &fakeDecoder{decodeObj: rbCreateExceeds},
			clientObjects:      []client.Object{frqForCreateExceeds},
			featureGateEnabled: true,
			wantResponse:       admission.Denied("FederatedResourceQuota(quota-ns/frq-create-exceeds) exceeded for resource cpu: requested sum 200m, limit 150m."),
		},
		{
			name: "update passes quota (allowed response, non-dryrun)",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, commonRBUpdatePassNew.Namespace, commonRBUpdatePassNew.Name, "update-pass-nondryrun").
				WithObject(commonRBUpdatePassNew).
				WithOldObject(commonRBUpdatePassOld).
				WithDryRun(false).
				Build(),
			decoder:            &fakeDecoder{decodeObj: commonRBUpdatePassNew, rawDecodedObj: commonRBUpdatePassOld},
			clientObjects:      []client.Object{frqForUpdatePassNonDryRun},
			featureGateEnabled: true,
			wantResponse: admission.Allowed(
				fmt.Sprintf("All relevant FederatedResourceQuota checks passed for ResourceBinding %s/%s. Quota check passed for FRQ %s/%s. 1 FRQ(s) updated.",
					commonRBUpdatePassNew.Namespace, commonRBUpdatePassNew.Name, frqForUpdatePassNonDryRun.Namespace, frqForUpdatePassNonDryRun.Name),
			),
		},
		{
			name: "update passes quota (allowed response, dry run)",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, commonRBUpdatePassNew.Namespace, commonRBUpdatePassNew.Name, "update-pass-dryrun").
				WithObject(commonRBUpdatePassNew).
				WithOldObject(commonRBUpdatePassOld).
				WithDryRun(true).
				Build(),
			decoder:            &fakeDecoder{decodeObj: commonRBUpdatePassNew, rawDecodedObj: commonRBUpdatePassOld},
			clientObjects:      []client.Object{frqForUpdatePassDryRun},
			featureGateEnabled: true,
			wantResponse: admission.Allowed(
				fmt.Sprintf("All relevant FederatedResourceQuota checks passed for ResourceBinding %s/%s. Quota check passed for FRQ %s/%s. 1 FRQ(s) updated.",
					commonRBUpdatePassNew.Namespace, commonRBUpdatePassNew.Name, frqForUpdatePassDryRun.Namespace, frqForUpdatePassDryRun.Name),
			),
		},
		{
			name: "update fails quota (denied response)",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, rbUpdateFailNew.Namespace, rbUpdateFailNew.Name, "update-fail").
				WithObject(rbUpdateFailNew).
				WithOldObject(rbUpdateFailOld).
				Build(),
			decoder:            &fakeDecoder{decodeObj: rbUpdateFailNew, rawDecodedObj: rbUpdateFailOld},
			clientObjects:      []client.Object{frqForUpdateFail},
			featureGateEnabled: true,
			wantResponse:       admission.Denied("FederatedResourceQuota(quota-ns/frq-update-fail) exceeded for resource cpu: requested sum 110m, limit 100m."),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalGateState := features.FeatureGate.Enabled(features.FederatedQuotaEnforcement)
			if errFG := features.FeatureGate.Set(fmt.Sprintf("%s=%v", features.FederatedQuotaEnforcement, tt.featureGateEnabled)); errFG != nil {
				t.Fatalf("Failed to set feature gate %s to %v: %v", features.FederatedQuotaEnforcement, tt.featureGateEnabled, errFG)
			}
			t.Cleanup(func() {
				if errFG := features.FeatureGate.Set(fmt.Sprintf("%s=%v", features.FederatedQuotaEnforcement, originalGateState)); errFG != nil {
					t.Logf("Warning: Failed to restore feature gate %s to %v: %v", features.FederatedQuotaEnforcement, originalGateState, errFG)
				}
			})

			fakeClientBuilder := fake.NewClientBuilder().WithScheme(testScheme)
			if len(tt.clientObjects) > 0 {
				fakeClientBuilder = fakeClientBuilder.WithObjects(tt.clientObjects...)
			}
			fakeClient := fakeClientBuilder.WithStatusSubresource(&policyv1alpha1.FederatedResourceQuota{}).Build()

			v := ValidatingAdmission{
				Client:  fakeClient,
				Decoder: tt.decoder,
			}
			got := v.Handle(context.Background(), tt.req)

			if got.Allowed != tt.wantResponse.Allowed {
				t.Errorf("Handle() got.Allowed = %v, want %v. Got Response: %+v", got.Allowed, tt.wantResponse.Allowed, got)
			}

			if (got.Result == nil) != (tt.wantResponse.Result == nil) {
				t.Errorf("Handle() got.Result nil status mismatch: got %v (%+v), want %v (%+v)",
					(got.Result == nil), got.Result, (tt.wantResponse.Result == nil), tt.wantResponse.Result)
			}
			if got.Result != nil && tt.wantResponse.Result != nil {
				if got.Result.Message != tt.wantResponse.Result.Message {
					t.Errorf("Handle() got.Result.Message = %q, want %q", got.Result.Message, tt.wantResponse.Result.Message)
				}
				if got.Result.Code != tt.wantResponse.Result.Code {
					t.Errorf("Handle() got.Result.Code = %d, want %d", got.Result.Code, tt.wantResponse.Result.Code)
				}
			}

			if len(got.Patches) != 0 {
				t.Errorf("Handle() returned %d patches, want 0 for a validating webhook. Patches: %v", len(got.Patches), got.Patches)
			}
		})
	}
}
