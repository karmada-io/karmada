/*
Copyright 2022 The Karmada Authors.

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

package storage

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	genericrequest "k8s.io/apiserver/pkg/endpoints/request"
)

var (
	apiPrefixes          = sets.NewString("api", "apis")
	groupLessAPIPrefixes = sets.NewString("api")
)

func parseK8sNativeResourceInfo(reqParts []string) (*genericrequest.RequestInfo, error) {
	requestInfo := &genericrequest.RequestInfo{
		IsResourceRequest: false,
		Path:              strings.Join(reqParts, "/"),
	}

	if len(reqParts) < 3 {
		// return a non-resource request
		return requestInfo, nil
	}

	if !apiPrefixes.Has(reqParts[0]) {
		// return a non-resource request
		return requestInfo, nil
	}

	requestInfo.APIPrefix = reqParts[0]
	currentParts := reqParts[1:]

	if !groupLessAPIPrefixes.Has(requestInfo.APIPrefix) {
		if len(currentParts) < 3 {
			// return a non-resource request
			return requestInfo, nil
		}

		requestInfo.APIGroup = currentParts[0]
		currentParts = currentParts[1:]
	}

	requestInfo.IsResourceRequest = true
	requestInfo.APIVersion = currentParts[0]
	currentParts = currentParts[1:]

	// URL forms: /namespaces/{namespace}/{kind}/*, where parts are adjusted to be relative to kind
	if currentParts[0] == "namespaces" {
		if len(currentParts) > 1 {
			requestInfo.Namespace = currentParts[1]
			if len(currentParts) > 2 {
				currentParts = currentParts[2:]
			}
		}
	} else {
		requestInfo.Namespace = metav1.NamespaceNone
	}

	switch {
	case len(currentParts) >= 3:
		return nil, fmt.Errorf("invalid request parts(%s) for k8s native request URL", currentParts)
	case len(currentParts) >= 2:
		requestInfo.Name = currentParts[1]
		fallthrough
	case len(currentParts) >= 1:
		requestInfo.Resource = currentParts[0]
	}

	if requestInfo.Resource == "namespaces" {
		requestInfo.Namespace = ""
	}
	return requestInfo, nil
}
