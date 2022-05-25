/*
Copyright 2022 The Flux authors

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

package acl

import "github.com/spf13/pflag"

const (
	flagNoCrossNamespaceRefs = "no-cross-namespace-refs"
)

// Options contains the ACL configuration for a GitOps Toolkit controller.
//
// The struct can be used in the main.go file of your controller by binding it to the main flag set, and then utilizing
// the configured options later:
//
//	func main() {
//		var (
//			// other controller specific configuration variables
//			aclOptions acl.Options
//		)
//
//		// Bind the options to the main flag set, and parse it
//		aclOptions.BindFlags(flag.CommandLine)
//		flag.Parse()
//	}
type Options struct {
	// NoCrossNamespaceRefs indicates that references between custom resources are allowed
	// only if the reference and the referee are in the same namespace.
	NoCrossNamespaceRefs bool
}

// BindFlags will parse the given pflag.FlagSet for ACL option flags and set the Options accordingly.
func (o *Options) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.NoCrossNamespaceRefs, flagNoCrossNamespaceRefs, false,
		"When set to true, references between custom resources are allowed "+
			"only if the reference and the referee are in the same namespace.")
}
