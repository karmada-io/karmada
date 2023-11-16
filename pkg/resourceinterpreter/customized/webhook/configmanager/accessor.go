/*
Copyright 2021 The Karmada Authors.

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
	"sync"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	webhookutil "k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/client-go/rest"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
)

// WebhookAccessor provides a common interface to get webhook configuration.
type WebhookAccessor interface {
	// GetUID gets a string that uniquely identifies the webhook.
	GetUID() string
	// GetConfigurationName gets the name of the webhook configuration that owns this webhook.
	GetConfigurationName() string
	// GetName gets the webhook Name field.
	GetName() string
	// GetClientConfig gets the webhook ClientConfig field.
	GetClientConfig() admissionregistrationv1.WebhookClientConfig
	// GetRules gets the webhook Rules field.
	GetRules() []configv1alpha1.RuleWithOperations
	// GetTimeoutSeconds gets the webhook TimeoutSeconds field.
	GetTimeoutSeconds() *int32
	// GetInterpreterContextVersions gets the webhook InterpreterContextVersions field.
	GetInterpreterContextVersions() []string

	// GetRESTClient gets the webhook client.
	GetRESTClient(clientManager *webhookutil.ClientManager) (*rest.RESTClient, error)
}

type resourceExploringAccessor struct {
	*configv1alpha1.ResourceInterpreterWebhook
	uid               string
	configurationName string

	initClient sync.Once
	client     *rest.RESTClient
	clientErr  error
}

// NewResourceExploringAccessor create an accessor for webhook.
func NewResourceExploringAccessor(uid, configurationName string, hook *configv1alpha1.ResourceInterpreterWebhook) WebhookAccessor {
	return &resourceExploringAccessor{uid: uid, configurationName: configurationName, ResourceInterpreterWebhook: hook}
}

// GetUID gets a string that uniquely identifies the webhook.
func (a *resourceExploringAccessor) GetUID() string {
	return a.uid
}

// GetConfigurationName gets the name of the webhook configuration that owns this webhook.
func (a *resourceExploringAccessor) GetConfigurationName() string {
	return a.configurationName
}

// GetName gets the webhook Name field.
func (a *resourceExploringAccessor) GetName() string {
	return a.Name
}

// GetClientConfig gets the webhook ClientConfig field.
func (a *resourceExploringAccessor) GetClientConfig() admissionregistrationv1.WebhookClientConfig {
	return a.ClientConfig
}

// GetRules gets the webhook Rules field.
func (a *resourceExploringAccessor) GetRules() []configv1alpha1.RuleWithOperations {
	return a.Rules
}

// GetTimeoutSeconds gets the webhook TimeoutSeconds field.
func (a *resourceExploringAccessor) GetTimeoutSeconds() *int32 {
	return a.TimeoutSeconds
}

// GetInterpreterContextVersions gets the webhook InterpreterContextVersions field.
func (a *resourceExploringAccessor) GetInterpreterContextVersions() []string {
	return a.InterpreterContextVersions
}

// GetRESTClient gets the webhook client.
func (a *resourceExploringAccessor) GetRESTClient(clientManager *webhookutil.ClientManager) (*rest.RESTClient, error) {
	a.initClient.Do(func() {
		a.client, a.clientErr = clientManager.HookClient(hookClientConfigForWebhook(a.Name, a.ClientConfig))
	})
	return a.client, a.clientErr
}

// hookClientConfigForWebhook construct a webhookutil.ClientConfig using an admissionregistrationv1.WebhookClientConfig
// to access v1alpha1.ResourceInterpreterWebhook. webhookutil.ClientConfig is used to create a HookClient
// and the purpose of the config struct is to share that with other packages that need to create a HookClient.
func hookClientConfigForWebhook(hookName string, config admissionregistrationv1.WebhookClientConfig) webhookutil.ClientConfig {
	clientConfig := webhookutil.ClientConfig{Name: hookName, CABundle: config.CABundle}
	if config.URL != nil {
		clientConfig.URL = *config.URL
	}
	if config.Service != nil {
		clientConfig.Service = &webhookutil.ClientConfigService{
			Name:      config.Service.Name,
			Namespace: config.Service.Namespace,
		}
		if config.Service.Port != nil {
			clientConfig.Service.Port = *config.Service.Port
		} else {
			clientConfig.Service.Port = 443
		}
		if config.Service.Path != nil {
			clientConfig.Service.Path = *config.Service.Path
		}
	}
	return clientConfig
}
