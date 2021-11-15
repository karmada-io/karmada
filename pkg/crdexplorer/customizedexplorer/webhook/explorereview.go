package webhook

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// CreateExploreReview returns the unique request uid, the ExploreReview object to send the webhook,
// or an error if the webhook does not support receiving any of the versions we know to send.
func CreateExploreReview(versions []string, attributes *RequestAttributes) (uid types.UID, request runtime.Object, err error) {
	for _, version := range versions {
		switch version {
		case configv1alpha1.GroupVersion.Version:
			uid = uuid.NewUUID()
			request = CreateV1alpha1ExploreReview(uid, attributes)
			return
		}
	}

	err = fmt.Errorf("webhook does not accept known ExploreReview versions (v1alpha1)")
	return
}

// CreateV1alpha1ExploreReview creates an ExploreReview for the provided RequestAttributes.
func CreateV1alpha1ExploreReview(uid types.UID, attributes *RequestAttributes) *configv1alpha1.ExploreReview {
	return &configv1alpha1.ExploreReview{
		Request: &configv1alpha1.ExploreRequest{
			UID: uid,
			Kind: metav1.GroupVersionKind{
				Group:   attributes.Object.GroupVersionKind().Group,
				Version: attributes.Object.GroupVersionKind().Version,
				Kind:    attributes.Object.GetKind(),
			},
			Name:      attributes.Object.GetName(),
			Namespace: attributes.Object.GetNamespace(),
			Operation: attributes.Operation,
			Object: runtime.RawExtension{
				Object: attributes.Object.DeepCopyObject(),
			},
			DesiredReplicas:  &attributes.ReplicasSet,
			AggregatedStatus: attributes.AggregatedStatus,
		},
	}
}

// VerifyExploreReview checks the validity of the provided explore review object, and returns ResponseAttributes,
// or an error if the provided admission review was not valid.
func VerifyExploreReview(uid types.UID, operation configv1alpha1.OperationType, review runtime.Object) (response *ResponseAttributes, err error) {
	switch r := review.(type) {
	case *configv1alpha1.ExploreReview:
		if r.Response == nil {
			return nil, fmt.Errorf("webhook resonse was absent")
		}

		if r.Response.UID != uid {
			return nil, fmt.Errorf("expected response.uid %q, got %q", uid, r.Response.UID)
		}

		v1alpha1GVK := configv1alpha1.SchemeGroupVersion.WithKind("ExploreReview")
		if r.GroupVersionKind() != v1alpha1GVK {
			return nil, fmt.Errorf("expected webhook response of %v, got %v", v1alpha1GVK.String(), r.GroupVersionKind().String())
		}

		if err := verifyExploreResponse(operation, r.Response); err != nil {
			return nil, err
		}
		return extractResponseFromV1alpha1ExploreReview(r.Response), nil
	default:
		return nil, fmt.Errorf("unexpected response type %T", review)
	}
}

func verifyExploreResponse(operation configv1alpha1.OperationType, response *configv1alpha1.ExploreResponse) error {
	switch operation {
	case configv1alpha1.ExploreReplica:
		if response.Replicas == nil {
			return fmt.Errorf("webhook returned nil response.replicas")
		}
		return nil
	case configv1alpha1.ExploreDependencies:
		return nil
	case configv1alpha1.ExplorePacking, configv1alpha1.ExploreReplicaRevising,
		configv1alpha1.ExploreRetaining, configv1alpha1.ExploreStatusAggregating:
		return verifyExploreResponseWithPatch(response)
	case configv1alpha1.ExploreStatus:
		return nil
	case configv1alpha1.ExploreHealthy:
		if response.Healthy == nil {
			return fmt.Errorf("webhook returned nil response.healthy")
		}
		return nil
	default:
		return fmt.Errorf("input wrong operation type: %s", operation)
	}
}

func verifyExploreResponseWithPatch(response *configv1alpha1.ExploreResponse) error {
	if len(response.Patch) == 0 && response.PatchType == nil {
		return fmt.Errorf("webhook returned empty response.patch and nil response.patchType")
	}
	if response.PatchType == nil {
		return fmt.Errorf("webhook returned nil response.patchType")
	}
	if len(response.Patch) == 0 {
		return fmt.Errorf("webhook returned empty response.patch")
	}
	patchType := *response.PatchType
	if patchType != configv1alpha1.PatchTypeJSONPatch {
		return fmt.Errorf("webhook returned invalid response.patchType of %q", patchType)
	}
	return nil
}

func extractResponseFromV1alpha1ExploreReview(response *configv1alpha1.ExploreResponse) *ResponseAttributes {
	return &ResponseAttributes{
		Replicas:            *response.Replicas,
		ReplicaRequirements: response.ReplicaRequirements,
		Dependencies:        response.Dependencies,
		Patch:               response.Patch,
		PatchType:           *response.PatchType,
		RawStatus:           *response.RawStatus,
		Healthy:             *response.Healthy,
	}
}
