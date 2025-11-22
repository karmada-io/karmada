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

func WithComponents(components []workv1alpha2.Component) RBOption {
	return func(rb *workv1alpha2.ResourceBinding) {
		rb.Spec.Components = components
	}
}

func WithResourceRef(ref workv1alpha2.ObjectReference) RBOption {
	return func(rb *workv1alpha2.ResourceBinding) {
		rb.Spec.Resource = ref
	}
}

func WithAnnotations(annotations map[string]string) RBOption {
	return func(rb *workv1alpha2.ResourceBinding) {
		if rb.Annotations == nil {
			rb.Annotations = make(map[string]string)
		}
		for k, v := range annotations {
			rb.Annotations[k] = v
		}
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

	makeTestComponent := func(name string, replicas int32, cpu string) workv1alpha2.Component {
		return workv1alpha2.Component{
			Name:     name,
			Replicas: replicas,
			ReplicaRequirements: &workv1alpha2.ComponentReplicaRequirements{
				ResourceRequest: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu)},
			},
		}
	}
	rbWithComponents := makeTestRB("quota-ns", "rb-with-components",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1"}, {Name: "m2"}}),
		WithComponents([]workv1alpha2.Component{
			makeTestComponent("comp1", 2, "25m"),
			makeTestComponent("comp2", 1, "30m"),
		}),
	)
	// For: "create with components and sufficient quota (allowed response)"
	frqForComponentsSufficient := makeTestFRQ("quota-ns", "frq-components-sufficient",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0m")}),
	)
	// For: "create with components and exceeded quota (denied response)"
	frqForComponentsExceeded := makeTestFRQ("quota-ns", "frq-components-exceeded",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("150m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0m")}),
	)
	// For: "update components passes quota (allowed response)"
	rbComponentsPassOld := makeTestRB("quota-ns", "rb-components-pass",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1"}}),
		WithComponents([]workv1alpha2.Component{
			makeTestComponent("comp-old", 1, "50m"),
		}),
	)
	rbComponentsPassNew := makeTestRB("quota-ns", "rb-components-pass",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1"}}),
		WithComponents([]workv1alpha2.Component{
			makeTestComponent("comp-new", 2, "40m"),
		}),
	)
	frqForComponentsPass := makeTestFRQ("quota-ns", "frq-components-pass",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
	)

	// For: "update components fails quota (denied response)"
	rbComponentsFailOld := makeTestRB("quota-ns", "rb-components-fail",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1"}}),
		WithComponents([]workv1alpha2.Component{
			makeTestComponent("comp-old", 1, "50m"),
		}),
	)
	rbComponentsFailNew := makeTestRB("quota-ns", "rb-components-fail",
		WithClusters([]workv1alpha2.TargetCluster{{Name: "m1"}}),
		WithComponents([]workv1alpha2.Component{
			makeTestComponent("comp-new", 2, "80m"),
		}),
	)
	frqForComponentsFail := makeTestFRQ("quota-ns", "frq-components-fail",
		WithOverallLimits(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
		WithOverallUsed(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0m")}),
	)

	tests := []struct {
		name                                            string
		req                                             admission.Request
		decoder                                         admission.Decoder
		clientObjects                                   []client.Object
		enableFederatedQuotaEnforcementFeatureGate      bool
		enableMultiplePodTemplatesSchedulingFeatureGate bool
		wantResponse                                    admission.Response
	}{
		{
			name:          "feature gate disabled",
			req:           newAdmissionRequestBuilder(t, admissionv1.Create, "default", "any-rb", "fg-disabled").Build(),
			decoder:       &fakeDecoder{},
			clientObjects: nil,
			enableFederatedQuotaEnforcementFeatureGate: false,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "decode error on new object",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, rbForDecodeErrorCase.Namespace, rbForDecodeErrorCase.Name, "decode-err").
				WithObject(rbForDecodeErrorCase).
				Build(),
			decoder:       &fakeDecoder{err: errors.New("decode failed")},
			clientObjects: []client.Object{frqForDecodeErrorCase},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Errored(http.StatusBadRequest, errors.New("decode failed")),
		},
		{
			name: "valid dependencies annotation should be allowed",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, "default", "rb-valid-deps", "valid-deps").
				WithObject(makeTestRB("default", "rb-valid-deps",
					WithAnnotations(map[string]string{
						"resourcebinding.karmada.io/dependencies": `[{"apiVersion":"v1","kind":"ConfigMap","namespace":"default","name":"my-config"}]`,
					}))).
				Build(),
			decoder: &fakeDecoder{decodeObj: makeTestRB("default", "rb-valid-deps",
				WithAnnotations(map[string]string{
					"resourcebinding.karmada.io/dependencies": `[{"apiVersion":"v1","kind":"ConfigMap","namespace":"default","name":"my-config"}]`,
				}))},
			clientObjects: nil,
			enableFederatedQuotaEnforcementFeatureGate: false,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "invalid dependencies annotation should be denied",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, "default", "rb-invalid-deps", "invalid-deps").
				WithObject(makeTestRB("default", "rb-invalid-deps",
					WithAnnotations(map[string]string{
						"resourcebinding.karmada.io/dependencies": `invalid-json-string`,
					}))).
				Build(),
			decoder: &fakeDecoder{decodeObj: makeTestRB("default", "rb-invalid-deps",
				WithAnnotations(map[string]string{
					"resourcebinding.karmada.io/dependencies": `invalid-json-string`,
				}))},
			clientObjects: nil,
			enableFederatedQuotaEnforcementFeatureGate: false,
			wantResponse: admission.Denied("metadata.annotations.resourcebinding.karmada.io/dependencies: Invalid value: \"invalid-json-string\": annotation value must be a valid JSON"),
		},
		{
			name: "no dependencies annotation should be allowed",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, "default", "rb-no-deps", "no-deps").
				WithObject(makeTestRB("default", "rb-no-deps")).
				Build(),
			decoder:       &fakeDecoder{decodeObj: makeTestRB("default", "rb-no-deps")},
			clientObjects: nil,
			enableFederatedQuotaEnforcementFeatureGate: false,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "create operation not yet scheduled should be allowed",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, rbUnscheduledCreate.Namespace, rbUnscheduledCreate.Name, "create-unscheduled").
				WithObject(rbUnscheduledCreate).
				Build(),
			decoder:       &fakeDecoder{decodeObj: rbUnscheduledCreate},
			clientObjects: []client.Object{frqForUnscheduledCreate},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "update operation with no relevant field change should be allowed",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, rbForNoChange.Namespace, rbForNoChange.Name, "update-nochange").
				WithObject(rbForNoChange).
				WithOldObject(rbForNoChange).
				Build(),
			decoder:       &fakeDecoder{decodeObj: rbForNoChange, rawDecodedObj: rbForNoChange},
			clientObjects: []client.Object{frqForNoChange},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "total RB delta is zero should be allowed (update with identical states)",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, rbForDeltaZeroNew.Namespace, rbForDeltaZeroNew.Name, "delta-zero").
				WithObject(rbForDeltaZeroNew).
				WithOldObject(rbForDeltaZeroOld).
				Build(),
			decoder:       &fakeDecoder{decodeObj: rbForDeltaZeroNew, rawDecodedObj: rbForDeltaZeroOld},
			clientObjects: []client.Object{frqForDeltaZero},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "no FRQs found should be allowed",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, rbForNoFRQ.Namespace, rbForNoFRQ.Name, "no-frq-found").
				WithObject(rbForNoFRQ).
				Build(),
			decoder:       &fakeDecoder{decodeObj: rbForNoFRQ},
			clientObjects: []client.Object{},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "create quota exceeded should deny",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, rbCreateExceeds.Namespace, rbCreateExceeds.Name, "create-exceeds").
				WithObject(rbCreateExceeds).
				Build(),
			decoder:       &fakeDecoder{decodeObj: rbCreateExceeds},
			clientObjects: []client.Object{frqForCreateExceeds},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Denied("FederatedResourceQuota(quota-ns/frq-create-exceeds) exceeded for resource cpu: requested sum 200m, limit 150m."),
		},
		{
			name: "update passes quota (allowed response, non-dryrun)",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, commonRBUpdatePassNew.Namespace, commonRBUpdatePassNew.Name, "update-pass-nondryrun").
				WithObject(commonRBUpdatePassNew).
				WithOldObject(commonRBUpdatePassOld).
				WithDryRun(false).
				Build(),
			decoder:       &fakeDecoder{decodeObj: commonRBUpdatePassNew, rawDecodedObj: commonRBUpdatePassOld},
			clientObjects: []client.Object{frqForUpdatePassNonDryRun},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "update passes quota (allowed response, dry run)",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, commonRBUpdatePassNew.Namespace, commonRBUpdatePassNew.Name, "update-pass-dryrun").
				WithObject(commonRBUpdatePassNew).
				WithOldObject(commonRBUpdatePassOld).
				WithDryRun(true).
				Build(),
			decoder:       &fakeDecoder{decodeObj: commonRBUpdatePassNew, rawDecodedObj: commonRBUpdatePassOld},
			clientObjects: []client.Object{frqForUpdatePassDryRun},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "update fails quota (denied response)",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, rbUpdateFailNew.Namespace, rbUpdateFailNew.Name, "update-fail").
				WithObject(rbUpdateFailNew).
				WithOldObject(rbUpdateFailOld).
				Build(),
			decoder:       &fakeDecoder{decodeObj: rbUpdateFailNew, rawDecodedObj: rbUpdateFailOld},
			clientObjects: []client.Object{frqForUpdateFail},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Denied("FederatedResourceQuota(quota-ns/frq-update-fail) exceeded for resource cpu: requested sum 110m, limit 100m."),
		},
		{
			name: "validateComponents: zero components (should allow)",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, "comp-ns", "rb-zero-components", "zero-comps").
				WithObject(makeTestRB("comp-ns", "rb-zero-components", WithComponents(nil))).
				Build(),
			decoder:       &fakeDecoder{decodeObj: makeTestRB("comp-ns", "rb-zero-components", WithComponents(nil))},
			clientObjects: nil,
			enableFederatedQuotaEnforcementFeatureGate:      false,
			enableMultiplePodTemplatesSchedulingFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "validateComponents: one component without name (should deny)",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, "comp-ns", "rb-one-component-no-name", "one-comp-no-name").
				WithObject(makeTestRB("comp-ns", "rb-one-component-no-name", WithComponents([]workv1alpha2.Component{{}}))).
				Build(),
			decoder:       &fakeDecoder{decodeObj: makeTestRB("comp-ns", "rb-one-component-no-name", WithComponents([]workv1alpha2.Component{{}}))},
			clientObjects: nil,
			enableFederatedQuotaEnforcementFeatureGate:      false,
			enableMultiplePodTemplatesSchedulingFeatureGate: true,
			wantResponse: admission.Denied("spec.components[0].name: Invalid value: \"\": component names must be non-empty"),
		},
		{
			name: "validateComponents: one component with name (should allow)",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, "comp-ns", "rb-one-component-with-name", "one-comp-with-name").
				WithObject(makeTestRB("comp-ns", "rb-one-component-with-name", WithComponents([]workv1alpha2.Component{
					{Name: "foo"},
				}))).
				Build(),
			decoder:       &fakeDecoder{decodeObj: makeTestRB("comp-ns", "rb-one-component-with-name", WithComponents([]workv1alpha2.Component{{Name: "foo"}}))},
			clientObjects: nil,
			enableFederatedQuotaEnforcementFeatureGate:      false,
			enableMultiplePodTemplatesSchedulingFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "validateComponents: multiple components with unique names (should allow)",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, "comp-ns", "rb-multi-unique", "multi-unique").
				WithObject(makeTestRB("comp-ns", "rb-multi-unique", WithComponents([]workv1alpha2.Component{
					{Name: "foo"}, {Name: "bar"},
				}))).
				Build(),
			decoder:       &fakeDecoder{decodeObj: makeTestRB("comp-ns", "rb-multi-unique", WithComponents([]workv1alpha2.Component{{Name: "foo"}, {Name: "bar"}}))},
			clientObjects: nil,
			enableFederatedQuotaEnforcementFeatureGate:      false,
			enableMultiplePodTemplatesSchedulingFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "validateComponents: multiple components with empty name (should deny)",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, "comp-ns", "rb-multi-empty", "multi-empty").
				WithObject(makeTestRB("comp-ns", "rb-multi-empty", WithComponents([]workv1alpha2.Component{
					{Name: "foo"}, {Name: ""}, {Name: ""},
				}))).
				Build(),
			decoder:       &fakeDecoder{decodeObj: makeTestRB("comp-ns", "rb-multi-empty", WithComponents([]workv1alpha2.Component{{Name: "foo"}, {Name: ""}}))},
			clientObjects: nil,
			enableFederatedQuotaEnforcementFeatureGate:      false,
			enableMultiplePodTemplatesSchedulingFeatureGate: true,
			wantResponse: admission.Denied("spec.components[1].name: Invalid value: \"\": component names must be non-empty"),
		},
		{
			name: "validateComponents: multiple components with duplicate names (should deny)",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, "comp-ns", "rb-multi-dup", "multi-dup").
				WithObject(makeTestRB("comp-ns", "rb-multi-dup", WithComponents([]workv1alpha2.Component{
					{Name: "foo"}, {Name: "foo"},
				}))).
				Build(),
			decoder:       &fakeDecoder{decodeObj: makeTestRB("comp-ns", "rb-multi-dup", WithComponents([]workv1alpha2.Component{{Name: "foo"}, {Name: "foo"}}))},
			clientObjects: nil,
			enableFederatedQuotaEnforcementFeatureGate:      false,
			enableMultiplePodTemplatesSchedulingFeatureGate: true,
			wantResponse: admission.Denied("spec.components[1].name: Invalid value: \"foo\": component names must be unique"),
		},
		{
			name: "create with components and sufficient quota (allowed)",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, rbWithComponents.Namespace, rbWithComponents.Name, "create-components-sufficient").
				WithObject(rbWithComponents).
				Build(),
			decoder:       &fakeDecoder{decodeObj: rbWithComponents},
			clientObjects: []client.Object{frqForComponentsSufficient},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "create with components and exceeded quota (denied)",
			req: newAdmissionRequestBuilder(t, admissionv1.Create, rbWithComponents.Namespace, rbWithComponents.Name, "create-components-exceeded").
				WithObject(rbWithComponents).
				Build(),
			decoder:       &fakeDecoder{decodeObj: rbWithComponents},
			clientObjects: []client.Object{frqForComponentsExceeded},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Denied("FederatedResourceQuota(quota-ns/frq-components-exceeded) exceeded for resource cpu: requested sum 160m, limit 150m."),
		},
		{
			name: "update components passes quota (allowed response)",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, rbComponentsPassNew.Namespace, rbComponentsPassNew.Name, "update-components-pass").
				WithObject(rbComponentsPassNew).
				WithOldObject(rbComponentsPassOld).
				Build(),
			decoder:       &fakeDecoder{decodeObj: rbComponentsPassNew, rawDecodedObj: rbComponentsPassOld},
			clientObjects: []client.Object{frqForComponentsPass},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Allowed(""),
		},
		{
			name: "update components fails quota (denied response)",
			req: newAdmissionRequestBuilder(t, admissionv1.Update, rbComponentsFailNew.Namespace, rbComponentsFailNew.Name, "update-components-fail").
				WithObject(rbComponentsFailNew).
				WithOldObject(rbComponentsFailOld).
				Build(),
			decoder:       &fakeDecoder{decodeObj: rbComponentsFailNew, rawDecodedObj: rbComponentsFailOld},
			clientObjects: []client.Object{frqForComponentsFail},
			enableFederatedQuotaEnforcementFeatureGate: true,
			wantResponse: admission.Denied("FederatedResourceQuota(quota-ns/frq-components-fail) exceeded for resource cpu: requested sum 110m, limit 100m."),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalFeatureGates := features.FeatureGate.DeepCopy()
			if errFG := features.FeatureGate.Set(fmt.Sprintf("%s=%v", features.FederatedQuotaEnforcement, tt.enableFederatedQuotaEnforcementFeatureGate)); errFG != nil {
				t.Fatalf("Failed to set feature gate %s to %v: %v", features.FederatedQuotaEnforcement, tt.enableFederatedQuotaEnforcementFeatureGate, errFG)
			}
			if errFg := features.FeatureGate.Set(fmt.Sprintf("%s=%v", features.MultiplePodTemplatesScheduling, tt.enableMultiplePodTemplatesSchedulingFeatureGate)); errFg != nil {
				t.Fatalf("Failed to set feature gate %s to %v: %v", features.MultiplePodTemplatesScheduling, tt.enableMultiplePodTemplatesSchedulingFeatureGate, errFg)
			}
			t.Cleanup(func() {
				features.FeatureGate = originalFeatureGates
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

func TestIsQuotaRelevantFieldChanged(t *testing.T) {
	tests := []struct {
		name   string
		oldRB  *workv1alpha2.ResourceBinding
		newRB  *workv1alpha2.ResourceBinding
		expect bool
	}{
		{
			name:   "nil old RB should return true",
			oldRB:  nil,
			newRB:  makeTestRB("default", "test"),
			expect: true,
		},
		{
			name:   "nil new RB should return true",
			oldRB:  makeTestRB("default", "test"),
			newRB:  nil,
			expect: true,
		},
		{
			name:   "both nil should return true",
			oldRB:  nil,
			newRB:  nil,
			expect: true,
		},
		{
			name: "no changes should return false",
			oldRB: makeTestRB("default", "test",
				WithClusters([]workv1alpha2.TargetCluster{{Name: "c1", Replicas: 2}}),
				WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
			),
			newRB: makeTestRB("default", "test",
				WithClusters([]workv1alpha2.TargetCluster{{Name: "c1", Replicas: 2}}),
				WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
			),
			expect: false,
		},
		{
			name: "resource request changed should return true",
			oldRB: makeTestRB("default", "test",
				WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
			),
			newRB: makeTestRB("default", "test",
				WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
			),
			expect: true,
		},
		{
			name: "scheduled replicas changed should return true",
			oldRB: makeTestRB("default", "test",
				WithClusters([]workv1alpha2.TargetCluster{{Name: "c1", Replicas: 1}}),
			),
			newRB: makeTestRB("default", "test",
				WithClusters([]workv1alpha2.TargetCluster{{Name: "c1", Replicas: 2}}),
			),
			expect: true,
		},
		{
			name: "components changed should return true",
			oldRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{{Name: "comp1", Replicas: 1}}),
			),
			newRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{{Name: "comp1", Replicas: 2}}),
			),
			expect: true,
		},
		{
			name: "non-quota relevant field changed should return false",
			oldRB: makeTestRB("default", "test",
				WithResourceRef(workv1alpha2.ObjectReference{APIVersion: "v1", Kind: "Pod", Name: "old-pod"}),
			),
			newRB: makeTestRB("default", "test",
				WithResourceRef(workv1alpha2.ObjectReference{APIVersion: "v1", Kind: "Pod", Name: "new-pod"}),
			),
			expect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isQuotaRelevantFieldChanged(tt.oldRB, tt.newRB)
			if got != tt.expect {
				t.Errorf("isQuotaRelevantFieldChanged() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestIsResourceRequestChanged(t *testing.T) {
	tests := []struct {
		name   string
		oldRB  *workv1alpha2.ResourceBinding
		newRB  *workv1alpha2.ResourceBinding
		expect bool
	}{
		{
			name: "no replica requirements in both should return false",
			oldRB: makeTestRB("default", "test",
				WithReplicaRequirements(nil),
			),
			newRB: makeTestRB("default", "test",
				WithReplicaRequirements(nil),
			),
			expect: false,
		},
		{
			name: "old has no replica requirements, new has some should return true",
			oldRB: makeTestRB("default", "test",
				WithReplicaRequirements(nil),
			),
			newRB: makeTestRB("default", "test",
				WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
			),
			expect: true,
		},
		{
			name: "old has replica requirements, new has none should return true",
			oldRB: makeTestRB("default", "test",
				WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
			),
			newRB: makeTestRB("default", "test",
				WithReplicaRequirements(nil),
			),
			expect: true,
		},
		{
			name: "same resource requests should return false",
			oldRB: makeTestRB("default", "test",
				WithReplicaRequirements(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				}),
			),
			newRB: makeTestRB("default", "test",
				WithReplicaRequirements(corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				}),
			),
			expect: false,
		},
		{
			name: "different resource requests should return true",
			oldRB: makeTestRB("default", "test",
				WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
			),
			newRB: makeTestRB("default", "test",
				WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")}),
			),
			expect: true,
		},
		{
			name: "different resource types should return true",
			oldRB: makeTestRB("default", "test",
				WithReplicaRequirements(corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m")}),
			),
			newRB: makeTestRB("default", "test",
				WithReplicaRequirements(corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("128Mi")}),
			),
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isResourceRequestChanged(tt.oldRB, tt.newRB)
			if got != tt.expect {
				t.Errorf("isResourceRequestChanged() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestIsScheduledReplicasChanged(t *testing.T) {
	tests := []struct {
		name   string
		oldRB  *workv1alpha2.ResourceBinding
		newRB  *workv1alpha2.ResourceBinding
		expect bool
	}{
		{
			name: "no clusters in both should return false",
			oldRB: makeTestRB("default", "test",
				WithClusters(nil),
			),
			newRB: makeTestRB("default", "test",
				WithClusters(nil),
			),
			expect: false,
		},
		{
			name: "same total replicas should return false",
			oldRB: makeTestRB("default", "test",
				WithClusters([]workv1alpha2.TargetCluster{
					{Name: "c1", Replicas: 2},
					{Name: "c2", Replicas: 3},
				}),
			),
			newRB: makeTestRB("default", "test",
				WithClusters([]workv1alpha2.TargetCluster{
					{Name: "c1", Replicas: 1},
					{Name: "c2", Replicas: 4},
				}),
			),
			expect: false,
		},
		{
			name: "different total replicas should return true",
			oldRB: makeTestRB("default", "test",
				WithClusters([]workv1alpha2.TargetCluster{
					{Name: "c1", Replicas: 2},
				}),
			),
			newRB: makeTestRB("default", "test",
				WithClusters([]workv1alpha2.TargetCluster{
					{Name: "c1", Replicas: 3},
				}),
			),
			expect: true,
		},
		{
			name: "old has no clusters, new has clusters should return true",
			oldRB: makeTestRB("default", "test",
				WithClusters(nil),
			),
			newRB: makeTestRB("default", "test",
				WithClusters([]workv1alpha2.TargetCluster{{Name: "c1", Replicas: 1}}),
			),
			expect: true,
		},
		{
			name: "old has clusters, new has no clusters should return true",
			oldRB: makeTestRB("default", "test",
				WithClusters([]workv1alpha2.TargetCluster{{Name: "c1", Replicas: 1}}),
			),
			newRB: makeTestRB("default", "test",
				WithClusters(nil),
			),
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isScheduledReplicasChanged(tt.oldRB, tt.newRB)
			if got != tt.expect {
				t.Errorf("isScheduledReplicasChanged() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestIsComponentsChanged(t *testing.T) {
	makeComponent := func(name string, replicas int32, cpu string) workv1alpha2.Component {
		comp := workv1alpha2.Component{
			Name:     name,
			Replicas: replicas,
		}
		if cpu != "" {
			comp.ReplicaRequirements = &workv1alpha2.ComponentReplicaRequirements{
				ResourceRequest: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu)},
			}
		}
		return comp
	}

	tests := []struct {
		name   string
		oldRB  *workv1alpha2.ResourceBinding
		newRB  *workv1alpha2.ResourceBinding
		expect bool
	}{
		{
			name: "no components in both should return false",
			oldRB: makeTestRB("default", "test",
				WithComponents(nil),
			),
			newRB: makeTestRB("default", "test",
				WithComponents(nil),
			),
			expect: false,
		},
		{
			name: "same components should return false",
			oldRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 2, "100m"),
					makeComponent("comp2", 1, "50m"),
				}),
			),
			newRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 2, "100m"),
					makeComponent("comp2", 1, "50m"),
				}),
			),
			expect: false,
		},
		{
			name: "different number of components should return true",
			oldRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 1, "100m"),
				}),
			),
			newRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 1, "100m"),
					makeComponent("comp2", 1, "50m"),
				}),
			),
			expect: true,
		},
		{
			name: "different replicas should return true",
			oldRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 1, "100m"),
				}),
			),
			newRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 2, "100m"),
				}),
			),
			expect: true,
		},
		{
			name: "old has replica requirements, new doesn't should return true",
			oldRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 1, "100m"),
				}),
			),
			newRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 1, ""),
				}),
			),
			expect: true,
		},
		{
			name: "old doesn't have replica requirements, new does should return true",
			oldRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 1, ""),
				}),
			),
			newRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 1, "100m"),
				}),
			),
			expect: true,
		},
		{
			name: "different resource requests should return true",
			oldRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 1, "100m"),
				}),
			),
			newRB: makeTestRB("default", "test",
				WithComponents([]workv1alpha2.Component{
					makeComponent("comp1", 1, "200m"),
				}),
			),
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isComponentsChanged(tt.oldRB, tt.newRB)
			if got != tt.expect {
				t.Errorf("isComponentsChanged() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestAreResourceListsEqual(t *testing.T) {
	tests := []struct {
		name   string
		a      corev1.ResourceList
		b      corev1.ResourceList
		expect bool
	}{
		{
			name:   "both nil should return true",
			a:      nil,
			b:      nil,
			expect: true,
		},
		{
			name:   "both empty should return true",
			a:      corev1.ResourceList{},
			b:      corev1.ResourceList{},
			expect: true,
		},
		{
			name: "same resources should return true",
			a: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			b: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			expect: true,
		},
		{
			name: "different lengths should return false",
			a: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
			b: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			expect: false,
		},
		{
			name: "different keys should return false",
			a: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
			b: corev1.ResourceList{
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			expect: false,
		},
		{
			name: "different values should return false",
			a: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
			b: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("200m"),
			},
			expect: false,
		},
		{
			name:   "one nil, one empty should return true",
			a:      nil,
			b:      corev1.ResourceList{},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := areResourceListsEqual(tt.a, tt.b)
			if got != tt.expect {
				t.Errorf("areResourceListsEqual() = %v, want %v", got, tt.expect)
			}
		})
	}
}
