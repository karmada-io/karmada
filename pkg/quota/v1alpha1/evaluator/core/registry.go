package core

import (
	"k8s.io/apimachinery/pkg/util/clock"

	quota "github.com/karmada-io/karmada/pkg/util/quota/v1alpha1"
)

// NewEvaluators returns the list of static evaluators that manage more than counts
func NewEvaluators(f quota.ListerForResourceFunc) []quota.Evaluator {
	// these evaluators have special logic
	result := []quota.Evaluator{
		NewResourceBindingEvaluator(f, clock.RealClock{}),
	}
	return result
}
