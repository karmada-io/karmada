package options

import (
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"
)

// Validate checks Options and return a slice of found errs.
func (opts *Options) Validate() field.ErrorList {
	errs := field.ErrorList{}
	rootPath := field.NewPath("Options")

	if net.ParseIP(opts.BindAddress) == nil {
		errs = append(errs, field.Invalid(rootPath.Child("BindAddress"), opts.BindAddress, "not a valid textual representation of an IP address"))
	}

	if opts.SecurePort < 0 || opts.SecurePort > 65535 {
		errs = append(errs, field.Invalid(rootPath.Child("SecurePort"), opts.SecurePort, "must be a valid port between 0 and 65535 inclusive"))
	}

	return errs
}
