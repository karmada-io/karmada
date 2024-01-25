/*
Copyright 2021 The Karmada Authors.

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

package clusterrestriction

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	// OwnerAnnotationKey is the annotation key to declare resource ownership by karmada-agent
	OwnerAnnotationKey = "clusterrestriction.karmada.io/owner"
)

var (
	rfc6901Encoder      = strings.NewReplacer("~", "~0", "/", "~1")
	ownerAnnotationPath = fmt.Sprintf("/metadata/annotations/%s", rfc6901Encoder.Replace(OwnerAnnotationKey))

	denyToOperateOwnerAnnotation = admission.Denied(
		"karmada-agent can not add/update/remove owner annotation on resources",
	)
	allowForBackwardCompatibility = admission.Allowed(
		"for backward compatibility although ownership is not defined.",
	)

	cantOperateOwnedByAnother = func(operation admissionv1.Operation) admission.Response {
		return admission.Denied(fmt.Sprintf("karmada-agent can not %s resources owned by another karmada-agent", operation))
	}
)

// MutatingAdmission mutates API request if necessary.
type MutatingAdmission struct {
	Decoder *admission.Decoder
}

var _ admission.Handler = &MutatingAdmission{}

// Handle yields a response to an AdmissionRequest.
//
// The supplied context is extracted from the received http.Request, allowing wrapping
// http.Handlers to inject values into and control cancelation of downstream request processing.
func (a *MutatingAdmission) Handle(_ context.Context, req admission.Request) admission.Response {
	agentOperating, isAgent := AgentIdentity(req.UserInfo)

	// Focusing on requests from karmada-agent
	if !isAgent {
		return admission.Allowed("")
	}

	object := unstructured.Unstructured{}
	if err := a.Decoder.Decode(req, &object); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	switch req.Operation {
	case admissionv1.Create:
		return a.admitCreate(&object, agentOperating)

	case admissionv1.Update, admissionv1.Delete:
		oldObject := unstructured.Unstructured{}
		if err := a.Decoder.Decode(req, &oldObject); err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		return a.admitUpdateOrDelete(&object, &oldObject, agentOperating, req.Operation)

	case admissionv1.Connect:
		return a.admitConnect(&object, agentOperating)

	default:
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("operation=%s is not supported", req.Operation))
	}
}

func (a *MutatingAdmission) admitCreate(object metav1.Object, agentOperating string) admission.Response {
	_, ownerExistsOnNew := getOwnershipAnnotation(object)
	if ownerExistsOnNew {
		return denyToOperateOwnerAnnotation
	}
	return ownerAnnotationPatchedResponse(agentOperating, object)
}

func (a *MutatingAdmission) admitUpdateOrDelete(newObject, oldObject metav1.Object, agentOperating string, operation admissionv1.Operation) admission.Response {
	ownerOnNew, ownerExistsOnNew := getOwnershipAnnotation(newObject)
	ownerOnOld, ownerExistsOnOld := getOwnershipAnnotation(oldObject)

	switch {
	case ownerExistsOnNew && ownerExistsOnOld:
		// try to operate others should deny
		if ownerOnOld != agentOperating {
			return cantOperateOwnedByAnother(operation)
		}
		// giving away the ownership should deny
		if ownerOnNew != agentOperating {
			return denyToOperateOwnerAnnotation
		}
		// ownerOnNew == ownerOnOld == agentOperating
		return admission.Allowed("")

	case ownerExistsOnNew && !ownerExistsOnOld:
		// adding owner annotation should deny
		return denyToOperateOwnerAnnotation

	case !ownerExistsOnNew && ownerExistsOnOld:
		// removing owner annotation should deny
		return denyToOperateOwnerAnnotation

	case !ownerExistsOnNew && !ownerExistsOnOld:
		// without owner annotation should allow for backward compatibility
		return allowForBackwardCompatibility

	default:
		return admission.Errored(http.StatusInternalServerError, errors.New("it must not happen"))
	}
}

func (a *MutatingAdmission) admitConnect(object metav1.Object, agentOperating string) admission.Response {
	owner, ownerExists := getOwnershipAnnotation(object)
	if !ownerExists {
		return allowForBackwardCompatibility
	}

	if owner != agentOperating {
		return cantOperateOwnedByAnother(admissionv1.Connect)
	}
	return admission.Allowed("")
}

func ownerAnnotationPatchedResponse(owner string, object metav1.Object) admission.Response {
	patches := createOwnerAnnotationPatch(owner, object)

	return admission.Patched(
		fmt.Sprintf("Patched annotation key/value: %s=%s", OwnerAnnotationKey, owner),
		patches...,
	)
}

func createOwnerAnnotationPatch(owner string, object metav1.Object) []jsonpatch.Operation {
	patches := []jsonpatch.JsonPatchOperation{}

	if object.GetAnnotations() == nil {
		patches = append(patches, jsonpatch.NewOperation(
			"add",
			"/metadata/annotations",
			map[string]string{},
		))
	}

	if _, exists := getOwnershipAnnotation(object); exists {
		patches = append(patches, jsonpatch.NewOperation(
			"replace",
			ownerAnnotationPath,
			owner,
		))
	} else {
		patches = append(patches, jsonpatch.NewOperation(
			"add",
			ownerAnnotationPath,
			owner,
		))
	}
	return patches
}

func getOwnershipAnnotation(object metav1.Object) (owner string, exists bool) {
	owner, exists = object.GetAnnotations()[OwnerAnnotationKey]
	return
}
