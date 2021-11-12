/*
Copyright The Karmada Authors.

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

package configmanager

import (
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// WebhookAccessor provides a common interface to get webhook configuration.
type WebhookAccessor interface {
	// GetUID gets a string that uniquely identifies the webhook.
	GetUID() string
	// GetConfigurationName gets the name of the webhook configuration that owns this webhook.
	GetConfigurationName() string
	// GetClientConfig gets the webhook ClientConfig field.
	GetClientConfig() admissionregistrationv1.WebhookClientConfig
	// GetRules gets the webhook Rules field.
	GetRules() []configv1alpha1.RuleWithOperations
	// GetFailurePolicy gets the webhook FailurePolicy field.
	GetFailurePolicy() *admissionregistrationv1.FailurePolicyType
	// GetTimeoutSeconds gets the webhook TimeoutSeconds field.
	GetTimeoutSeconds() *int32
	// GetExploreReviewVersions gets the webhook ExploreReviewVersions field.
	GetExploreReviewVersions() []string
}

type resourceExploringAccessor struct {
	*configv1alpha1.ResourceExploringWebhook
	uid               string
	configurationName string
}

// NewResourceExploringAccessor create an accessor for webhook.
func NewResourceExploringAccessor(uid, configurationName string, hook *configv1alpha1.ResourceExploringWebhook) WebhookAccessor {
	return &resourceExploringAccessor{uid: uid, configurationName: configurationName, ResourceExploringWebhook: hook}
}

// GetUID gets a string that uniquely identifies the webhook.
func (a *resourceExploringAccessor) GetUID() string {
	return a.uid
}

// GetConfigurationName gets the name of the webhook configuration that owns this webhook.
func (a *resourceExploringAccessor) GetConfigurationName() string {
	return a.configurationName
}

// GetClientConfig gets the webhook ClientConfig field.
func (a *resourceExploringAccessor) GetClientConfig() admissionregistrationv1.WebhookClientConfig {
	return a.ClientConfig
}

// GetRules gets the webhook Rules field.
func (a *resourceExploringAccessor) GetRules() []configv1alpha1.RuleWithOperations {
	return a.Rules
}

// GetFailurePolicy gets the webhook FailurePolicy field.
func (a *resourceExploringAccessor) GetFailurePolicy() *admissionregistrationv1.FailurePolicyType {
	return a.FailurePolicy
}

// GetTimeoutSeconds gets the webhook TimeoutSeconds field.
func (a *resourceExploringAccessor) GetTimeoutSeconds() *int32 {
	return a.TimeoutSeconds
}

// GetExploreReviewVersions gets the webhook ExploreReviewVersions field.
func (a *resourceExploringAccessor) GetExploreReviewVersions() []string {
	return a.ExploreReviewVersions
}
