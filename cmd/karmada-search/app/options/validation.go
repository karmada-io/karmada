/*
Copyright 2022 The Karmada Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package options

import (
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

// Validate validates Options.
func (o *Options) Validate() error {
	var errs []error
	errs = append(errs, o.Etcd.Validate()...)
	errs = append(errs, o.SecureServing.Validate()...)
	errs = append(errs, o.Authentication.Validate()...)
	errs = append(errs, o.Authorization.Validate()...)
	errs = append(errs, o.Audit.Validate()...)
	errs = append(errs, o.Features.Validate()...)
	errs = append(errs, o.CoreAPI.Validate()...)
	errs = append(errs, o.ServerRunOptions.Validate()...)
	return utilerrors.NewAggregate(errs)
}
