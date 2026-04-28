/*
Copyright 2023 The Karmada Authors.

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

package webhookconfiguration

import (
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/karmada-io/karmada/operator/pkg/util"
	"github.com/karmada-io/karmada/operator/pkg/util/apiclient"
)

// EnsureWebhookConfiguration creates karmada webhook mutatingWebhookConfiguration and validatingWebhookConfiguration
func EnsureWebhookConfiguration(client clientset.Interface, namespace, name, caBundle string) error {
	if err := mutatingWebhookConfiguration(client, namespace, name, caBundle); err != nil {
		return err
	}
	return validatingWebhookConfiguration(client, namespace, name, caBundle)
}

func mutatingWebhookConfiguration(client clientset.Interface, namespace, name, caBundle string) error {
	configurationBytes, err := util.ParseTemplate(KarmadaWebhookMutatingWebhookConfiguration, struct {
		Service   string
		Namespace string
		CaBundle  string
	}{
		Service:   util.KarmadaWebhookName(name),
		Namespace: namespace,
		CaBundle:  caBundle,
	})
	if err != nil {
		return fmt.Errorf("error when parsing Webhook MutatingWebhookConfiguration template: %w", err)
	}

	mwc := &admissionregistrationv1.MutatingWebhookConfiguration{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), configurationBytes, mwc); err != nil {
		return fmt.Errorf("err when decoding Webhook MutatingWebhookConfiguration: %w", err)
	}

	return apiclient.CreateOrUpdateMutatingWebhookConfiguration(client, mwc)
}

func validatingWebhookConfiguration(client clientset.Interface, namespace, name, caBundle string) error {
	configurationBytes, err := util.ParseTemplate(KarmadaWebhookValidatingWebhookConfiguration, struct {
		Service   string
		Namespace string
		CaBundle  string
	}{
		Service:   util.KarmadaWebhookName(name),
		Namespace: namespace,
		CaBundle:  caBundle,
	})
	if err != nil {
		return fmt.Errorf("error when parsing Webhook ValidatingWebhookConfiguration template: %w", err)
	}

	vwc := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	if err := kuberuntime.DecodeInto(clientsetscheme.Codecs.UniversalDecoder(), configurationBytes, vwc); err != nil {
		return fmt.Errorf("err when decoding Webhook ValidatingWebhookConfiguration: %w", err)
	}

	return apiclient.CreateOrUpdateValidatingWebhookConfiguration(client, vwc)
}
