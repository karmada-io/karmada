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
	err := wait.PollImmediate(pollInterval, pollTimeout, func() (done bool, err error) {
		saRefSecret, err := client.CoreV1().Secrets(saNamespace).Get(context.TODO(), saName, metav1.GetOptions{})
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

	// #nosec
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	httpClient := &http.Client{Transport: transport}
	resp, err := httpClient.Do(res)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	return resp.StatusCode, nil
}
