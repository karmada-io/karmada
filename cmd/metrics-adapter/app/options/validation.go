package options

import (
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() error {
	var errs []error

	errs = append(errs, o.CustomMetricsAdapterServerOptions.Validate()...)

	return utilerrors.NewAggregate(errs)
}
