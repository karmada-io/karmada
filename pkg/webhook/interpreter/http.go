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

package interpreter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog/v2"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

const (
	// MaxRespBodyLength is the max length of http response body
	MaxRespBodyLength = 1 << 20 // 1 MiB
)

var admissionScheme = runtime.NewScheme()
var admissionCodecs = serializer.NewCodecFactory(admissionScheme)

// ServeHTTP write reply headers and data to the ResponseWriter and then return.
func (wh *Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var body []byte
	var err error
	ctx := r.Context()

	var res Response
	if r.Body == nil {
		err = errors.New("request body is empty")
		klog.Errorf("bad request: %v", err)
		res = Errored(http.StatusBadRequest, err)
		wh.writeResponse(w, res)
		return
	}

	defer r.Body.Close()
	if body, err = io.ReadAll(io.LimitReader(r.Body, MaxRespBodyLength)); err != nil {
		klog.Errorf("unable to read the body from the incoming request: %v", err)
		res = Errored(http.StatusBadRequest, err)
		wh.writeResponse(w, res)
		return
	}

	// verify the content type is accurate
	if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
		err = fmt.Errorf("contentType=%s, expected application/json", contentType)
		klog.Errorf("unable to process a request with an unknown content type: %v", err)
		res = Errored(http.StatusBadRequest, err)
		wh.writeResponse(w, res)
		return
	}

	request := Request{}
	interpreterContext := configv1alpha1.ResourceInterpreterContext{}
	// avoid an extra copy
	interpreterContext.Request = &request.ResourceInterpreterRequest
	_, _, err = admissionCodecs.UniversalDeserializer().Decode(body, nil, &interpreterContext)
	if err != nil {
		klog.Errorf("unable to decode the request: %v", err)
		res = Errored(http.StatusBadRequest, err)
		wh.writeResponse(w, res)
		return
	}
	klog.V(1).Infof("Received request UID: %q, kind: %s", request.UID, request.Kind)

	res = wh.Handle(ctx, request)
	wh.writeResponse(w, res)
}

// writeResponse writes response to w generically, i.e. without encoding GVK information.
func (wh *Webhook) writeResponse(w io.Writer, response Response) {
	wh.writeResourceInterpreterResponse(w, configv1alpha1.ResourceInterpreterContext{
		Response: &response.ResourceInterpreterResponse,
	})
}

// writeResourceInterpreterResponse writes ar to w.
func (wh *Webhook) writeResourceInterpreterResponse(w io.Writer, interpreterContext configv1alpha1.ResourceInterpreterContext) {
	if err := json.NewEncoder(w).Encode(interpreterContext); err != nil {
		klog.Errorf("unable to encode the response: %v", err)
		wh.writeResponse(w, Errored(http.StatusInternalServerError, err))
	} else {
		response := interpreterContext.Response
		if response.Successful {
			klog.V(4).Infof("Wrote response UID: %q, successful: %t", response.UID, response.Successful)
		} else {
			klog.V(4).Infof("Wrote response UID: %q, successful: %t, response.status.code: %d, response.status.message: %s",
				response.UID, response.Successful, response.Status.Code, response.Status.Message)
		}
	}
}
