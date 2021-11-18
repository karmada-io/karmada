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
	review := &configv1alpha1.ExploreReview{
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

	if attributes.ObservedObj != nil {
		review.Request.ObservedObject = &runtime.RawExtension{Object: attributes.ObservedObj.DeepCopyObject()}
	}
	return review
}

// VerifyExploreReview checks the validity of the provided explore review object, and returns ResponseAttributes,
// or an error if the provided explore review was not valid.
func VerifyExploreReview(uid types.UID, operation configv1alpha1.OperationType, review runtime.Object) (response *ResponseAttributes, err error) {
	switch r := review.(type) {
	case *configv1alpha1.ExploreReview:
		if r.Response == nil {
			return nil, fmt.Errorf("webhook resonse was absent")
		}

		if r.Response.UID != uid {
			return nil, fmt.Errorf("expected response.uid %q, got %q", uid, r.Response.UID)
		}

		res, err := verifyExploreResponse(operation, r.Response)
		if err != nil {
			return nil, err
		}
		return res, nil
	default:
		return nil, fmt.Errorf("unexpected response type %T", review)
	}
}

func verifyExploreResponse(operation configv1alpha1.OperationType, response *configv1alpha1.ExploreResponse) (*ResponseAttributes, error) {
	res := &ResponseAttributes{}

	res.Successful = response.Successful
	if !response.Successful {
		if response.Status == nil {
			return nil, fmt.Errorf("webhook require response.status when response.successful is false")
		}
		res.Status = *response.Status
		return res, nil
	}

	switch operation {
	case configv1alpha1.ExploreReplica:
		if response.Replicas == nil {
			return nil, fmt.Errorf("webhook returned nil response.replicas")
		}
		res.Replicas = *response.Replicas
		res.ReplicaRequirements = response.ReplicaRequirements
		return res, nil
	case configv1alpha1.ExploreDependencies:
		res.Dependencies = response.Dependencies
		return res, nil
	case configv1alpha1.ExplorePacking, configv1alpha1.ExploreReplicaRevising,
		configv1alpha1.ExploreRetaining, configv1alpha1.ExploreStatusAggregating:
		err := verifyExploreResponseWithPatch(response)
		if err != nil {
			return nil, err
		}
		res.Patch = response.Patch
		res.PatchType = *response.PatchType
		return res, nil
	case configv1alpha1.ExploreStatus:
		res.RawStatus = *response.RawStatus
		return res, nil
	case configv1alpha1.ExploreHealthy:
		if response.Healthy == nil {
			return nil, fmt.Errorf("webhook returned nil response.healthy")
		}
		res.Healthy = *response.Healthy
		return res, nil
	default:
		return nil, fmt.Errorf("input wrong operation type: %s", operation)
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
