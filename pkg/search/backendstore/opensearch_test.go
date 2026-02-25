/*
Copyright 2025 The Karmada Authors.

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

package backendstore

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/opensearch-project/opensearch-go"
	"github.com/opensearch-project/opensearch-go/opensearchapi"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientset "k8s.io/client-go/kubernetes"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	coretesting "k8s.io/client-go/testing"
	"k8s.io/klog/v2"

	clusterv1alpha1 "github.com/karmada-io/karmada/pkg/apis/cluster/v1alpha1"
	searchv1alpha1 "github.com/karmada-io/karmada/pkg/apis/search/v1alpha1"
	testing3 "github.com/karmada-io/karmada/pkg/util/testing"
)

func TestNewOpenSearch(t *testing.T) {
	secretName, namespace := "opensearch-credentials", "default"
	tests := []struct {
		name        string
		clusterName string
		cfg         *searchv1alpha1.BackendStoreConfig
		prep        func() error
		wantErr     bool
		errMsg      string
		logMsg      string
	}{
		{
			name:        "NewOpenSearch_WithoutOpenSearchConfig_OpenSearchConfigIsNil",
			clusterName: "member1",
			cfg:         nil,
			prep:        func() error { return nil },
			wantErr:     true,
			errMsg:      "opensearch config is nil",
		},
		{
			name:        "NewOpenSearch_WithoutAddresses_OpenSearchAddressesNotFound",
			clusterName: "member1",
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{},
				},
			},
			prep:    func() error { return nil },
			wantErr: true,
			errMsg:  "not found opensearch address",
		},
		{
			name:        "WithoutOpenSearchCredentialsSecret_SecretNotFoundUnauthorizedClient",
			clusterName: "member1",
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{"https://10.0.0.1:9200"},
				},
			},
			prep: func() error {
				OpenSearchClientBuilder = func(opensearch.Config) (*opensearch.Client, error) {
					response := &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
					mTransport := computeMTransport(response, nil)
					client := &opensearch.Client{Transport: mTransport}
					client.API = opensearchapi.New(client)
					return client, nil
				}
				return nil
			},
			wantErr: false,
			logMsg:  "Not found secret for opensearch, try to without auth",
		},
		{
			name:        "NetworkIssueWhileGettingSecrets_FailedToGetSecrets",
			clusterName: "member1",
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{"https://10.0.0.1:9200"},
					SecretRef: clusterv1alpha1.LocalSecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			},
			prep: func() error {
				k8sClient = fakeclientset.NewSimpleClientset()
				k8sClient.(*fakeclientset.Clientset).Fake.PrependReactor("get", "secrets", func(coretesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.New("unexpected error: encountered a network issue while getting the secrets")
				})
				OpenSearchClientBuilder = func(opensearch.Config) (*opensearch.Client, error) {
					response := &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
					mTransport := computeMTransport(response, nil)
					client := &opensearch.Client{Transport: mTransport}
					client.API = opensearchapi.New(client)
					return client, nil
				}
				return nil
			},
			wantErr: false,
			logMsg:  "encountered a network issue while getting the secrets",
		},
		{
			name:        "NetworkIssueWhileCreatingOpenSearchClient_FailedToCreateClient",
			clusterName: "member1",
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{"https://10.0.0.1:9200"},
					SecretRef: clusterv1alpha1.LocalSecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			},
			prep: func() error {
				username, password := "opensearchuser", "opensearchpass"
				k8sClient = fakeclientset.NewSimpleClientset()
				if err := createOpenSearchCredentialsSecret(k8sClient, username, password, secretName, namespace); err != nil {
					return fmt.Errorf("failed to create open search credentials secret, got: %v", err)
				}
				OpenSearchClientBuilder = func(opensearch.Config) (*opensearch.Client, error) {
					return nil, errors.New("got network issue")
				}
				return nil
			},
			wantErr: true,
			errMsg:  "cannot create opensearch client: got network issue",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.logMsg != "" {
				r, w, oldStdErr, err := prepLogMsgWatcher()
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := verifyLogMsg(r, w, oldStdErr, test.logMsg); err != nil {
						t.Error(err)
					}
				}()
			}
			if err := test.prep(); err != nil {
				t.Fatalf("failed to prep test environment, got error: %v", err)
			}
			_, err := NewOpenSearch(test.clusterName, test.cfg)
			if err == nil && test.wantErr {
				t.Fatal("expected an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
		})
	}
}

func TestGetOrCreateIndexName(t *testing.T) {
	secretName, namespace := "opensearch-credentials", "default"
	tests := []struct {
		name          string
		us            *unstructured.Unstructured
		os            *OpenSearch
		cfg           *searchv1alpha1.BackendStoreConfig
		client        clientset.Interface
		prep          func(*searchv1alpha1.BackendStoreConfig, clientset.Interface, *unstructured.Unstructured) (*OpenSearch, error)
		wantErr       bool
		errMsg        string
		wantIndexName string
	}{
		{
			name: "NetworkIssueInRequestErr_FailedToCreateIndex",
			us: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{"https://10.0.0.1:9200"},
					SecretRef: clusterv1alpha1.LocalSecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			},
			client: fakeclientset.NewSimpleClientset(),
			prep: func(_ *searchv1alpha1.BackendStoreConfig, client clientset.Interface, _ *unstructured.Unstructured) (*OpenSearch, error) {
				return createOpenSearch(client, nil, errors.New("got network issue while doing the request"), secretName, namespace)
			},
			wantErr: true,
			errMsg:  "got network issue while doing the request",
		},
		{
			name: "NetworkIssueInResponse_FailedToCreateIndex",
			us: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{"https://10.0.0.1:9200"},
					SecretRef: clusterv1alpha1.LocalSecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			},
			client: fakeclientset.NewSimpleClientset(),
			prep: func(_ *searchv1alpha1.BackendStoreConfig, client clientset.Interface, _ *unstructured.Unstructured) (*OpenSearch, error) {
				response := &http.Response{StatusCode: http.StatusBadGateway, Body: http.NoBody}
				return createOpenSearch(client, computeMTransport(response, nil), nil, secretName, namespace)
			},
			wantErr: true,
			errMsg:  "cannot create index: [502 Bad Gateway]",
		},
		{
			name: "RequestErrorIndexExists_UpdateTheIndexInMemoryAndGetIt",
			us: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{"https://10.0.0.1:9200"},
					SecretRef: clusterv1alpha1.LocalSecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			},
			client: fakeclientset.NewSimpleClientset(),
			prep: func(_ *searchv1alpha1.BackendStoreConfig, client clientset.Interface, _ *unstructured.Unstructured) (*OpenSearch, error) {
				return createOpenSearch(client, computeMTransport(nil, errors.New(resourceExistsError)), nil, secretName, namespace)
			},
			wantErr:       false,
			wantIndexName: fmt.Sprintf("%s-deployment", defaultPrefix),
		},
		{
			name: "ResponseErrorIndexExists_UpdateTheIndexInMemoryAndGetIt",
			us: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{"https://10.0.0.1:9200"},
					SecretRef: clusterv1alpha1.LocalSecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			},
			client: fakeclientset.NewSimpleClientset(),
			prep: func(_ *searchv1alpha1.BackendStoreConfig, client clientset.Interface, _ *unstructured.Unstructured) (*OpenSearch, error) {
				response := &http.Response{StatusCode: http.StatusConflict, Body: io.NopCloser(strings.NewReader(resourceExistsError))}
				return createOpenSearch(client, computeMTransport(response, nil), nil, secretName, namespace)
			},
			wantErr:       false,
			wantIndexName: fmt.Sprintf("%s-deployment", defaultPrefix),
		},
		{
			name: "IndexNotExist_CreatedThatIndex",
			us: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{"https://10.0.0.1:9200"},
					SecretRef: clusterv1alpha1.LocalSecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			},
			client: fakeclientset.NewSimpleClientset(),
			prep: func(_ *searchv1alpha1.BackendStoreConfig, client clientset.Interface, _ *unstructured.Unstructured) (*OpenSearch, error) {
				response := &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}
				return createOpenSearch(client, computeMTransport(response, nil), nil, secretName, namespace)
			},
			wantErr:       false,
			wantIndexName: fmt.Sprintf("%s-deployment", defaultPrefix),
		},
		{
			name: "IndexIsAlreadyInMemory_GotThatIndex",
			us: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
				},
			},
			cfg: &searchv1alpha1.BackendStoreConfig{
				OpenSearch: &searchv1alpha1.OpenSearchConfig{
					Addresses: []string{"https://10.0.0.1:9200"},
					SecretRef: clusterv1alpha1.LocalSecretReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			},
			client: fakeclientset.NewSimpleClientset(),
			prep: func(_ *searchv1alpha1.BackendStoreConfig, client clientset.Interface, us *unstructured.Unstructured) (*OpenSearch, error) {
				os, err := createOpenSearch(client, computeMTransport(nil, nil), nil, secretName, namespace)
				if err != nil {
					return nil, err
				}
				indexName := fmt.Sprintf("%s-%s", defaultPrefix, strings.ToLower(us.GetKind()))
				os.indices[indexName] = struct{}{}
				return os, nil
			},
			wantErr:       false,
			wantIndexName: fmt.Sprintf("%s-deployment", defaultPrefix),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Init(test.client)
			os, err := test.prep(test.cfg, test.client, test.us)
			if err != nil {
				t.Fatalf("failed to prep test environment, got error: %v", err)
			}
			test.os = os

			gotIndexName, err := test.os.getOrCreateIndexName(test.us)
			if err == nil && test.wantErr {
				t.Fatal("expecterd an error, but got none")
			}
			if err != nil && !test.wantErr {
				t.Errorf("unexpected error, got: %v", err)
			}
			if err != nil && test.wantErr && !strings.Contains(err.Error(), test.errMsg) {
				t.Errorf("expected error message %s to be in %s", test.errMsg, err.Error())
			}
			if gotIndexName != test.wantIndexName {
				t.Errorf("expected index name to be %s, but got %s", test.wantIndexName, gotIndexName)
			}
		})
	}
}

// prepLogMsgWatcher redirects os.Stderr to a pipe for capturing log messages.
// Returns the pipe's reader and writer, the original os.Stderr, and an error.
func prepLogMsgWatcher() (*os.File, *os.File, *os.File, error) {
	r, w, err := watchStdErr()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to watch stdError, got: %v", err)
	}
	oldStderr := os.Stderr
	os.Stderr = w
	return r, w, oldStderr, err
}

// verifyLogMsg checks if the expected log message is in the captured stderr output.
// Restores os.Stderr after verification and returns an error if the message is missing.
func verifyLogMsg(r, w *os.File, oldStdErr *os.File, logMsgExpected string) error {
	klog.Flush()
	gotLog, err := closeAndReadStdErr(r, w)
	if err != nil {
		return err
	}
	if !strings.Contains(gotLog, logMsgExpected) {
		return fmt.Errorf("expected log message %s to be in %s", logMsgExpected, gotLog)
	}
	os.Stderr = oldStdErr
	return nil
}

// watchStdErr creates a pipe to capture os.Stderr. Returns the pipe's reader, writer, and an error.
func watchStdErr() (*os.File, *os.File, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pipe: %v", err)
	}
	return r, w, err
}

// closeAndReadStdErr closes the writer and reads from the reader to capture stderr output.
// Returns the output as a string and an error if reading fails.
func closeAndReadStdErr(r *os.File, w *os.File) (string, error) {
	// Close the writer to finish the capture.
	w.Close()

	// Read the captured output from the pipe.
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	if err != nil {
		return "", fmt.Errorf("failed to read from pipe: %v", err)
	}

	output := buf.String()
	return output, err
}

// createOpenSearch initializes an OpenSearch client with credentials and an optional mock transport.
// Returns the OpenSearch instance or an error if initialization fails.
func createOpenSearch(client clientset.Interface, transport *testing3.MockOpenSearchTransport, responseError error, secretName, namespace string) (*OpenSearch, error) {
	if err := createOpenSearchCredentialsSecret(client, "opensearchuser", "opensearchpass", secretName, namespace); err != nil {
		return nil, fmt.Errorf("failed to create open search credentials secret, got: %v", err)
	}

	mTransport := computeMTransport(nil, responseError)
	if transport != nil {
		mTransport = transport
	}

	osClient, err := opensearch.NewClient(opensearch.Config{UseResponseCheckOnly: true})
	if err != nil {
		return nil, err
	}
	osClient.Transport = mTransport

	os := &OpenSearch{client: osClient, indices: make(map[string]struct{})}
	return os, nil
}

// computeMTransport creates a mock OpenSearch transport for testing using the given response and error.
func computeMTransport(res *http.Response, err error) *testing3.MockOpenSearchTransport {
	mTransport := testing3.MockOpenSearchTransport{}
	mTransport.PerformFunc = func(*http.Request) (*http.Response, error) {
		return res, err
	}
	return &mTransport
}
