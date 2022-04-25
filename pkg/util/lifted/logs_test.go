/*
Copyright 2014 The Kubernetes Authors.

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

// This code is directly lifted from the Kubernetes codebase.
// For reference:
// https://github.com/kubernetes/kubernetes/blob/release-1.23/staging/src/k8s.io/kubectl/pkg/cmd/logs/logs_test.go#L724-L789

package lifted

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"testing/iotest"

	restclient "k8s.io/client-go/rest"
)

func TestDefaultConsumeRequest(t *testing.T) {
	tests := []struct {
		name        string
		request     restclient.ResponseWrapper
		expectedErr string
		expectedOut string
	}{
		{
			name: "error from request stream",
			request: &responseWrapperMock{
				err: errors.New("err from the stream"),
			},
			expectedErr: "err from the stream",
		},
		{
			name: "error while reading",
			request: &responseWrapperMock{
				data: iotest.TimeoutReader(strings.NewReader("Some data")),
			},
			expectedErr: iotest.ErrTimeout.Error(),
			expectedOut: "Some data",
		},
		{
			name: "read with empty string",
			request: &responseWrapperMock{
				data: strings.NewReader(""),
			},
			expectedOut: "",
		},
		{
			name: "read without new lines",
			request: &responseWrapperMock{
				data: strings.NewReader("some string without a new line"),
			},
			expectedOut: "some string without a new line",
		},
		{
			name: "read with newlines in the middle",
			request: &responseWrapperMock{
				data: strings.NewReader("foo\nbar"),
			},
			expectedOut: "foo\nbar",
		},
		{
			name: "read with newline at the end",
			request: &responseWrapperMock{
				data: strings.NewReader("foo\n"),
			},
			expectedOut: "foo\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := DefaultConsumeRequest(test.request, buf)

			if err != nil && !strings.Contains(err.Error(), test.expectedErr) {
				t.Errorf("%s: expected to find:\n\t%s\nfound:\n\t%s\n", test.name, test.expectedErr, err.Error())
			}

			if buf.String() != test.expectedOut {
				t.Errorf("%s: did not get expected log content. Got: %s", test.name, buf.String())
			}
		})
	}
}

type responseWrapperMock struct {
	data io.Reader
	err  error
}

func (r *responseWrapperMock) DoRaw(context.Context) ([]byte, error) {
	data, _ := ioutil.ReadAll(r.data)
	return data, r.err
}

func (r *responseWrapperMock) Stream(context.Context) (io.ReadCloser, error) {
	return ioutil.NopCloser(r.data), r.err
}
