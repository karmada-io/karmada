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

package clusterauthorization

import (
	"context"
	"errors"
	"fmt"
	"strings"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apiextensions-apiserver/pkg/apihelpers"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/karmada-io/karmada/pkg/util/authorization_webhook"
	"github.com/karmada-io/karmada/pkg/webhook/clusterrestriction"
)

// Authorizer authorizes access to resources in karmada control-plane from karmada-agents
type Authorizer struct {
	client client.Client
}

var _ authorization_webhook.Handler = &Authorizer{}

var (
	noOpinionForNonExistingResources  = authorization_webhook.NoOpinion("clusterauthorization webhook is not interested in requests for non-existing resources")
	noOpinionForNonResources          = authorization_webhook.NoOpinion("clusterauthorization webhook is not interested in requests for non-resources")
	noOpinionForCollectionResources   = authorization_webhook.NoOpinion("clusterauthorization webhook is not interested in requests to resource collection")
	noOpinionResourcesWithoutOwner    = authorization_webhook.NoOpinion("clusterauthorization webhook is not interested in requests for resources without agent ownership")
	denyAccessingResourceOwnedByOther = func(verb string) authorization_webhook.Response {
		return authorization_webhook.Denied(fmt.Sprintf("clusterauthorization webhook prohibits karmada-agent from %s-ing resources owned by other karamda-agent", verb))
	}
)

// Handle yields a response to an SubjectAccessReview.
// The supplied context is extracted from the received http.Request, allowing wrapping
// http.Handlers to inject values into and control cancelation of downstream request processing.
func (a *Authorizer) Handle(ctx context.Context, req authorization_webhook.Request) authorization_webhook.Response {
	// Focusing on requests from karmada-agent
	agentOperating, isAgent := clusterrestriction.AgentIdentity(authenticationv1.UserInfo{Username: req.Spec.User, Groups: req.Spec.Groups})
	if !isAgent {
		return authorization_webhook.NoOpinion("clusterauthorization webhook is only interested in requests from karmada-agent")
	}
	switch {
	case req.Spec.ResourceAttributes != nil:
		return a.authorizeResourceAccess(ctx, *req.Spec.ResourceAttributes, agentOperating)
	case req.Spec.NonResourceAttributes != nil:
		return a.authorizeNonResourceAccess(ctx, *req.Spec.NonResourceAttributes, agentOperating)
	default:
		return authorization_webhook.Errored(errors.New("it should not happen"))
	}
}

func (a *Authorizer) authorizeResourceAccess(ctx context.Context, attr authorizationv1.ResourceAttributes, agentOperating string) authorization_webhook.Response {
	if attr.Name == "" {
		return noOpinionForCollectionResources
	}

	object, err := a.getObject(ctx, schema.GroupVersionResource{
		Group:    attr.Group,
		Version:  attr.Version,
		Resource: attr.Resource,
	}, client.ObjectKey{Namespace: attr.Namespace, Name: attr.Name})

	if err != nil {
		if apierrors.IsNotFound(err) {
			return noOpinionForNonExistingResources
		}
		return authorization_webhook.Errored(err)
	}

	objectOwner, ok := object.GetAnnotations()[clusterrestriction.OwnerAnnotationKey]
	if !ok {
		return noOpinionResourcesWithoutOwner
	}

	if agentOperating != objectOwner {
		return denyAccessingResourceOwnedByOther(attr.Verb)
	}

	return authorization_webhook.Allowed()
}

func (a *Authorizer) authorizeNonResourceAccess(_ context.Context, _ authorizationv1.NonResourceAttributes, _ string) authorization_webhook.Response {
	return noOpinionForNonResources
}

func (a *Authorizer) getObject(ctx context.Context, gvr schema.GroupVersionResource, key client.ObjectKey) (client.Object, error) {
	gvk, err := a.client.RESTMapper().KindFor(gvr)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	if err := a.client.Get(ctx, key, u); err != nil {
		return nil, err
	}
	return u, err
}

func (a *Authorizer) karmadaResourceEventHandlerFuncs(mgr controllerruntime.Manager) toolscache.ResourceEventHandlerFuncs {
	activateCache := func(obj interface{}) {
		crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
		if !ok {
			return
		}
		if !apihelpers.IsCRDConditionTrue(crd, apiextensionsv1.Established) {
			return
		}
		if !strings.HasSuffix(crd.Spec.Group, ".karmada.io") {
			return
		}

		for _, v := range crd.Spec.Versions {
			if v.Served {
				gvk := schema.GroupVersionKind{
					Group:   crd.Spec.Group,
					Kind:    crd.Spec.Names.Kind,
					Version: v.Name,
				}
				if mgr.GetScheme().Recognizes(gvk) {
					informer, err := mgr.GetCache().GetInformerForKind(context.Background(), gvk)
					if err != nil {
						return
					}
					// registering some event handler is required to activate caches
					if _, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
						AddFunc: func(obj interface{}) { /* nop */ },
					}); err != nil {
						return
					}
				}
			}
		}
	}

	return toolscache.ResourceEventHandlerFuncs{
		AddFunc: activateCache,
		UpdateFunc: func(_, newObj interface{}) {
			activateCache(newObj)
		},
	}
}

// SetupWithManager setups the authorizer on the top of Manager
func (a *Authorizer) SetupWithManager(ctx context.Context, mgr controllerruntime.Manager) error {
	a.client = mgr.GetClient()

	// Setting up caches on core API objects
	cacheTargets := []client.Object{
		&corev1.Namespace{},
		&corev1.Secret{},
		&corev1.Event{},
		&coordinationv1.Lease{},
	}
	for _, cacheTarget := range cacheTargets {
		clusterInformer, err := mgr.GetCache().GetInformer(ctx, cacheTarget)
		if err != nil {
			return err
		}
		// registering some event handler is required to activate caches
		if _, err := clusterInformer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) { /* nop */ },
		}); err != nil {
			return err
		}
	}

	// Setting up karmada API caches lazily
	// because CRD has not been installed in karmada controlplane in some cases (particularly initial setup)
	crdInformer, err := mgr.GetCache().GetInformer(ctx, &apiextensionsv1.CustomResourceDefinition{})
	if err != nil {
		return err
	}
	if _, err := crdInformer.AddEventHandler(a.karmadaResourceEventHandlerFuncs(mgr)); err != nil {
		return err
	}

	hookServer := mgr.GetWebhookServer()
	hookServer.Register("/clusterauthorization/authorize", &authorization_webhook.Webhook{Handler: a})
	return nil
}
