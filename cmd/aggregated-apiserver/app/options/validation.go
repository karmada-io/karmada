package options

import (
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// Validate validates Options.
func (o *Options) Validate() error {
	var errs []error
	errs = append(errs, o.RecommendedOptions.Validate()...)

	return utilerrors.NewAggregate(errs)
}
