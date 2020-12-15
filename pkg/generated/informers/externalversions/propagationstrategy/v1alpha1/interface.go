// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	internalinterfaces "github.com/karmada-io/karmada/pkg/generated/informers/externalversions/internalinterfaces"
)

// Interface provides access to all the informers in this group version.
type Interface interface {
	// PropagationBindings returns a PropagationBindingInformer.
	PropagationBindings() PropagationBindingInformer
	// PropagationPolicies returns a PropagationPolicyInformer.
	PropagationPolicies() PropagationPolicyInformer
	// PropagationWorks returns a PropagationWorkInformer.
	PropagationWorks() PropagationWorkInformer
}

type version struct {
	factory          internalinterfaces.SharedInformerFactory
	namespace        string
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// New returns a new Interface.
func New(f internalinterfaces.SharedInformerFactory, namespace string, tweakListOptions internalinterfaces.TweakListOptionsFunc) Interface {
	return &version{factory: f, namespace: namespace, tweakListOptions: tweakListOptions}
}

// PropagationBindings returns a PropagationBindingInformer.
func (v *version) PropagationBindings() PropagationBindingInformer {
	return &propagationBindingInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// PropagationPolicies returns a PropagationPolicyInformer.
func (v *version) PropagationPolicies() PropagationPolicyInformer {
	return &propagationPolicyInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}

// PropagationWorks returns a PropagationWorkInformer.
func (v *version) PropagationWorks() PropagationWorkInformer {
	return &propagationWorkInformer{factory: v.factory, namespace: v.namespace, tweakListOptions: v.tweakListOptions}
}
