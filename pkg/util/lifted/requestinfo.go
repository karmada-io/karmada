/*
Copyright 2016 The Kubernetes Authors.
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

// This code is directly lifted from the Kubernetes codebase in order to avoid relying on the k8s.io/kubernetes package.
// For reference:
// https://github.com/kubernetes/apiserver/blob/release-1.23/pkg/endpoints/request/requestinfo.go

package lifted

import (
	"net/http"
	"strings"

	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
)

var apiPrefixes = sets.NewString("apis", "api")
var grouplessAPIPrefixes = sets.NewString("api")

// +lifted:source=https://github.com/kubernetes/apiserver/blob/release-1.23/pkg/endpoints/request/requestinfo.go#L88-L247
// +lifted:changed

// TODO write an integration test against the swagger doc to test the RequestInfo and match up behavior to responses

// NewRequestInfo returns the information from the http request.  If error is not nil, RequestInfo holds the information as best it is known before the failure
// It handles both resource and non-resource requests and fills in all the pertinent information for each.
// Valid Inputs:
// Resource paths
// /apis/{api-group}/{version}/namespaces
// /api/{version}/namespaces
// /api/{version}/namespaces/{namespace}
// /api/{version}/namespaces/{namespace}/{resource}
// /api/{version}/namespaces/{namespace}/{resource}/{resourceName}
// /api/{version}/{resource}
// /api/{version}/{resource}/{resourceName}
//
// Special verbs without subresources:
// /api/{version}/proxy/{resource}/{resourceName}
// /api/{version}/proxy/namespaces/{namespace}/{resource}/{resourceName}
//
// Special verbs with subresources:
// /api/{version}/watch/{resource}
// /api/{version}/watch/namespaces/{namespace}/{resource}
//
// NonResource paths
// /apis/{api-group}/{version}
// /apis/{api-group}
// /apis
// /api/{version}
// /api
// /healthz
// /
//nolint:gocyclo
func NewRequestInfo(req *http.Request) *apirequest.RequestInfo {
	// start with a non-resource request until proven otherwise
	requestInfo := apirequest.RequestInfo{
		IsResourceRequest: false,
		Verb:              strings.ToLower(req.Method),
	}

	currentParts := SplitPath(req.URL.Path)
	if len(currentParts) < 3 {
		// return a non-resource request
		return &requestInfo
	}

	if !apiPrefixes.Has(currentParts[0]) {
		// return a non-resource request
		return &requestInfo
	}
	requestInfo.APIPrefix = currentParts[0]
	currentParts = currentParts[1:]

	if !grouplessAPIPrefixes.Has(requestInfo.APIPrefix) {
		// one part (APIPrefix) has already been consumed, so this is actually "do we have four parts?"
		if len(currentParts) < 3 {
			// return a non-resource request
			return &requestInfo
		}

		requestInfo.APIGroup = currentParts[0]
		currentParts = currentParts[1:]
	}

	requestInfo.IsResourceRequest = true
	requestInfo.APIVersion = currentParts[0]
	currentParts = currentParts[1:]

	// handle input of form /{specialVerb}/*
	if specialVerbs.Has(currentParts[0]) {
		if len(currentParts) < 2 {
			return &requestInfo
		}

		requestInfo.Verb = currentParts[0]
		currentParts = currentParts[1:]
	} else {
		switch req.Method {
		case "POST":
			requestInfo.Verb = "create"
		case "GET", "HEAD":
			requestInfo.Verb = "get"
		case "PUT":
			requestInfo.Verb = "update"
		case "PATCH":
			requestInfo.Verb = "patch"
		case "DELETE":
			requestInfo.Verb = "delete"
		default:
			requestInfo.Verb = ""
		}
	}

	// URL forms: /namespaces/{namespace}/{kind}/*, where parts are adjusted to be relative to kind
	if currentParts[0] == "namespaces" {
		if len(currentParts) > 1 {
			requestInfo.Namespace = currentParts[1]

			// if there is another step after the namespace name, and it is not a known namespace subresource
			// move currentParts to include it as a resource in its own right
			if len(currentParts) > 2 && !namespaceSubresources.Has(currentParts[2]) {
				currentParts = currentParts[2:]
			}
		}
	} else {
		requestInfo.Namespace = metav1.NamespaceNone
	}

	// parsing successful, so we now know the proper value for .Parts
	requestInfo.Parts = currentParts

	// parts look like: resource/resourceName/subresource/other/stuff/we/don't/interpret
	switch {
	case len(requestInfo.Parts) >= 3 && !specialVerbsNoSubresources.Has(requestInfo.Verb):
		requestInfo.Subresource = requestInfo.Parts[2]
		fallthrough
	case len(requestInfo.Parts) >= 2:
		requestInfo.Name = requestInfo.Parts[1]
		fallthrough
	case len(requestInfo.Parts) >= 1:
		requestInfo.Resource = requestInfo.Parts[0]
	}

	// if there's no name on the request and we thought it was a get before, then the actual verb is a list or a watch
	if len(requestInfo.Name) == 0 && requestInfo.Verb == "get" {
		opts := metainternalversion.ListOptions{}
		if values := req.URL.Query()["watch"]; len(values) > 0 {
			switch strings.ToLower(values[0]) {
			case "false", "0":
			default:
				opts.Watch = true
			}
		}

		if opts.Watch {
			requestInfo.Verb = "watch"
		} else {
			requestInfo.Verb = "list"
		}
	}
	// if there's no name on the request and we thought it was a delete before, then the actual verb is deletecollection
	if len(requestInfo.Name) == 0 && requestInfo.Verb == "delete" {
		requestInfo.Verb = "deletecollection"
	}

	return &requestInfo
}

// +lifted:source=https://github.com/kubernetes/apiserver/blob/release-1.23/pkg/endpoints/request/requestinfo.go#L267-L274
// +lifted:changed

// SplitPath returns the segments for a URL path.
func SplitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return []string{}
	}
	return strings.Split(path, "/")
}

// +lifted:source=https://github.com/kubernetes/apiserver/blob/release-1.23/pkg/endpoints/request/requestinfo.go#L73-L74

// specialVerbsNoSubresources contains root verbs which do not allow subresources
var specialVerbsNoSubresources = sets.NewString("proxy")

// +lifted:source=https://github.com/kubernetes/apiserver/blob/release-1.23/pkg/endpoints/request/requestinfo.go#L76-L78

// namespaceSubresources contains subresources of namespace
// this list allows the parser to distinguish between a namespace subresource, and a namespaced resource
var namespaceSubresources = sets.NewString("status", "finalize")

// +lifted:source=https://github.com/kubernetes/apiserver/blob/release-1.23/pkg/endpoints/request/requestinfo.go#L67-L71

// specialVerbs contains just strings which are used in REST paths for special actions that don't fall under the normal
// CRUDdy GET/POST/PUT/DELETE actions on REST objects.
// TODO: find a way to keep this up to date automatically.  Maybe dynamically populate list as handlers added to
// master's Mux.
var specialVerbs = sets.NewString("proxy", "watch")
