package options

import (
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() field.ErrorList {
	errs := field.ErrorList{}

	newPath := field.NewPath("Options")
	if net.ParseIP(o.BindAddress) == nil {
		errs = append(errs, field.Invalid(newPath.Child("BindAddress"), o.BindAddress, "not a valid textual representation of an IP address"))
	}

	if o.SecurePort < 0 || o.SecurePort > 65535 {
		errs = append(errs, field.Invalid(newPath.Child("SecurePort"), o.SecurePort, "must be a valid port between 0 and 65535 inclusive"))
	}

	if o.SchedulerEstimatorPort < 0 || o.SchedulerEstimatorPort > 65535 {
		errs = append(errs, field.Invalid(newPath.Child("SchedulerEstimatorPort"), o.SchedulerEstimatorPort, "must be a valid port between 0 and 65535 inclusive"))
	}

	if o.SchedulerEstimatorTimeout.Duration < 0 {
		errs = append(errs, field.Invalid(newPath.Child("SchedulerEstimatorTimeout"), o.SchedulerEstimatorTimeout, "must be greater than or equal to 0"))
	}

	if o.SchedulerName == "" {
		errs = append(errs, field.Invalid(newPath.Child("SchedulerName"), o.SchedulerName, "should not be empty"))
	}

	return errs
}
