package explorer

import (
	"net/http"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// Errored creates a new Response for error-handling a request.
func Errored(code int32, err error) Response {
	return Response{
		ExploreResponse: configv1alpha1.ExploreResponse{
			Successful: false,
			Status: &configv1alpha1.RequestStatus{
				Code:    code,
				Message: err.Error(),
			},
		},
	}
}

// Succeeded constructs a response indicating the given operation is handled successfully.
func Succeeded(msg string) Response {
	return ValidationResponse(true, msg)
}

// ValidationResponse returns a response for handle a explore request.
func ValidationResponse(successful bool, msg string) Response {
	code := http.StatusForbidden
	if successful {
		code = http.StatusOK
	}
	return Response{
		ExploreResponse: configv1alpha1.ExploreResponse{
			Successful: successful,
			Status: &configv1alpha1.RequestStatus{
				Code:    int32(code),
				Message: msg,
			},
		},
	}
}
