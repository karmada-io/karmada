package customizedexplorer

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/klog/v2"
	utiltrace "k8s.io/utils/trace"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	workv1alpha2 "github.com/karmada-io/karmada/pkg/apis/work/v1alpha2"
	"github.com/karmada-io/karmada/pkg/crdexplorer/customizedexplorer/configmanager"
	"github.com/karmada-io/karmada/pkg/crdexplorer/customizedexplorer/webhook"
	crdexplorerutil "github.com/karmada-io/karmada/pkg/util/crdexplorer"
	"github.com/karmada-io/karmada/pkg/util/informermanager"
)

// CustomizedExplorer explore custom resource with webhook configuration.
type CustomizedExplorer struct {
	hookManager   configmanager.ConfigManager
	configManager *webhookutil.ClientManager
}

// NewCustomizedExplorer return a new CustomizedExplorer.
func NewCustomizedExplorer(kubeconfig string, informer informermanager.SingleClusterInformerManager) (*CustomizedExplorer, error) {
	cm, err := webhookutil.NewClientManager(
		[]schema.GroupVersion{configv1alpha1.SchemeGroupVersion},
		configv1alpha1.AddToScheme,
	)
	if err != nil {
		return nil, err
	}
	authInfoResolver, err := webhookutil.NewDefaultAuthenticationInfoResolver(kubeconfig)
	if err != nil {
		return nil, err
	}
	cm.SetAuthenticationInfoResolver(authInfoResolver)
	cm.SetServiceResolver(webhookutil.NewDefaultServiceResolver())

	return &CustomizedExplorer{
		hookManager:   configmanager.NewExploreConfigManager(informer),
		configManager: &cm,
	}, nil
}

// HookEnabled tells if any hook exist for specific resource type and operation type.
func (e *CustomizedExplorer) HookEnabled(attributes *webhook.RequestAttributes) bool {
	if !e.hookManager.HasSynced() {
		klog.Errorf("not yet ready to handle request")
		return false
	}

	hook := e.getFirstRelevantHook(attributes)
	if hook == nil {
		klog.V(4).Infof("Hook explorer is not enabled for kind %q with operation %q.",
			attributes.Object.GroupVersionKind(), attributes.Operation)
	}
	return hook != nil
}

// GetReplicas returns the desired replicas of the object as well as the requirements of each replica.
// return matched value to indicate whether there is a matching hook.
func (e *CustomizedExplorer) GetReplicas(ctx context.Context, attributes *webhook.RequestAttributes) (replica int32, requires *workv1alpha2.ReplicaRequirements, matched bool, err error) {
	var response *webhook.ResponseAttributes
	response, matched, err = e.explore(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}

	return response.Replicas, response.ReplicaRequirements, matched, nil
}

// Retain returns the patch that based on the "desired" object but with values retained from the "observed" object.
// return matched value to indicate whether there is a matching hook.
func (e *CustomizedExplorer) Retain(ctx context.Context, attributes *webhook.RequestAttributes) (patch []byte, patchType configv1alpha1.PatchType, matched bool, err error) {
	var response *webhook.ResponseAttributes
	response, matched, err = e.explore(ctx, attributes)
	if err != nil {
		return
	}
	if !matched {
		return
	}

	return response.Patch, response.PatchType, matched, nil
}

func (e *CustomizedExplorer) getFirstRelevantHook(attributes *webhook.RequestAttributes) configmanager.WebhookAccessor {
	relevantHooks := make([]configmanager.WebhookAccessor, 0)
	for _, hook := range e.hookManager.HookAccessors() {
		if shouldCallHook(hook, attributes) {
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

func (e *CustomizedExplorer) explore(ctx context.Context, attributes *webhook.RequestAttributes) (*webhook.ResponseAttributes, bool, error) {
	if !e.hookManager.HasSynced() {
		return nil, false, fmt.Errorf("not yet ready to handle request")
	}

	hook := e.getFirstRelevantHook(attributes)
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
	var response *webhook.ResponseAttributes
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

func shouldCallHook(hook configmanager.WebhookAccessor, attributes *webhook.RequestAttributes) bool {
	for _, rule := range hook.GetRules() {
		matcher := crdexplorerutil.Matcher{
			Operation: attributes.Operation,
			Object:    attributes.Object,
			Rule:      rule,
		}
		if matcher.Matches() {
			return true
		}
	}
	return false
}

func (e *CustomizedExplorer) callHook(ctx context.Context, hook configmanager.WebhookAccessor, attributes *webhook.RequestAttributes) (*webhook.ResponseAttributes, error) {
	uid, req, err := webhook.CreateExploreReview(hook.GetExploreReviewVersions(), attributes)
	if err != nil {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: hook.GetUID(),
			Reason:      fmt.Errorf("could not create ResourceInterpreterContext objects: %w", err),
		}
	}

	client, err := hook.GetRESTClient(e.configManager)
	if err != nil {
		return nil, &webhookutil.ErrCallingWebhook{
			WebhookName: hook.GetUID(),
			Reason:      fmt.Errorf("could not get REST client: %w", err),
		}
	}

	trace := utiltrace.New("Call resource explore webhook",
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

	var res *webhook.ResponseAttributes
	res, err = webhook.VerifyExploreReview(uid, attributes.Operation, response)
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
