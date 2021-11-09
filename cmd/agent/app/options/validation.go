package options

import (
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() field.ErrorList {
	errs := field.ErrorList{}

	newPath := field.NewPath("Options")
	if len(o.ClusterName) == 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterName"), o.ClusterName, "clusterName cannot be empty"))
	}
	if o.ClusterStatusUpdateFrequency.Duration < 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterStatusUpdateFrequency"), o.ClusterStatusUpdateFrequency, "must be greater than or equal to 0"))
	}

	if o.ClusterLeaseDuration.Duration < 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterLeaseDuration"), o.ClusterLeaseDuration, "must be greater than or equal to 0"))
	}

	if o.ClusterLeaseRenewIntervalFraction <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterLeaseRenewIntervalFraction"), o.ClusterLeaseRenewIntervalFraction, "must be greater than 0"))
	}

	return errs
}
