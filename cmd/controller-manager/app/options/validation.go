package options

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/karmada-io/karmada/pkg/util"
)

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() field.ErrorList {
	errs := field.ErrorList{}
	newPath := field.NewPath("Options")

	skippedResourceConfig := util.NewSkippedResourceConfig()
	if err := skippedResourceConfig.Parse(o.SkippedPropagatingAPIs); err != nil {
		errs = append(errs, field.Invalid(newPath.Child("SkippedPropagatingAPIs"), o.SkippedPropagatingAPIs, "Invalid API string"))
	}
	if o.SecurePort < 0 || o.SecurePort > 65535 {
		errs = append(errs, field.Invalid(newPath.Child("SecurePort"), o.SecurePort, "must be between 0 and 65535 inclusive"))
	}
	if o.ClusterStatusUpdateFrequency.Duration <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterStatusUpdateFrequency"), o.ClusterStatusUpdateFrequency, "must be greater than 0"))
	}
	if o.ClusterLeaseDuration.Duration <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterLeaseDuration"), o.ClusterLeaseDuration, "must be greater than 0"))
	}
	if o.ClusterMonitorPeriod.Duration <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterMonitorPeriod"), o.ClusterMonitorPeriod, "must be greater than 0"))
	}
	if o.ClusterMonitorGracePeriod.Duration <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterMonitorGracePeriod"), o.ClusterMonitorGracePeriod, "must be greater than 0"))
	}
	if o.ClusterStartupGracePeriod.Duration <= 0 {
		errs = append(errs, field.Invalid(newPath.Child("ClusterStartupGracePeriod"), o.ClusterStartupGracePeriod, "must be greater than 0"))
	}
	for index, ns := range o.SkippedPropagatingNamespaces {
		if _, err := regexp.Compile(fmt.Sprintf("^%s$", ns)); err != nil {
			errs = append(errs, field.Invalid(newPath.Child("SkippedPropagatingNamespaces").Index(index), ns, "Invalid namespace regular expression"))
		}
	}
	return errs
}
