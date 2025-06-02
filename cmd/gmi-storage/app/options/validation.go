/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2025/05/07 16:20:29
 Desc     :
*/

package options

import "k8s.io/apimachinery/pkg/util/validation/field"

// Validate checks Options and return a slice of found errs.
func (o *Options) Validate() field.ErrorList {
	errs := field.ErrorList{}

	return errs
}
