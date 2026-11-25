/*
Copyright 2020 The Karmada Authors.

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

package util

import (
	"context"
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeclient "k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// GetSecret just try to get the secret.
func GetSecret(client kubeclient.Interface, namespace, name string) (*corev1.Secret, error) {
	return client.CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
}

// CreateSecret just try to create the secret.
func CreateSecret(client kubeclient.Interface, secret *corev1.Secret) (*corev1.Secret, error) {
	st, err := client.CoreV1().Secrets(secret.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return client.CoreV1().Secrets(secret.Namespace).Get(context.TODO(), secret.Name, metav1.GetOptions{})
		}
		return nil, err
	}

	return st, nil
}

// PatchSecret just try to patch the secret.
func PatchSecret(client kubeclient.Interface, namespace, name string, pt types.PatchType, patchSecretBody *corev1.Secret) error {
	patchSecretByte, err := json.Marshal(patchSecretBody)
	if err != nil {
		klog.Errorf("Failed to marshal patch body of secret object %v into JSON: %v", patchSecretByte, err)
		return err
	}

	_, err = client.CoreV1().Secrets(namespace).Patch(context.TODO(), name, pt, patchSecretByte, metav1.PatchOptions{})
	if err != nil {
		return err
	}
	return nil
}

// DeleteSecret just try to delete the Secret.
func DeleteSecret(client kubeclient.Interface, namespace, name string) error {
	err := client.CoreV1().Secrets(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}
