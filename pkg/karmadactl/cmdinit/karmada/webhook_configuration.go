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

package karmada

import (
	"context"
	"encoding/json"
	"fmt"

	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/karmada-io/karmada/pkg/karmadactl/cmdinit/utils"
)

func mutatingConfig(caBundle string, systemNamespace string) string {
	return fmt.Sprintf(`apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: mutating-config
  labels:
    app: mutating-config
webhooks:
  - name: propagationpolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["propagationpolicies"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/mutate-propagationpolicy
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: clusterpropagationpolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["clusterpropagationpolicies"]
        scope: "Cluster"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/mutate-clusterpropagationpolicy
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: overridepolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["overridepolicies"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/mutate-overridepolicy
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: resourcebinding.karmada.io
    rules:
      - operations: ["CREATE"]
        apiGroups: ["work.karmada.io"]
        apiVersions: ["*"]
        resources: ["resourcebindings"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/mutate-resourcebinding
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: clusterresourcebinding.karmada.io
    rules:
      - operations: ["CREATE"]
        apiGroups: ["work.karmada.io"]
        apiVersions: ["*"]
        resources: ["clusterresourcebindings"]
        scope: "Cluster"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/mutate-clusterresourcebinding
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: work.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["work.karmada.io"]
        apiVersions: ["*"]
        resources: ["works"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/mutate-work
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: autoscaling.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["autoscaling.karmada.io"]
        apiVersions: ["*"]
        resources: ["federatedhpas"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/mutate-federatedhpa
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3
  - name: multiclusterservice.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["networking.karmada.io"]
        apiVersions: ["*"]
        resources: ["multiclusterservices"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/mutate-multiclusterservice
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3`, systemNamespace, caBundle)
}

func validatingConfig(caBundle string, systemNamespace string) string {
	return fmt.Sprintf(`apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: validating-config
  labels:
    app: validating-config
webhooks:
  - name: propagationpolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["propagationpolicies"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-propagationpolicy
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: clusterpropagationpolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["clusterpropagationpolicies"]
        scope: "Cluster"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-clusterpropagationpolicy
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: overridepolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["overridepolicies"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-overridepolicy
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: clusteroverridepolicy.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["clusteroverridepolicies"]
        scope: "Cluster"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-clusteroverridepolicy
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: config.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["config.karmada.io"]
        apiVersions: ["*"]
        resources: ["resourceinterpreterwebhookconfigurations"]
        scope: "Cluster"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-resourceinterpreterwebhookconfiguration
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: resourceinterpretercustomization.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["config.karmada.io"]
        apiVersions: ["*"]
        resources: ["resourceinterpretercustomizations"]
        scope: "Cluster"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-resourceinterpretercustomization
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 10
  - name: federatedresourcequota.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["policy.karmada.io"]
        apiVersions: ["*"]
        resources: ["federatedresourcequotas"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-federatedresourcequota
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3
  - name: multiclusteringress.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["networking.karmada.io"]
        apiVersions: ["*"]
        resources: ["multiclusteringresses"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-multiclusteringress
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: ["v1"]
    timeoutSeconds: 3
  - name: federatedhpa.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["autoscaling.karmada.io"]
        apiVersions: ["*"]
        resources: ["federatedhpas"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-federatedhpa
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3
  - name: cronfederatedhpa.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["autoscaling.karmada.io"]
        apiVersions: ["*"]
        resources: ["cronfederatedhpas"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-cronfederatedhpa
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3
  - name: multiclusterservice.karmada.io
    rules:
      - operations: ["CREATE", "UPDATE"]
        apiGroups: ["networking.karmada.io"]
        apiVersions: ["*"]
        resources: ["multiclusterservices"]
        scope: "Namespaced"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-multiclusterservice
      caBundle: %[2]s
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3
  - name: resourcedeletionprotection.karmada.io
    rules:
      - operations: ["DELETE"]
        apiGroups: ["*"]
        apiVersions: ["*"]
        resources: ["*"]
        scope: "*"
    clientConfig:
      url: https://karmada-webhook.%[1]s.svc:443/validate-resourcedeletionprotection
      caBundle: %[2]s
    objectSelector:
      matchExpressions:
        - { key: "resourcetemplate.karmada.io/deletion-protected", operator: "Exists" }
    failurePolicy: Fail
    sideEffects: None
    admissionReviewVersions: [ "v1" ]
    timeoutSeconds: 3
`, systemNamespace, caBundle)
}

func createOrUpdateValidatingWebhookConfiguration(c kubernetes.Interface, staticYaml string) error {
	obj := admissionregistrationv1.ValidatingWebhookConfiguration{}

	if err := json.Unmarshal(utils.StaticYamlToJSONByte(staticYaml), &obj); err != nil {
		klog.Errorln("Error convert json byte to admissionregistration v1 ValidatingWebhookConfiguration struct.")
		return err
	}

	if _, err := c.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.TODO(), &obj, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		existVwc, err := c.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(context.TODO(), obj.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		obj.ResourceVersion = existVwc.ResourceVersion

		if _, err := c.AdmissionregistrationV1().ValidatingWebhookConfigurations().Update(context.TODO(), &obj, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	klog.Infof("ValidatingWebhookConfiguration %s has been created or updated successfully.", obj.Name)

	return nil
}

func createOrUpdateMutatingWebhookConfiguration(c kubernetes.Interface, staticYaml string) error {
	obj := admissionregistrationv1.MutatingWebhookConfiguration{}

	if err := json.Unmarshal(utils.StaticYamlToJSONByte(staticYaml), &obj); err != nil {
		klog.Errorln("Error convert json byte to admissionregistration v1 MutatingWebhookConfiguration struct.")
		return err
	}

	if _, err := c.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), &obj, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}

		existMwc, err := c.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.TODO(), obj.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		obj.ResourceVersion = existMwc.ResourceVersion

		if _, err := c.AdmissionregistrationV1().MutatingWebhookConfigurations().Update(context.TODO(), &obj, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	klog.Infof("MutatingWebhookConfiguration %s has been created or updated successfully.", obj.Name)

	return nil
}
