package generic

import (
	quota "github.com/karmada-io/karmada/pkg/util/quota/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// implements a basic configuration
type simpleConfiguration struct {
	evaluators       []quota.Evaluator
	ignoredResources map[schema.GroupResource]struct{}
}

// NewConfiguration creates a quota configuration
func NewConfiguration(evaluators []quota.Evaluator, ignoredResources map[schema.GroupResource]struct{}) quota.Configuration {
	return &simpleConfiguration{
		evaluators:       evaluators,
		ignoredResources: ignoredResources,
	}
}

func (c *simpleConfiguration) IgnoredResources() map[schema.GroupResource]struct{} {
	return c.ignoredResources
}

func (c *simpleConfiguration) Evaluators() []quota.Evaluator {
	return c.evaluators
}
