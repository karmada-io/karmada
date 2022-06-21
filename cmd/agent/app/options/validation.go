package options

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/karmada-io/karmada/pkg/apis/cluster/validation"
)

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() field.ErrorList {
	errs := field.ErrorList{}

	newPath := field.NewPath("Options")
	if errMsgs := validation.ValidateClusterName(o.ClusterName); len(errMsgs) > 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterName"), o.ClusterName, strings.Join(errMsgs, ",")))
	}

	if o.SecurePort < 0 || o.SecurePort > 65535 {
		errs = append(errs, field.Invalid(newPath.Child("SecurePort"), o.SecurePort, "must be between 0 and 65535 inclusive"))
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
