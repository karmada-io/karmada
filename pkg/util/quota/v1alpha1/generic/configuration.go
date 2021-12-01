package generic

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/karmada-io/karmada/pkg/util/quota/v1alpha1"
)

// implements a basic configuration
type simpleConfiguration struct {
	evaluators       []v1alpha1.Evaluator
	ignoredResources map[schema.GroupResource]struct{}
}

// NewConfiguration creates a quota configuration
func NewConfiguration(evaluators []v1alpha1.Evaluator, ignoredResources map[schema.GroupResource]struct{}) v1alpha1.Configuration {
	return &simpleConfiguration{
		evaluators:       evaluators,
		ignoredResources: ignoredResources,
	}
}

func (c *simpleConfiguration) IgnoredResources() map[schema.GroupResource]struct{} {
	return c.ignoredResources
}

func (c *simpleConfiguration) Evaluators() []v1alpha1.Evaluator {
	return c.evaluators
}
