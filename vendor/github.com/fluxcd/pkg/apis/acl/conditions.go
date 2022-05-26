/*
Copyright 2021 The Flux authors

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

// These constants define the Condition types for when the GitOps Toolkit components perform ACL assertions.
const (
	// AccessDeniedCondition indicates that access to a resource has been denied by an ACL assertion.
	// The Condition adheres to an "abnormal-true" polarity pattern, and MUST only be present on the resource if the
	// Condition is True.
	AccessDeniedCondition string = "AccessDenied"
)

// These constants define the Condition reasons for when the GitOps Toolkit components perform ACL assertions.
const (
	// AccessDeniedReason indicates that access to a resource has been denied by an ACL assertion.
	AccessDeniedReason string = "AccessDenied"
)
