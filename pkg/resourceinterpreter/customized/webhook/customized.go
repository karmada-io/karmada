package webhook

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch/v5"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/klog/v2"
	utiltrace "k8s.io/utils/trace"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/webhook/configmanager"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/webhook/request"
	"github.com/karmada-io/karmada/pkg/util/fedinformer/genericmanager"
	interpreterutil "github.com/karmada-io/karmada/pkg/util/interpreter"
)

// CustomizedInterpreter interpret custom resource with webhook configuration.
type CustomizedInterpreter struct {
	// hookManager caches all webhook configurations.
	hookManager configmanager.ConfigManager
	// clientManager builds REST clients to talk to webhooks.
	clientManager *webhookutil.ClientManager
}

// NewCustomizedInterpreter return a new CustomizedInterpreter.
func NewCustomizedInterpreter(informer genericmanager.SingleClusterInformerManager) (*CustomizedInterpreter, error) {
	cm, err := webhookutil.NewClientManager(
		[]schema.GroupVersion{configv1alpha1.SchemeGroupVersion},
		configv1alpha1.AddToScheme,
	)
	if err != nil {
		return nil, err
	}
	authInfoResolver, err := webhookutil.NewDefaultAuthenticationInfoResolver("")
	if err != nil {
		return nil, err
	}
	cm.SetAuthenticationInfoResolver(authInfoResolver)
	cm.SetServiceResolver(webhookutil.NewDefaultServiceResolver())

	return &CustomizedInterpreter{
		hookManager:   configmanager.NewExploreConfigManager(informer),
		clientManager: &cm,
	}, nil
}

// HookEnabled tells if any hook exist for specific resource gvk and operation type.
func (e *CustomizedInterpreter) HookEnabled(objGVK schema.GroupVersionKind, operation configv1alpha1.InterpreterOperation) bool {
	if !e.hookManager.HasSynced() {
		klog.Errorf("not yet ready to handle request")
		return false
	}

	hook := e.getFirstRelevantHook(objGVK, operation)
	if hook == nil {
		klog.V(4).Infof("Hook interpreter is not enabled for kind %q with operation %q.", objGVK, operation)
	}
	return hook != nil
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
// return matched value to indicate whether there is a matching hook.
func (e *CustomizedInterpreter) GetReplicas(ctx context.Context, attributes *request.Attributes) (replica int32, requires *workv1alpha2.ReplicaRequirements, matched bool, err error) {
	klog.V(4).Infof("Get replicas for object: %v %s/%s with webhook interpreter.",
		attributes.Object.GroupVersionKind(), attributes.Object.GetNamespace(), attributes.Object.GetName())
	var response *request.ResponseAttributes
	response, matched, err = e.interpret(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}

	return response.Replicas, response.ReplicaRequirements, matched, nil
}

// Patch returns the Unstructured object that applied patch response that based on the RequestAttributes.
// return matched value to indicate whether there is a matching hook.
func (e *CustomizedInterpreter) Patch(ctx context.Context, attributes *request.Attributes) (obj *unstructured.Unstructured, matched bool, err error) {
	var response *request.ResponseAttributes
	response, matched, err = e.interpret(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}
	obj, err = applyPatch(attributes.Object, response.Patch, response.PatchType)
	if err != nil {
		return
	}
	return
}

func (e *CustomizedInterpreter) getFirstRelevantHook(objGVK schema.GroupVersionKind, operation configv1alpha1.InterpreterOperation) configmanager.WebhookAccessor {
	relevantHooks := make([]configmanager.WebhookAccessor, 0)
	for _, hook := range e.hookManager.HookAccessors() {
		if shouldCallHook(hook, objGVK, operation) {
			relevantHooks = append(relevantHooks, hook)
		}
	}

	if len(relevantHooks) == 0 {
		return nil
	}

	// Sort relevantHooks alphabetically, taking the first element for spawning remote calls
	sort.SliceStable(relevantHooks, func(i, j int) bool {
		return relevantHooks[i].GetUID() < relevantHooks[j].GetUID()
	})
	return relevantHooks[0]
}

func (e *CustomizedInterpreter) interpret(ctx context.Context, attributes *request.Attributes) (*request.ResponseAttributes, bool, error) {
	if !e.hookManager.HasSynced() {
		return nil, false, fmt.Errorf("not yet ready to handle request")
	}

	hook := e.getFirstRelevantHook(attributes.Object.GroupVersionKind(), attributes.Operation)
	if hook == nil {
		return nil, false, nil
	}

	// Check if the request has already timed out before spawning remote calls
	select {
	case <-ctx.Done():
		// parent context is canceled or timed out, no point in continuing
		err := apierrors.NewTimeoutError("request did not complete within requested timeout", 0)
		return nil, true, err
	default:
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	var response *request.ResponseAttributes
	var callErr error
	go func(hook configmanager.WebhookAccessor) {
		defer wg.Done()
		response, callErr = e.callHook(ctx, hook, attributes)
		if callErr != nil {
			klog.Warningf("Failed calling webhook %v: %v", hook.GetUID(), callErr)
			callErr = apierrors.NewInternalError(callErr)
		}
	}(hook)

	wg.Wait()

	if callErr != nil {
		return nil, true, callErr
	}
	if response == nil {
		return nil, true, apierrors.NewInternalError(fmt.Errorf("get nil response from webhook call"))
	}
	return response, true, nil
}

func shouldCallHook(hook configmanager.WebhookAccessor, objGVK schema.GroupVersionKind, operation configv1alpha1.InterpreterOperation) bool {
	for _, rule := range hook.GetRules() {
		matcher := interpreterutil.Matcher{
			ObjGVK:    objGVK,
			Operation: operation,
			Rule:      rule,
		}
		if matcher.Matches() {
			return true
		}
	}
	return false
}

func (e *CustomizedInterpreter) callHook(ctx context.Context, hook configmanager.WebhookAccessor, attributes *request.Attributes) (*request.ResponseAttributes, error) {
	uid, req, err := request.CreateResourceInterpreterContext(hook.GetInterpreterContextVersions(), attributes)
	if err != nil {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: hook.GetUID(),
			Reason:      fmt.Errorf("could not create ResourceInterpreterContext objects: %w", err),
		}
	}

	client, err := hook.GetRESTClient(e.clientManager)
	if err != nil {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: hook.GetUID(),
			Reason:      fmt.Errorf("could not get REST client: %w", err),
		}
	}

	trace := utiltrace.New("Call resource interpret webhook",
		utiltrace.Field{Key: "configuration", Value: hook.GetConfigurationName()},
		utiltrace.Field{Key: "webhook", Value: hook.GetName()},
		utiltrace.Field{Key: "kind", Value: attributes.Object.GroupVersionKind()},
		utiltrace.Field{Key: "operation", Value: attributes.Operation},
		utiltrace.Field{Key: "UID", Value: uid})
	defer trace.LogIfLong(500 * time.Millisecond)

	// if the webhook has a specific timeout, wrap the context to apply it
	if hook.GetTimeoutSeconds() != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(*hook.GetTimeoutSeconds())*time.Second)
		defer cancel()
	}

	r := client.Post().Body(req)

	// if the context has a deadline, set it as a parameter to inform the backend
	if deadline, ok := ctx.Deadline(); ok {
		if timeout := time.Until(deadline); timeout > 0 {
			// if it's not an even number of seconds, round up to the nearest second
			if truncated := timeout.Truncate(time.Second); truncated != timeout {
				timeout = truncated + time.Second
			}
			r.Timeout(timeout)
		}
	}

	response := &configv1alpha1.ResourceInterpreterContext{}
	err = r.Do(ctx).Into(response)
	if err != nil {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: hook.GetUID(),
			Reason:      fmt.Errorf("failed to call webhook: %w", err),
		}
	}
	trace.Step("Request completed")

	var res *request.ResponseAttributes
	res, err = request.VerifyResourceInterpreterContext(uid, attributes.Operation, response)
	if err != nil {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: hook.GetUID(),
			Reason:      fmt.Errorf("reveived invalid webhook response: %w", err),
		}
	}

	if !res.Successful {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: hook.GetUID(),
			Reason:      fmt.Errorf("webhook call failed, get status code: %d, msg: %s", res.Status.Code, res.Status.Message),
		}
	}

	return res, nil
}

// applyPatch uses patchType mode to patch object.
func applyPatch(object *unstructured.Unstructured, patch []byte, patchType configv1alpha1.PatchType) (*unstructured.Unstructured, error) {
	if len(patch) == 0 && len(patchType) == 0 {
		klog.Infof("Skip apply patch for object(%s: %s) as patch and patchType is nil", object.GroupVersionKind().String(), object.GetName())
		return object, nil
	}
	switch patchType {
	case configv1alpha1.PatchTypeJSONPatch:
		if len(patch) == 0 {
			return object, nil
		}
		patchObj, err := jsonpatch.DecodePatch(patch)
		if err != nil {
			return nil, err
		}
		if len(patchObj) == 0 {
			return object, nil
		}

		objectJSONBytes, err := object.MarshalJSON()
		if err != nil {
			return nil, err
		}
		patchedObjectJSONBytes, err := patchObj.Apply(objectJSONBytes)
		if err != nil {
			return nil, err
		}

		err = object.UnmarshalJSON(patchedObjectJSONBytes)
		return object, err
	default:
		return nil, fmt.Errorf("return patch type %s is not support", patchType)
	}
}

// GetDependencies returns the dependencies of give object.
// return matched value to indicate whether there is a matching hook.
func (e *CustomizedInterpreter) GetDependencies(ctx context.Context, attributes *request.Attributes) (dependencies []configv1alpha1.DependentObjectReference, matched bool, err error) {
	var response *request.ResponseAttributes
	response, matched, err = e.interpret(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}

	return response.Dependencies, matched, nil
}

// ReflectStatus returns the status of the object.
// return matched value to indicate whether there is a matching hook.
func (e *CustomizedInterpreter) ReflectStatus(ctx context.Context, attributes *request.Attributes) (status *runtime.RawExtension, matched bool, err error) {
	var response *request.ResponseAttributes
	response, matched, err = e.interpret(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}

	return &response.RawStatus, matched, nil
}

// InterpretHealth returns the health state of the object.
// return matched value to indicate whether there is a matching hook.
func (e *CustomizedInterpreter) InterpretHealth(ctx context.Context, attributes *request.Attributes) (healthy bool, matched bool, err error) {
	var response *request.ResponseAttributes
	response, matched, err = e.interpret(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}

	return response.Healthy, matched, nil
}
