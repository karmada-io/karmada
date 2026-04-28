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

package helper

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const (
	// pollInterval defines the interval time for a poll operation.
	pollInterval = 5 * time.Second
	// pollTimeout defines the time after which the poll operation times out.
	pollTimeout = 300 * time.Second
)

// GetTokenFromServiceAccount get token from serviceAccount's related secret.
func GetTokenFromServiceAccount(client kubernetes.Interface, saNamespace, saName string) (string, error) {
	klog.Infof("Get serviceAccount(%s/%s)'s refer secret", saNamespace, saName)
	var token string
	err := wait.PollUntilContextTimeout(context.TODO(), pollInterval, pollTimeout, true, func(ctx context.Context) (done bool, err error) {
		saRefSecret, err := client.CoreV1().Secrets(saNamespace).Get(ctx, saName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			klog.Errorf("Failed to get serviceAccount(%s/%s)'s refer secret, error: %v", saNamespace, saName, err)
			return false, nil
		}

		tokenByte, ok := saRefSecret.Data["token"]
		if !ok {
			return false, nil
		}
		token = base64.StdEncoding.EncodeToString(tokenByte)
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// DoRequest use bearer token to call karmada-apiserver.
func DoRequest(urlPath string, token string) (int, error) {
	decodeToken, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		klog.Infof("DoRequest decode token failed: %v", err)
		return 0, err
	}
	bearToken := fmt.Sprintf("Bearer %s", string(decodeToken))

	res, err := http.NewRequest(http.MethodGet, urlPath, nil)
	if err != nil {
		return 0, err
	}
	res.Header.Add("Authorization", bearToken)

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // nolint:gosec // G402: TLS InsecureSkipVerify set true.
	}
	httpClient := &http.Client{Transport: transport}
	resp, err := httpClient.Do(res)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}
