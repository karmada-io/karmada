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

package token

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/cluster-bootstrap/token/api"

	"github.com/karmada-io/karmada/pkg/karmadactl/options"
	"github.com/karmada-io/karmada/pkg/karmadactl/util"
)

type mockIOWriter struct {
}

func (m *mockIOWriter) Write(p []byte) (n int, err error) {
	str := string(p)
	fmt.Println(str)
	return len(str), nil
}

func NewTestCommandTokenOptions() *CommandTokenOptions {
	return &CommandTokenOptions{
		TTL: &metav1.Duration{
			Duration: time.Second,
		},
		Description:   "foo",
		Groups:        []string{},
		Usages:        []string{},
		parentCommand: "foo",
	}
}

func TestNewCmdTokenCreate(t *testing.T) {
	f := util.NewFactory(options.DefaultConfigFlags)
	cmd := NewCmdTokenCreate(f, &mockIOWriter{}, NewTestCommandTokenOptions())
	if cmd == nil {
		t.Errorf("NewCmdTokenCreate() return nil, but want config")
	}
}

func TestNewCmdTokenList(t *testing.T) {
	f := util.NewFactory(options.DefaultConfigFlags)
	cmd := NewCmdTokenList(f, &mockIOWriter{}, &mockIOWriter{}, NewTestCommandTokenOptions())
	if cmd == nil {
		t.Errorf("NewCmdTokenList() return nil, but want config")
	}
}

func TestNewCmdTokenDelete(t *testing.T) {
	f := util.NewFactory(options.DefaultConfigFlags)
	cmd := NewCmdTokenDelete(f, &mockIOWriter{}, NewTestCommandTokenOptions())
	if cmd == nil {
		t.Errorf("NewCmdTokenDelete() return nil, but want config")
	}
}

func TestCommandTokenOptions_runCreateToken(t *testing.T) {
	cmdOpt := NewTestCommandTokenOptions()
	client := kubernetesfake.NewClientset()
	if err := cmdOpt.runCreateToken(&mockIOWriter{}, client); err != nil {
		t.Errorf("runCreateToken() want nil, but get:%v", err)
	}
}

func TestCommandTokenOptions_runCreateToken_TTL(t *testing.T) {
	tests := []struct {
		name           string
		ttl            time.Duration
		wantErr        bool
		wantExpiration bool
	}{
		{
			name:    "negative ttl is rejected",
			ttl:     -time.Hour,
			wantErr: true,
		},
		{
			name:           "zero ttl never expires",
			ttl:            0,
			wantErr:        false,
			wantExpiration: false,
		},
		{
			name:           "positive ttl sets an expiration",
			ttl:            time.Hour,
			wantErr:        false,
			wantExpiration: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdOpt := NewTestCommandTokenOptions()
			cmdOpt.TTL = &metav1.Duration{Duration: tt.ttl}
			client := kubernetesfake.NewClientset()

			err := cmdOpt.runCreateToken(&mockIOWriter{}, client)
			if (err != nil) != tt.wantErr {
				t.Fatalf("runCreateToken() want error:%v, but get:%v", tt.wantErr, err)
			}

			secrets, listErr := client.CoreV1().Secrets(metav1.NamespaceSystem).List(context.Background(), metav1.ListOptions{})
			if listErr != nil {
				t.Fatalf("List secrets error:%v", listErr)
			}
			if tt.wantErr {
				if len(secrets.Items) != 0 {
					t.Fatalf("want no token secret to be created, but got %d", len(secrets.Items))
				}
				return
			}
			if len(secrets.Items) != 1 {
				t.Fatalf("want 1 token secret, but got %d", len(secrets.Items))
			}
			_, hasExpiration := secrets.Items[0].Data[api.BootstrapTokenExpirationKey]
			if hasExpiration != tt.wantExpiration {
				t.Errorf("token has expiration:%v, want:%v", hasExpiration, tt.wantExpiration)
			}
		})
	}
}

func TestCommandTokenOptions_runListTokens(t *testing.T) {
	tests := []struct {
		name        string
		tokenID     []string
		tokenSecret []string
		wantErr     bool
	}{
		{
			name: "token format error and parse error",
			//It needs special format, follow it will pass checking
			tokenID:     []string{"foo0id"},
			tokenSecret: []string{"---"},
			wantErr:     false,
		},
		{
			name: "token fmt ok and parse ok",
			//It needs special format, follow it will pass checking
			tokenID:     []string{"foo0id"},
			tokenSecret: []string{"foo0secret0foofo"},
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdOpt := NewTestCommandTokenOptions()
			client := kubernetesfake.NewClientset()
			ctx := context.Background()

			for i, id := range tt.tokenID {
				tokenSecret := corev1.Secret{
					Type: api.SecretTypeBootstrapToken,
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("bootstrap-token-%s", id),
					},
					Data: map[string][]byte{
						"token-id":     []byte(id),
						"token-secret": []byte(tt.tokenSecret[i]),
					},
				}

				if _, err := client.CoreV1().Secrets(metav1.NamespaceSystem).
					Create(ctx, &tokenSecret, metav1.CreateOptions{}); err != nil {
					t.Errorf("Create secret error:%v", err)
				}
			}

			if err := cmdOpt.runListTokens(client, &mockIOWriter{}, &mockIOWriter{}); (err != nil) != tt.wantErr {
				t.Errorf("runListTokens() want:%v, but get:%v", tt.wantErr, err)
			}
		})
	}
}

func TestCommandTokenOptions_runDeleteTokens(t *testing.T) {
	tests := []struct {
		name         string
		tokenID      string
		tokenInCache []string
		wantErr      bool
	}{
		{
			name:         "invalid format token id",
			tokenID:      "---",
			tokenInCache: []string{},
			wantErr:      true,
		},
		{ // #nosec G101 -- not a credential; this is a fake token ID used purely as a test fixture.
			name:         "invalid format token id but match BootstrapTokenPattern",
			tokenID:      "foo0id.foo0secret0foofo",
			tokenInCache: []string{},
			wantErr:      true,
		},
		{
			name:         "valid format but not exist in client",
			tokenID:      "foo0id",
			tokenInCache: []string{},
			wantErr:      true,
		},
		{
			name:         "valid format and exist in client",
			tokenID:      "foo0id",
			tokenInCache: []string{"foo0id"},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdOpt := NewTestCommandTokenOptions()
			client := kubernetesfake.NewClientset()
			ctx := context.Background()

			for _, tokenName := range tt.tokenInCache {
				tokenSecret := corev1.Secret{
					Type: api.SecretTypeBootstrapToken,
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("bootstrap-token-%s", tokenName),
					},
				}

				if _, err := client.CoreV1().Secrets(metav1.NamespaceSystem).
					Create(ctx, &tokenSecret, metav1.CreateOptions{}); err != nil {
					t.Errorf("Create secret error:%v", err)
				}
			}
			tokenList := []string{tt.tokenID}
			if err := cmdOpt.runDeleteTokens(&mockIOWriter{}, client, tokenList); (err != nil) != tt.wantErr {
				t.Errorf("runDeleteTokens() want:%v, but get:%v", tt.wantErr, err)
			}
		})
	}
}

func TestCommandTokenOptions_runDeleteTokens_InvalidTokenMessage(t *testing.T) {
	cmdOpt := NewTestCommandTokenOptions()
	client := kubernetesfake.NewClientset()

	err := cmdOpt.runDeleteTokens(&mockIOWriter{}, client, []string{"not-a-token"})
	if err == nil {
		t.Fatalf("runDeleteTokens() want an error for an invalid token, but get nil")
	}
	for _, pattern := range []string{api.BootstrapTokenIDPattern, api.BootstrapTokenPattern} {
		if !strings.Contains(err.Error(), fmt.Sprintf("%q", pattern)) {
			t.Errorf("error %q does not mention pattern %q", err.Error(), pattern)
		}
	}
}

func TestNewCmdToken(t *testing.T) {
	stream := genericiooptions.IOStreams{}
	f := util.NewFactory(options.DefaultConfigFlags)
	cmdToken := NewCmdToken(f, "karmadactl", stream)
	if cmdToken == nil {
		t.Errorf("NewCmdToken() want return not nil, but return nil")
	}
}
