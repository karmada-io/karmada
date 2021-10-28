package options

import (
	"fmt"

	"github.com/karmada-io/karmada/pkg/util"
)

func validateSkippedResourceConfig(opts *Options) []error {
	var errs []error
	skippedResourceConfig := util.NewSkippedResourceConfig()
	if err := skippedResourceConfig.Parse(opts.SkippedPropagatingAPIs); err != nil {
		errs = append(errs, err)
	}

	return errs
}

func validateSecurePort(opts *Options) []error {
	var errs []error
	if opts.SecurePort < 0 || opts.SecurePort > 65535 {
		errs = append(errs, fmt.Errorf("--secure-port %v must be between 0 and 65535 inclusive. ", opts.SecurePort))
	}
	return errs
}

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() []error {
	var errs []error
	errs = append(errs, validateSkippedResourceConfig(o)...)
	errs = append(errs, validateSecurePort(o)...)

	return errs
}
