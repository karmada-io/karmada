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

package authorization_webhook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	authorizationv1 "k8s.io/api/authorization/v1"
	authorizationv1beta1 "k8s.io/api/authorization/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var authorizationScheme = runtime.NewScheme()
var authorizationCodecs = serializer.NewCodecFactory(authorizationScheme)

func init() {
	utilruntime.Must(authorizationv1.AddToScheme(authorizationScheme))
	utilruntime.Must(authorizationv1beta1.AddToScheme(authorizationScheme))
}

var _ http.Handler = &Webhook{}

func (wh *Webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if wh.WithContextFunc != nil {
		ctx = wh.WithContextFunc(ctx, r)
	}

	if r.Body == nil || r.Body == http.NoBody {
		err := errors.New("request body is empty")
		wh.getLogger(nil).Error(err, "bad request")
		wh.writeResponse(w, Errored(err))
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		wh.getLogger(nil).Error(err, "unable to read the body from the incoming request")
		wh.writeResponse(w, Errored(err))
		return
	}

	// verify the content type is accurate
	if contentType := r.Header.Get("Content-Type"); contentType != "application/json" {
		err = fmt.Errorf("contentType=%s, expected application/json", contentType)
		wh.getLogger(nil).Error(err, "unable to process a request with unknown content type")
		wh.writeResponse(w, Errored(err))
		return
	}

	// Decode request body into authorizationv1.SubjectAccessReviewSpec structure
	sar, actualTokRevGVK, err := wh.decodeRequestBody(body)
	if err != nil {
		wh.getLogger(nil).Error(err, "unable to decode the request")
		wh.writeResponse(w, Errored(err))
		return
	}
	req := Request{}
	req.SubjectAccessReview = sar.SubjectAccessReview
	wh.getLogger(&req).V(5).Info("received request")

	wh.writeResponseTyped(w, wh.Handle(ctx, req), actualTokRevGVK)
}

// writeResponse writes response to w generically, i.e. without encoding GVK information.
func (wh *Webhook) writeResponse(w io.Writer, response Response) {
	wh.writeSubjectAccessReviewResponse(w, response.SubjectAccessReview)
}

// writeResponseTyped writes response to w with GVK set to subjRevGVK, which is necessary
// if multiple SubjectAccessReview versions are permitted by the webhook.
func (wh *Webhook) writeResponseTyped(w io.Writer, response Response, subjRevGVK *schema.GroupVersionKind) {
	ar := response.SubjectAccessReview

	// Default to a v1 SubjectAccessReview, otherwise the API server may not recognize the request
	// if multiple SubjectAccessReview versions are permitted by the webhook config.
	if subjRevGVK == nil || *subjRevGVK == (schema.GroupVersionKind{}) {
		ar.SetGroupVersionKind(authorizationv1.SchemeGroupVersion.WithKind("SubjectAccessReview"))
	} else {
		ar.SetGroupVersionKind(*subjRevGVK)
	}
	wh.writeSubjectAccessReviewResponse(w, ar)
}

// writeSubjectAccessReviewResponse writes ar to w.
func (wh *Webhook) writeSubjectAccessReviewResponse(w io.Writer, ar authorizationv1.SubjectAccessReview) {
	if err := json.NewEncoder(w).Encode(ar); err != nil {
		wh.getLogger(nil).Error(err, "unable to encode the response")
		wh.writeResponse(w, Errored(err))
	}
	res := ar
	wh.getLogger(nil).V(5).Info("wrote response", "authorized", res.Status.Allowed)
}

func (wh *Webhook) decodeRequestBody(body []byte) (unversionedSubjectAccessReview, *schema.GroupVersionKind, error) {
	// v1 and v1beta1 SubjectAccessReview types are almost exactly the same (the only difference is the JSON key for the
	// 'Groups' field).The v1beta1 api is deprecated as of 1.19 and will be removed in authorization as of v1.22. We
	// decode the object into a v1 type and "manually" convert the 'Groups' field (see below).
	// However, the runtime codec's decoder guesses which type to decode into by type name if an Object's TypeMeta
	// isn't set. By setting TypeMeta of an unregistered type to the v1 GVK, the decoder will coerce a v1beta1
	// SubjectAccessReview to v1.
	var obj unversionedSubjectAccessReview
	obj.SetGroupVersionKind(authorizationv1.SchemeGroupVersion.WithKind("SubjectAccessReview"))

	_, gvk, err := authorizationCodecs.UniversalDeserializer().Decode(body, nil, &obj)
	if err != nil {
		return obj, nil, err
	}
	if gvk == nil {
		return obj, nil, fmt.Errorf("could not determine GVK for object in the request body")
	}

	// The only difference in v1beta1 is that the JSON key name of the 'Groups' field is different. Hence, when we
	// detect that v1beta1 was sent, we decode it once again into the "correct" type and manually "convert" the 'Groups'
	// information.
	switch *gvk {
	case authorizationv1beta1.SchemeGroupVersion.WithKind("SubjectAccessReview"):
		var tmp authorizationv1beta1.SubjectAccessReview
		if _, _, err := authorizationCodecs.UniversalDeserializer().Decode(body, nil, &tmp); err != nil {
			return obj, gvk, err
		}
		obj.Spec.Groups = tmp.Spec.Groups
	}

	return obj, gvk, nil
}

// unversionedSubjectAccessReview is used to decode both v1 and v1beta1 SubjectAccessReview types.
type unversionedSubjectAccessReview struct {
	authorizationv1.SubjectAccessReview
}

var _ runtime.Object = &unversionedSubjectAccessReview{}
