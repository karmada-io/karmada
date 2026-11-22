/*
Copyright 2026 The Karmada Authors.

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

package get

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	cmdtesting "k8s.io/kubectl/pkg/cmd/testing"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	karmadaclientset "github.com/karmada-io/karmada/pkg/generated/clientset/versioned"
	"github.com/karmada-io/karmada/pkg/karmadactl/options"
)

const (
	existingConfigMap = `{"kind":"ConfigMap","apiVersion":"v1","metadata":{"name":"exists","namespace":"test"},"data":{"k":"v"}}`
	notFoundStatus    = `{"kind":"Status","apiVersion":"v1","metadata":{},"status":"Failure","message":"configmaps \"missing\" not found","reason":"NotFound","details":{"name":"missing","kind":"configmaps"},"code":404}`
)

const existingConfigMapTable = `{"kind":"Table","apiVersion":"meta.k8s.io/v1","columnDefinitions":[{"name":"Name","type":"string"},{"name":"Data","type":"integer"},{"name":"Age","type":"string"}],"rows":[{"cells":["exists",1,"1d"],"object":{"kind":"PartialObjectMetadata","apiVersion":"meta.k8s.io/v1","metadata":{"name":"exists","namespace":"test"}}}]}`

type testFactory struct {
	cmdutil.Factory
}

func (testFactory) KarmadaClientSet() (karmadaclientset.Interface, error) {
	return nil, nil
}

func (testFactory) FactoryForMemberCluster(string) (cmdutil.Factory, error) {
	panic("not implemented")
}

func jsonResponse(code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Header:     cmdtesting.DefaultHeader(),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// newConfigMapFactory returns a factory whose API serves the ConfigMap
// "exists" and answers NotFound for the ConfigMap "missing".
func newConfigMapFactory(t *testing.T) *cmdtesting.TestFactory {
	t.Helper()
	tf := cmdtesting.NewTestFactory().WithNamespace("test")
	t.Cleanup(tf.Cleanup)

	client := &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/configmaps/exists"):
				body := existingConfigMap
				if strings.Contains(req.Header.Get("Accept"), "as=Table") {
					body = existingConfigMapTable
				}
				return jsonResponse(http.StatusOK, body), nil
			case req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/configmaps/missing"):
				return jsonResponse(http.StatusNotFound, notFoundStatus), nil
			default:
				return nil, fmt.Errorf("unexpected request: %s %s", req.Method, req.URL.Path)
			}
		}),
	}
	tf.Client = client
	tf.UnstructuredClient = client
	tf.ClientConfigVal = &restclient.Config{
		APIPath: "/api",
		ContentConfig: restclient.ContentConfig{
			NegotiatedSerializer: scheme.Codecs.WithoutConversion(),
			ContentType:          runtime.ContentTypeJSON,
			GroupVersion:         &corev1.SchemeGroupVersion,
		},
	}
	return tf
}

func newGetOptions(output string, ignoreNotFound bool) (*CommandGetOptions, *bytes.Buffer) {
	streams, _, out, _ := genericiooptions.NewTestIOStreams()
	g := NewCommandGetOptions(streams)
	g.PrintFlags.OutputFormat = &output
	g.ServerPrint = false
	g.IsHumanReadablePrinter = false
	g.Namespace = "test"
	g.OperationScope = options.KarmadaControlPlane
	g.IgnoreNotFound = ignoreNotFound
	g.ToPrinter = g.getResourcePrinter()
	return g, out
}

func TestRunStructuredOutputNotFound(t *testing.T) {
	tests := []struct {
		name           string
		output         string
		ignoreNotFound bool
		args           []string
		wantErr        bool
		wantOut        string
	}{
		{name: "json, existing resource", output: "json", args: []string{"configmaps", "exists"}, wantOut: `"name": "exists"`},
		{name: "json, missing resource", output: "json", args: []string{"configmaps", "missing"}, wantErr: true},
		{name: "yaml, missing resource", output: "yaml", args: []string{"configmaps", "missing"}, wantErr: true},
		{name: "json, missing resource with ignore-not-found", output: "json", ignoreNotFound: true, args: []string{"configmaps", "missing"}},
		{name: "yaml, missing resource with ignore-not-found", output: "yaml", ignoreNotFound: true, args: []string{"configmaps", "missing"}},
		{name: "json, partial failure", output: "json", args: []string{"configmaps", "exists", "missing"}, wantErr: true, wantOut: `"name": "exists"`},
		{name: "yaml, partial failure", output: "yaml", args: []string{"configmaps", "exists", "missing"}, wantErr: true, wantOut: "name: exists"},
		{name: "json, partial failure with ignore-not-found", output: "json", ignoreNotFound: true, args: []string{"configmaps", "exists", "missing"}, wantOut: `"name": "exists"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, out := newGetOptions(tt.output, tt.ignoreNotFound)
			err := g.Run(testFactory{Factory: newConfigMapFactory(t)}, tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				if !strings.Contains(err.Error(), "not found") {
					t.Errorf("expected a not found error, got: %v", err)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if tt.wantOut != "" && !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantOut, out.String())
			}
		})
	}
}

func newTableGetOptions(ignoreNotFound bool) (*CommandGetOptions, *bytes.Buffer) {
	streams, _, out, _ := genericiooptions.NewTestIOStreams()
	g := NewCommandGetOptions(streams)
	output := ""
	g.PrintFlags.OutputFormat = &output
	g.ServerPrint = true
	g.IsHumanReadablePrinter = true
	g.Namespace = "test"
	g.OperationScope = options.KarmadaControlPlane
	g.IgnoreNotFound = ignoreNotFound
	g.ToPrinter = g.getResourcePrinter()
	return g, out
}

func TestRunTableOutputNotFound(t *testing.T) {
	tests := []struct {
		name           string
		ignoreNotFound bool
		args           []string
		wantErr        bool
		wantOut        string
	}{
		{name: "existing resource", args: []string{"configmaps", "exists"}, wantOut: "exists"},
		{name: "missing resource", args: []string{"configmaps", "missing"}, wantErr: true},
		{name: "missing resource with ignore-not-found", ignoreNotFound: true, args: []string{"configmaps", "missing"}},
		{name: "partial failure", args: []string{"configmaps", "exists", "missing"}, wantErr: true},
		{name: "partial failure with ignore-not-found", ignoreNotFound: true, args: []string{"configmaps", "exists", "missing"}, wantOut: "exists"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, out := newTableGetOptions(tt.ignoreNotFound)
			err := g.Run(testFactory{Factory: newConfigMapFactory(t)}, tt.args)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got nil")
				}
				if !strings.Contains(err.Error(), "not found") {
					t.Errorf("expected a not found error, got: %v", err)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}

			if tt.wantOut != "" && !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("expected output to contain %q, got:\n%s", tt.wantOut, out.String())
			}
		})
	}
}
